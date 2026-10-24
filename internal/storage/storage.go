// Package storage assembles the bot's repositories.
package storage

import (
	"github.com/Senpa1k/Giveaway-bot-vk/internal/storage/postgres"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Repository groups storage dependencies used by the service layer.
type Repository struct {
	*postgres.HealthRepository
	*postgres.UserRepository
	*postgres.PartnerRepository
	*postgres.PromoRepository
	*postgres.DrawRepository
	*postgres.ChanceRepository
	*postgres.WinnerRepository
	*postgres.StatsRepository
}

// NewRepository creates repositories over the shared PostgreSQL pool.
func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{
		HealthRepository:  postgres.NewHealthRepository(pool),
		UserRepository:    postgres.NewUserRepository(pool),
		PartnerRepository: postgres.NewPartnerRepository(pool),
		PromoRepository:   postgres.NewPromoRepository(pool),
		DrawRepository:    postgres.NewDrawRepository(pool),
		ChanceRepository:  postgres.NewChanceRepository(pool),
		WinnerRepository:  postgres.NewWinnerRepository(pool),
		StatsRepository:   postgres.NewStatsRepository(pool),
	}
}
