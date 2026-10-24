package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ChanceRepository stores user chances: one active day is one ticket in the
// final draw.
type ChanceRepository struct {
	pool *pgxpool.Pool
}

// NewChanceRepository creates a chance repository over a connection pool.
func NewChanceRepository(pool *pgxpool.Pool) *ChanceRepository {
	return &ChanceRepository{pool: pool}
}

// AddChance inserts the day chance and reports whether it was new. The
// (user_id, day) unique constraint makes the second action of the day a no-op.
func (r *ChanceRepository) AddChance(ctx context.Context, userID uuid.UUID, day time.Time, source string) (bool, error) {
	tag, err := r.pool.Exec(ctx, `
		INSERT INTO chances (user_id, day, source)
		VALUES ($1, $2::date, $3)
		ON CONFLICT (user_id, day) DO NOTHING`,
		userID, day.Format(time.DateOnly), source,
	)
	if err != nil {
		return false, fmt.Errorf("insert chance: %w", err)
	}
	return tag.RowsAffected() > 0, nil
}

// CountByUser returns how many chances the user has accumulated.
func (r *ChanceRepository) CountByUser(ctx context.Context, userID uuid.UUID) (int, error) {
	var count int
	err := r.pool.QueryRow(ctx,
		`SELECT count(*) FROM chances WHERE user_id = $1`, userID,
	).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count chances: %w", err)
	}
	return count, nil
}

// UserTickets lists one user ID per chance: the tickets of the final draw.
func (r *ChanceRepository) UserTickets(ctx context.Context) ([]uuid.UUID, error) {
	rows, err := r.pool.Query(ctx, `SELECT user_id FROM chances`)
	if err != nil {
		return nil, fmt.Errorf("select chance tickets: %w", err)
	}
	defer rows.Close()

	tickets := make([]uuid.UUID, 0)
	for rows.Next() {
		var userID uuid.UUID
		if err := rows.Scan(&userID); err != nil {
			return nil, fmt.Errorf("scan chance ticket: %w", err)
		}
		tickets = append(tickets, userID)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read chance tickets: %w", err)
	}
	return tickets, nil
}

// PartnersByUser lists the names of partners the user interacted with: got a
// promocode from them or joined their daily draw.
func (r *ChanceRepository) PartnersByUser(ctx context.Context, userID uuid.UUID) ([]string, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT name FROM (
			SELECT p.name FROM partners p
			JOIN promocodes c ON c.partner_id = p.id AND c.user_id = $1
			UNION
			SELECT p.name FROM partners p
			JOIN draws d ON d.partner_id = p.id AND d.kind = 'daily'
			JOIN draw_entries e ON e.draw_id = d.id AND e.user_id = $1
		) partners
		ORDER BY name`,
		userID,
	)
	if err != nil {
		return nil, fmt.Errorf("select user partners: %w", err)
	}
	defer rows.Close()

	names := make([]string, 0)
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, fmt.Errorf("scan user partner: %w", err)
		}
		names = append(names, name)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read user partners: %w", err)
	}
	return names, nil
}
