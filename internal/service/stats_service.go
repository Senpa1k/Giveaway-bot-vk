package service

import (
	"context"
	"fmt"
	"time"

	"github.com/Senpa1k/Giveaway-bot-vk/internal/model"
)

// StatsRepository aggregates the analytics counters.
type StatsRepository interface {
	CountUsers(ctx context.Context) (int64, error)
	NewUsersByDay(ctx context.Context, timezone string, from time.Time) ([]model.DayCount, error)
	PromoTotals(ctx context.Context) (requests, issued int64, err error)
	PromoByPartner(ctx context.Context) ([]model.PartnerPromoStats, error)
	DrawParticipation(ctx context.Context) ([]model.DrawStats, error)
}

// statsWindowDays is how far back the new-users-per-day series goes.
const statsWindowDays = 30

// StatsService collects the analytics of the technical spec: user growth,
// promocode demand and supply, draw participation.
type StatsService struct {
	repo         StatsRepository
	timezone     string
	queryTimeout time.Duration
}

// NewStatsService constructs the stats service. The timezone fixes the day
// boundaries of the per-day series.
func NewStatsService(repo StatsRepository, timezone string, queryTimeout time.Duration) *StatsService {
	return &StatsService{repo: repo, timezone: timezone, queryTimeout: queryTimeout}
}

// Get returns the full analytics snapshot.
func (s *StatsService) Get(ctx context.Context) (*model.Stats, error) {
	queryCtx, cancel := context.WithTimeout(ctx, s.queryTimeout)
	defer cancel()

	totalUsers, err := s.repo.CountUsers(queryCtx)
	if err != nil {
		return nil, fmt.Errorf("count users: %w", err)
	}

	from := time.Now().AddDate(0, 0, -statsWindowDays)
	newUsers, err := s.repo.NewUsersByDay(queryCtx, s.timezone, from)
	if err != nil {
		return nil, fmt.Errorf("new users by day: %w", err)
	}

	requests, issued, err := s.repo.PromoTotals(queryCtx)
	if err != nil {
		return nil, fmt.Errorf("promo totals: %w", err)
	}

	byPartner, err := s.repo.PromoByPartner(queryCtx)
	if err != nil {
		return nil, fmt.Errorf("promo stats by partner: %w", err)
	}

	draws, err := s.repo.DrawParticipation(queryCtx)
	if err != nil {
		return nil, fmt.Errorf("draw participation: %w", err)
	}

	return &model.Stats{
		TotalUsers:         totalUsers,
		NewUsersByDay:      newUsers,
		PromoRequestsTotal: requests,
		PromoIssuedTotal:   issued,
		PromoByPartner:     byPartner,
		Draws:              draws,
	}, nil
}
