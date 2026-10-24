package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Senpa1k/Giveaway-bot-vk/internal/model"

	sq "github.com/Masterminds/squirrel"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// partnerColumns is the select list of the partners table.
const partnerColumns = "id, name, day, max_channel_id, max_url, promo_valid_until, promo_where, prize_name, prize_instruction, winners_count, created_at"

// PartnerRepository stores action partners.
type PartnerRepository struct {
	pool *pgxpool.Pool
}

// NewPartnerRepository creates a partner repository over a connection pool.
func NewPartnerRepository(pool *pgxpool.Pool) *PartnerRepository {
	return &PartnerRepository{pool: pool}
}

// Create inserts a partner and fills its ID and creation time.
func (r *PartnerRepository) Create(ctx context.Context, partner *model.Partner) error {
	query, args, err := statementBuilder().
		Insert("partners").
		Columns("name", "day", "max_channel_id", "max_url", "promo_valid_until", "promo_where", "prize_name", "prize_instruction", "winners_count").
		Values(
			partner.Name,
			sq.Expr("?::date", partner.Day.Format(time.DateOnly)),
			partner.MaxChannelID,
			partner.MaxURL,
			dateExpr(partner.PromoValidUntil),
			partner.PromoWhere,
			partner.PrizeName,
			partner.PrizeInstruction,
			partner.WinnersCount,
		).
		Suffix("RETURNING id, created_at").
		ToSql()
	if err != nil {
		return fmt.Errorf("build partner insert: %w", err)
	}

	if err := r.pool.QueryRow(ctx, query, args...).Scan(&partner.ID, &partner.CreatedAt); err != nil {
		return fmt.Errorf("insert partner: %w", err)
	}
	return nil
}

// ByDay returns the partner of the day or model.ErrPartnerNotFound.
func (r *PartnerRepository) ByDay(ctx context.Context, day time.Time) (*model.Partner, error) {
	query, args, err := statementBuilder().
		Select(partnerColumns).
		From("partners").
		Where(sq.Expr("day = ?::date", day.Format(time.DateOnly))).
		Limit(1).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("build partner query: %w", err)
	}
	return scanPartner(r.pool.QueryRow(ctx, query, args...))
}

// PartnerByID returns the partner or model.ErrPartnerNotFound.
func (r *PartnerRepository) PartnerByID(ctx context.Context, id uuid.UUID) (*model.Partner, error) {
	query, args, err := statementBuilder().
		Select(partnerColumns).
		From("partners").
		Where(sq.Eq{"id": id}).
		Limit(1).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("build partner query: %w", err)
	}
	return scanPartner(r.pool.QueryRow(ctx, query, args...))
}

// ListPartners returns all partners ordered by their day.
func (r *PartnerRepository) ListPartners(ctx context.Context) ([]model.Partner, error) {
	query, args, err := statementBuilder().
		Select(partnerColumns).
		From("partners").
		OrderBy("day").
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("build partners query: %w", err)
	}

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("select partners: %w", err)
	}
	defer rows.Close()

	partners := make([]model.Partner, 0)
	for rows.Next() {
		partner, err := scanPartner(rows)
		if err != nil {
			return nil, err
		}
		partners = append(partners, *partner)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read partners: %w", err)
	}
	return partners, nil
}

// scanPartner reads one partner row from a row or rows value.
func scanPartner(row pgx.Row) (*model.Partner, error) {
	var partner model.Partner
	if err := row.Scan(
		&partner.ID,
		&partner.Name,
		&partner.Day,
		&partner.MaxChannelID,
		&partner.MaxURL,
		&partner.PromoValidUntil,
		&partner.PromoWhere,
		&partner.PrizeName,
		&partner.PrizeInstruction,
		&partner.WinnersCount,
		&partner.CreatedAt,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, model.ErrPartnerNotFound
		}
		return nil, fmt.Errorf("scan partner: %w", err)
	}
	return &partner, nil
}

// dateExpr formats an optional date for a ::date cast, NULL when unset.
func dateExpr(value *time.Time) any {
	if value == nil {
		return nil
	}
	return sq.Expr("?::date", value.Format(time.DateOnly))
}
