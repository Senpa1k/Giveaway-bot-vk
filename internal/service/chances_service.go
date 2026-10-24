package service

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// ChancesRepository is the storage used by the chances service.
type ChancesRepository interface {
	AddChance(ctx context.Context, userID uuid.UUID, day time.Time, source string) (bool, error)
	CountByUser(ctx context.Context, userID uuid.UUID) (int, error)
	PartnersByUser(ctx context.Context, userID uuid.UUID) ([]string, error)
}

// CampaignView describes the campaign for the «Мои шансы» screen.
type CampaignView struct {
	MainPrize    string
	MaxChances   int
	RemainingDays func(today time.Time) int
}

// ChancesService accrues final-draw chances and builds the «Мои шансы»
// screen. One active day is one chance; the total is capped.
type ChancesService struct {
	repo         ChancesRepository
	campaign     CampaignView
	queryTimeout time.Duration
}

// NewChancesService constructs the chances service.
func NewChancesService(repo ChancesRepository, campaign CampaignView, queryTimeout time.Duration) *ChancesService {
	return &ChancesService{repo: repo, campaign: campaign, queryTimeout: queryTimeout}
}

// Add accrues the day chance: the first action of the day adds one, further
// actions of the same day are no-ops, and the total never exceeds the cap.
func (s *ChancesService) Add(ctx context.Context, userID uuid.UUID, today time.Time, source string) error {
	queryCtx, cancel := context.WithTimeout(ctx, s.queryTimeout)
	defer cancel()

	count, err := s.repo.CountByUser(queryCtx, userID)
	if err != nil {
		return fmt.Errorf("count chances: %w", err)
	}
	if count >= s.campaign.MaxChances {
		return nil
	}

	if _, err = s.repo.AddChance(queryCtx, userID, today, source); err != nil {
		return fmt.Errorf("add chance: %w", err)
	}
	return nil
}

// Count returns how many chances the user has accumulated.
func (s *ChancesService) Count(ctx context.Context, userID uuid.UUID) (int, error) {
	queryCtx, cancel := context.WithTimeout(ctx, s.queryTimeout)
	defer cancel()

	count, err := s.repo.CountByUser(queryCtx, userID)
	if err != nil {
		return 0, fmt.Errorf("count chances: %w", err)
	}
	return count, nil
}

// MyChances builds the «Мои шансы» screen of the user.
func (s *ChancesService) MyChances(ctx context.Context, userID uuid.UUID, today time.Time) (*Reply, error) {
	queryCtx, cancel := context.WithTimeout(ctx, s.queryTimeout)
	defer cancel()

	count, err := s.repo.CountByUser(queryCtx, userID)
	if err != nil {
		return nil, fmt.Errorf("count chances: %w", err)
	}
	partners, err := s.repo.PartnersByUser(queryCtx, userID)
	if err != nil {
		return nil, fmt.Errorf("list user partners: %w", err)
	}

	return chancesReply(chancesView{
		Count:      count,
		MaxChances: s.campaign.MaxChances,
		Days:       count, // one chance per day makes the counts equal
		Partners:   partners,
		Remaining:  s.campaign.RemainingDays(today),
		MainPrize:  s.campaign.MainPrize,
	}), nil
}
