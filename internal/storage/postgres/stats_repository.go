package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/Senpa1k/Giveaway-bot-vk/internal/model"

	"github.com/jackc/pgx/v5/pgxpool"
)

// StatsRepository aggregates the analytics of the technical spec.
type StatsRepository struct {
	pool *pgxpool.Pool
}

// NewStatsRepository creates a stats repository over a connection pool.
func NewStatsRepository(pool *pgxpool.Pool) *StatsRepository {
	return &StatsRepository{pool: pool}
}

// CountUsers returns how many users started the bot.
func (r *StatsRepository) CountUsers(ctx context.Context) (int64, error) {
	var count int64
	err := r.pool.QueryRow(ctx, `SELECT count(*) FROM users`).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count users: %w", err)
	}
	return count, nil
}

// NewUsersByDay returns new users per day since from. Days are counted in
// the given IANA time zone of the campaign.
func (r *StatsRepository) NewUsersByDay(ctx context.Context, timezone string, from time.Time) ([]model.DayCount, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT to_char(created_at AT TIME ZONE $1, 'YYYY-MM-DD') AS day, count(*) AS count
		FROM users
		WHERE created_at >= $2
		GROUP BY day
		ORDER BY day`,
		timezone, from,
	)
	if err != nil {
		return nil, fmt.Errorf("select new users by day: %w", err)
	}
	defer rows.Close()

	days := make([]model.DayCount, 0)
	for rows.Next() {
		var day model.DayCount
		if err := rows.Scan(&day.Day, &day.Count); err != nil {
			return nil, fmt.Errorf("scan new users day: %w", err)
		}
		days = append(days, day)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read new users by day: %w", err)
	}
	return days, nil
}

// PromoTotals returns the total promo button presses and issued codes.
func (r *StatsRepository) PromoTotals(ctx context.Context) (requests, issued int64, err error) {
	err = r.pool.QueryRow(ctx, `
		SELECT
			(SELECT count(*) FROM promo_requests),
			(SELECT count(*) FROM promocodes WHERE user_id IS NOT NULL)
	`).Scan(&requests, &issued)
	if err != nil {
		return 0, 0, fmt.Errorf("select promo totals: %w", err)
	}
	return requests, issued, nil
}

// PromoByPartner returns the promo request, issue and remaining counts of
// every partner.
func (r *StatsRepository) PromoByPartner(ctx context.Context) ([]model.PartnerPromoStats, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT p.id, p.name, to_char(p.day, 'YYYY-MM-DD') AS day,
		       COALESCE(req.count, 0) AS requests,
		       COALESCE(codes.total, 0) AS total,
		       COALESCE(codes.issued, 0) AS issued
		FROM partners p
		LEFT JOIN (
			SELECT partner_id, count(*) AS count FROM promo_requests GROUP BY partner_id
		) req ON req.partner_id = p.id
		LEFT JOIN (
			SELECT partner_id, count(*) AS total, count(user_id) AS issued
			FROM promocodes GROUP BY partner_id
		) codes ON codes.partner_id = p.id
		ORDER BY p.day`)
	if err != nil {
		return nil, fmt.Errorf("select promo stats by partner: %w", err)
	}
	defer rows.Close()

	stats := make([]model.PartnerPromoStats, 0)
	for rows.Next() {
		var row model.PartnerPromoStats
		if err := rows.Scan(
			&row.PartnerID,
			&row.Name,
			&row.Day,
			&row.Requests,
			&row.Total,
			&row.Issued,
		); err != nil {
			return nil, fmt.Errorf("scan promo partner stats: %w", err)
		}
		row.Remaining = row.Total - row.Issued
		stats = append(stats, row)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read promo partner stats: %w", err)
	}
	return stats, nil
}

// DrawParticipation returns the participant count of every draw.
func (r *StatsRepository) DrawParticipation(ctx context.Context) ([]model.DrawStats, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT d.id, d.kind, to_char(d.draw_date, 'YYYY-MM-DD') AS date,
		       COALESCE(p.name, '') AS partner_name, d.status,
		       (SELECT count(*) FROM draw_entries e WHERE e.draw_id = d.id) AS participants
		FROM draws d
		LEFT JOIN partners p ON p.id = d.partner_id
		ORDER BY d.draw_date DESC, d.kind`)
	if err != nil {
		return nil, fmt.Errorf("select draw participation: %w", err)
	}
	defer rows.Close()

	stats := make([]model.DrawStats, 0)
	for rows.Next() {
		var row model.DrawStats
		if err := rows.Scan(
			&row.DrawID,
			&row.Kind,
			&row.Date,
			&row.PartnerName,
			&row.Status,
			&row.Participants,
		); err != nil {
			return nil, fmt.Errorf("scan draw stats: %w", err)
		}
		stats = append(stats, row)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read draw stats: %w", err)
	}
	return stats, nil
}
