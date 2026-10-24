package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Senpa1k/Giveaway-bot-vk/internal/model"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// drawColumns is the select list of the draws table.
const drawColumns = "id, kind, partner_id, draw_date, status, winners_count, reserve_count, drawn_at, approved_at, approved_by, notified_at, created_at"

// DrawRepository stores draws, their entries and lifecycle transitions.
type DrawRepository struct {
	pool *pgxpool.Pool
}

// NewDrawRepository creates a draw repository over a connection pool.
func NewDrawRepository(pool *pgxpool.Pool) *DrawRepository {
	return &DrawRepository{pool: pool}
}

// GetOrCreateDaily returns the daily draw of the partner's day, creating a
// pending draw on the first registration of the day.
func (r *DrawRepository) GetOrCreateDaily(ctx context.Context, partner *model.Partner, day time.Time) (*model.Draw, error) {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO draws (kind, partner_id, draw_date, status)
		VALUES ('daily', $1, $2::date, 'pending')
		ON CONFLICT (kind, draw_date) DO NOTHING`,
		partner.ID, day.Format(time.DateOnly),
	)
	if err != nil {
		return nil, fmt.Errorf("create daily draw: %w", err)
	}
	return r.ByDate(ctx, model.DrawKindDaily, day)
}

// GetOrCreateFinal returns the final draw of the day, creating a pending one
// with the winner and reserve counts on first use.
func (r *DrawRepository) GetOrCreateFinal(ctx context.Context, day time.Time, winners, reserve int) (*model.Draw, error) {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO draws (kind, draw_date, status, winners_count, reserve_count)
		VALUES ('final', $1::date, 'pending', $2, $3)
		ON CONFLICT (kind, draw_date) DO NOTHING`,
		day.Format(time.DateOnly), winners, reserve,
	)
	if err != nil {
		return nil, fmt.Errorf("create final draw: %w", err)
	}
	return r.ByDate(ctx, model.DrawKindFinal, day)
}

// DrawByID returns the draw or model.ErrDrawNotFound.
func (r *DrawRepository) DrawByID(ctx context.Context, id uuid.UUID) (*model.Draw, error) {
	query := `SELECT ` + drawColumns + ` FROM draws WHERE id = $1 LIMIT 1`
	return scanDraw(r.pool.QueryRow(ctx, query, id))
}

// ByDate returns the draw of the kind on the day or model.ErrDrawNotFound.
func (r *DrawRepository) ByDate(ctx context.Context, kind model.DrawKind, day time.Time) (*model.Draw, error) {
	query := `SELECT ` + drawColumns + ` FROM draws WHERE kind = $1 AND draw_date = $2::date LIMIT 1`
	return scanDraw(r.pool.QueryRow(ctx, query, kind, day.Format(time.DateOnly)))
}

// ListDraws returns all draws newest first.
func (r *DrawRepository) ListDraws(ctx context.Context) ([]model.Draw, error) {
	rows, err := r.pool.Query(ctx, `SELECT `+drawColumns+` FROM draws ORDER BY draw_date DESC, kind`)
	if err != nil {
		return nil, fmt.Errorf("select draws: %w", err)
	}
	defer rows.Close()

	draws := make([]model.Draw, 0)
	for rows.Next() {
		draw, err := scanDraw(rows)
		if err != nil {
			return nil, err
		}
		draws = append(draws, *draw)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read draws: %w", err)
	}
	return draws, nil
}

// SaveDrawn atomically moves a pending draw to drawn and stores its winners.
// It returns model.ErrInvalidDrawStatus when the draw is not pending, so a
// draw can never run twice.
func (r *DrawRepository) SaveDrawn(ctx context.Context, draw *model.Draw, winners []model.Winner) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin draw transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	err = tx.QueryRow(ctx, `
		UPDATE draws
		SET status = 'drawn', winners_count = $1, reserve_count = $2, drawn_at = now()
		WHERE id = $3 AND status = 'pending'
		RETURNING status, drawn_at`,
		countByKind(winners, model.WinnerKindWinner), countByKind(winners, model.WinnerKindReserve), draw.ID,
	).Scan(&draw.Status, &draw.DrawnAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return model.ErrInvalidDrawStatus
		}
		return fmt.Errorf("update draw status: %w", err)
	}

	batch := &pgx.Batch{}
	for i := range winners {
		batch.Queue(`
			INSERT INTO winners (draw_id, user_id, position, kind, prize, tickets)
			VALUES ($1, $2, $3, $4, $5, $6)`,
			winners[i].DrawID, winners[i].UserID, winners[i].Position, winners[i].Kind, winners[i].Prize, winners[i].Tickets,
		)
	}
	if err := sendBatch(ctx, tx, batch); err != nil {
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit draw transaction: %w", err)
	}
	return nil
}

// Approve confirms the drawn results on behalf of an administrator.
// It returns model.ErrInvalidDrawStatus when the draw is not in drawn state.
func (r *DrawRepository) Approve(ctx context.Context, id uuid.UUID, approvedBy string) (*model.Draw, error) {
	draw, err := r.DrawByID(ctx, id)
	if err != nil {
		return nil, err
	}

	err = r.pool.QueryRow(ctx, `
		UPDATE draws
		SET status = 'approved', approved_at = now(), approved_by = $2
		WHERE id = $1 AND status = 'drawn'
		RETURNING approved_at`,
		id, approvedBy,
	).Scan(&draw.ApprovedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, model.ErrInvalidDrawStatus
		}
		return nil, fmt.Errorf("approve draw: %w", err)
	}
	draw.Status = model.DrawStatusApproved
	return draw, nil
}

// MarkDrawNotified completes the draw lifecycle after winner notifications.
func (r *DrawRepository) MarkDrawNotified(ctx context.Context, id uuid.UUID) error {
	tag, err := r.pool.Exec(ctx, `
		UPDATE draws SET status = 'notified', notified_at = now()
		WHERE id = $1 AND status = 'approved'`,
		id,
	)
	if err != nil {
		return fmt.Errorf("mark draw notified: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return model.ErrInvalidDrawStatus
	}
	return nil
}

// HasEntry reports whether the user is already registered in the draw.
func (r *DrawRepository) HasEntry(ctx context.Context, drawID, userID uuid.UUID) (bool, error) {
	var exists bool
	err := r.pool.QueryRow(ctx,
		`SELECT EXISTS (SELECT 1 FROM draw_entries WHERE draw_id = $1 AND user_id = $2)`,
		drawID, userID,
	).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("check draw entry: %w", err)
	}
	return exists, nil
}

// CreateEntry registers the user in the draw; a repeat returns
// model.ErrAlreadyRegistered.
func (r *DrawRepository) CreateEntry(ctx context.Context, drawID, userID uuid.UUID) error {
	tag, err := r.pool.Exec(ctx, `
		INSERT INTO draw_entries (draw_id, user_id) VALUES ($1, $2)
		ON CONFLICT (draw_id, user_id) DO NOTHING`,
		drawID, userID,
	)
	if err != nil {
		return fmt.Errorf("insert draw entry: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return model.ErrAlreadyRegistered
	}
	return nil
}

// EntryUserIDs lists the participants of the draw in registration order.
func (r *DrawRepository) EntryUserIDs(ctx context.Context, drawID uuid.UUID) ([]uuid.UUID, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT user_id FROM draw_entries WHERE draw_id = $1 ORDER BY created_at, id`,
		drawID,
	)
	if err != nil {
		return nil, fmt.Errorf("select draw entries: %w", err)
	}
	defer rows.Close()

	userIDs := make([]uuid.UUID, 0)
	for rows.Next() {
		var userID uuid.UUID
		if err := rows.Scan(&userID); err != nil {
			return nil, fmt.Errorf("scan draw entry: %w", err)
		}
		userIDs = append(userIDs, userID)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read draw entries: %w", err)
	}
	return userIDs, nil
}

// Entries lists full draw entries for exports.
func (r *DrawRepository) Entries(ctx context.Context, drawID uuid.UUID) ([]model.DrawEntry, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, draw_id, user_id, created_at FROM draw_entries WHERE draw_id = $1 ORDER BY created_at, id`,
		drawID,
	)
	if err != nil {
		return nil, fmt.Errorf("select draw entries: %w", err)
	}
	defer rows.Close()

	entries := make([]model.DrawEntry, 0)
	for rows.Next() {
		var entry model.DrawEntry
		if err := rows.Scan(&entry.ID, &entry.DrawID, &entry.UserID, &entry.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan draw entry: %w", err)
		}
		entries = append(entries, entry)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read draw entries: %w", err)
	}
	return entries, nil
}

// scanDraw reads one draw row from a row or rows value.
func scanDraw(row pgx.Row) (*model.Draw, error) {
	var draw model.Draw
	if err := row.Scan(
		&draw.ID,
		&draw.Kind,
		&draw.PartnerID,
		&draw.DrawDate,
		&draw.Status,
		&draw.WinnersCount,
		&draw.ReserveCount,
		&draw.DrawnAt,
		&draw.ApprovedAt,
		&draw.ApprovedBy,
		&draw.NotifiedAt,
		&draw.CreatedAt,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, model.ErrDrawNotFound
		}
		return nil, fmt.Errorf("scan draw: %w", err)
	}
	return &draw, nil
}

// countByKind counts the winners of a kind for the draw columns.
func countByKind(winners []model.Winner, kind model.WinnerKind) int {
	count := 0
	for _, winner := range winners {
		if winner.Kind == kind {
			count++
		}
	}
	return count
}

// sendBatch executes a statement batch inside a transaction.
func sendBatch(ctx context.Context, tx pgx.Tx, batch *pgx.Batch) error {
	results := tx.SendBatch(ctx, batch)
	for i := 0; i < batch.Len(); i++ {
		if _, err := results.Exec(); err != nil {
			_ = results.Close()
			return fmt.Errorf("insert winner %d: %w", i+1, err)
		}
	}
	if err := results.Close(); err != nil {
		return fmt.Errorf("close winners batch: %w", err)
	}
	return nil
}
