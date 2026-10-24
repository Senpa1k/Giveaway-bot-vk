package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/Senpa1k/Giveaway-bot-vk/internal/model"

	sq "github.com/Masterminds/squirrel"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// winnerColumns is the select list of the winners table.
const winnerColumns = "id, draw_id, user_id, position, kind, prize, tickets, status, contact_name, consent_given, consent_at, notified_at, created_at"

// WinnerRepository stores draw winners and their prize delivery state.
type WinnerRepository struct {
	pool *pgxpool.Pool
}

// NewWinnerRepository creates a winner repository over a connection pool.
func NewWinnerRepository(pool *pgxpool.Pool) *WinnerRepository {
	return &WinnerRepository{pool: pool}
}

// ListByDraw returns the winners of the draw, prize winners before reserves.
func (r *WinnerRepository) ListByDraw(ctx context.Context, drawID uuid.UUID) ([]model.Winner, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT `+winnerColumns+` FROM winners WHERE draw_id = $1 ORDER BY kind DESC, position`,
		drawID,
	)
	if err != nil {
		return nil, fmt.Errorf("select winners: %w", err)
	}
	defer rows.Close()

	winners := make([]model.Winner, 0)
	for rows.Next() {
		winner, err := scanWinner(rows)
		if err != nil {
			return nil, err
		}
		winners = append(winners, *winner)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read winners: %w", err)
	}
	return winners, nil
}

// ActiveClaim returns the newest prize the user has not confirmed yet, or
// model.ErrWinnerNotFound. It backs the «Получить приз» button.
func (r *WinnerRepository) ActiveClaim(ctx context.Context, userID uuid.UUID) (*model.Winner, error) {
	query := `SELECT ` + winnerColumns + `
		FROM winners
		WHERE user_id = $1 AND status IN ('awaiting_response', 'contacted')
		ORDER BY created_at DESC
		LIMIT 1`
	return scanWinner(r.pool.QueryRow(ctx, query, userID))
}

// ByID returns the winner or model.ErrWinnerNotFound.
func (r *WinnerRepository) WinnerByID(ctx context.Context, id uuid.UUID) (*model.Winner, error) {
	query := `SELECT ` + winnerColumns + ` FROM winners WHERE id = $1 LIMIT 1`
	return scanWinner(r.pool.QueryRow(ctx, query, id))
}

// MarkContacted records that the winner started the claim dialog.
func (r *WinnerRepository) MarkContacted(ctx context.Context, id uuid.UUID) error {
	return r.updateStatus(ctx, id, "contacted", "awaiting_response")
}

// ConfirmContact saves the contact data with the consent to share it and
// completes the claim.
func (r *WinnerRepository) ConfirmContact(ctx context.Context, id uuid.UUID, name string) error {
	tag, err := r.pool.Exec(ctx, `
		UPDATE winners
		SET contact_name = $2, consent_given = TRUE, consent_at = now(), status = 'confirmed'
		WHERE id = $1 AND status IN ('awaiting_response', 'contacted')`,
		id, name,
	)
	if err != nil {
		return fmt.Errorf("confirm winner contact: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return model.ErrInvalidWinnerStatus
	}
	return nil
}

// MarkNotified timestamps the notified winners.
func (r *WinnerRepository) MarkWinnersNotified(ctx context.Context, ids []uuid.UUID) error {
	if len(ids) == 0 {
		return nil
	}

	query, args, err := statementBuilder().
		Update("winners").
		Set("notified_at", sq.Expr("now()")).
		Where(sq.Eq{"id": ids}).
		ToSql()
	if err != nil {
		return fmt.Errorf("build winners update: %w", err)
	}

	if _, err = r.pool.Exec(ctx, query, args...); err != nil {
		return fmt.Errorf("mark winners notified: %w", err)
	}
	return nil
}

// MarkMissed closes the prize of a winner who missed the deadline.
func (r *WinnerRepository) MarkMissed(ctx context.Context, id uuid.UUID) error {
	tag, err := r.pool.Exec(ctx, `
		UPDATE winners SET status = 'missed'
		WHERE id = $1 AND status IN ('awaiting_response', 'contacted')`,
		id,
	)
	if err != nil {
		return fmt.Errorf("mark winner missed: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return model.ErrInvalidWinnerStatus
	}
	return nil
}

// FirstReserve returns the first reserve winner who can take the prize, or
// model.ErrWinnerNotFound when the reserves are exhausted.
func (r *WinnerRepository) FirstReserve(ctx context.Context, drawID uuid.UUID) (*model.Winner, error) {
	query := `SELECT ` + winnerColumns + `
		FROM winners
		WHERE draw_id = $1 AND kind = 'reserve' AND status = 'awaiting_response'
		ORDER BY position
		LIMIT 1`
	return scanWinner(r.pool.QueryRow(ctx, query, drawID))
}

// PromoteReserve turns a reserve winner into a prize winner.
func (r *WinnerRepository) PromoteReserve(ctx context.Context, id uuid.UUID) error {
	tag, err := r.pool.Exec(ctx, `
		UPDATE winners SET kind = 'winner'
		WHERE id = $1 AND kind = 'reserve' AND status = 'awaiting_response'`,
		id,
	)
	if err != nil {
		return fmt.Errorf("promote reserve winner: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return model.ErrInvalidWinnerStatus
	}
	return nil
}

// updateStatus moves the winner between two statuses; zero affected rows
// mean the transition is not allowed.
func (r *WinnerRepository) updateStatus(ctx context.Context, id uuid.UUID, to, from string) error {
	tag, err := r.pool.Exec(ctx, `UPDATE winners SET status = $2 WHERE id = $1 AND status = $3`, id, to, from)
	if err != nil {
		return fmt.Errorf("update winner status: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return model.ErrInvalidWinnerStatus
	}
	return nil
}

// scanWinner reads one winner row from a row or rows value.
func scanWinner(row pgx.Row) (*model.Winner, error) {
	var winner model.Winner
	if err := row.Scan(
		&winner.ID,
		&winner.DrawID,
		&winner.UserID,
		&winner.Position,
		&winner.Kind,
		&winner.Prize,
		&winner.Tickets,
		&winner.Status,
		&winner.ContactName,
		&winner.ConsentGiven,
		&winner.ConsentAt,
		&winner.NotifiedAt,
		&winner.CreatedAt,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, model.ErrWinnerNotFound
		}
		return nil, fmt.Errorf("scan winner: %w", err)
	}
	return &winner, nil
}
