package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/Senpa1k/Giveaway-bot-vk/internal/model"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PromoRepository stores partner promocodes and the request log.
type PromoRepository struct {
	pool *pgxpool.Pool
}

// NewPromoRepository creates a promo repository over a connection pool.
func NewPromoRepository(pool *pgxpool.Pool) *PromoRepository {
	return &PromoRepository{pool: pool}
}

// LogRequest records a promo button press, including presses after the
// codes run out, for analytics.
func (r *PromoRepository) LogRequest(ctx context.Context, userID, partnerID uuid.UUID) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO promo_requests (user_id, partner_id) VALUES ($1, $2)`,
		userID, partnerID,
	)
	if err != nil {
		return fmt.Errorf("insert promo request: %w", err)
	}
	return nil
}

// ActiveForUser returns the code the partner has already issued to the user,
// or model.ErrPromocodeNotFound. Codes stay bound to the user, so a repeated
// press shows the same code.
func (r *PromoRepository) ActiveForUser(ctx context.Context, userID, partnerID uuid.UUID) (*model.Promocode, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT id, partner_id, code, user_id, assigned_at, created_at
		FROM promocodes
		WHERE user_id = $1 AND partner_id = $2
		LIMIT 1`, userID, partnerID)
	return scanPromocode(row)
}

// AssignFree atomically binds the oldest free partner code to the user.
// It returns model.ErrPromocodesDepleted when no free code is left.
// FOR UPDATE SKIP LOCKED keeps concurrent assignments from waiting on or
// taking the same code.
func (r *PromoRepository) AssignFree(ctx context.Context, partnerID, userID uuid.UUID) (*model.Promocode, error) {
	row := r.pool.QueryRow(ctx, `
		UPDATE promocodes SET user_id = $1, assigned_at = now()
		WHERE id = (
			SELECT id FROM promocodes
			WHERE partner_id = $2 AND user_id IS NULL
			ORDER BY created_at, id
			FOR UPDATE SKIP LOCKED
			LIMIT 1
		)
		RETURNING id, partner_id, code, user_id, assigned_at, created_at`,
		userID, partnerID,
	)
	return scanPromocode(row)
}

// ImportCodes inserts a batch of free codes for the partner, skipping
// duplicates, and returns how many codes were added.
func (r *PromoRepository) ImportCodes(ctx context.Context, partnerID uuid.UUID, codes []string) (int64, error) {
	tag, err := r.pool.Exec(ctx, `
		INSERT INTO promocodes (partner_id, code)
		SELECT $1, unnest($2::text[])
		ON CONFLICT (partner_id, code) DO NOTHING`,
		partnerID, codes,
	)
	if err != nil {
		return 0, fmt.Errorf("import promocodes: %w", err)
	}
	return tag.RowsAffected(), nil
}

// scanPromocode reads one promocode row from a row or rows value.
func scanPromocode(row pgx.Row) (*model.Promocode, error) {
	var promo model.Promocode
	if err := row.Scan(
		&promo.ID,
		&promo.PartnerID,
		&promo.Code,
		&promo.UserID,
		&promo.AssignedAt,
		&promo.CreatedAt,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, model.ErrPromocodeNotFound
		}
		return nil, fmt.Errorf("scan promocode: %w", err)
	}
	return &promo, nil
}
