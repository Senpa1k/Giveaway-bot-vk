package service

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/Senpa1k/Giveaway-bot-vk/internal/model"

	"github.com/google/uuid"
)

// RaffleRepository is the storage used by raffles.
type RaffleRepository interface {
	GetOrCreateFinal(ctx context.Context, day time.Time, winners, reserve int) (*model.Draw, error)
	ByDate(ctx context.Context, kind model.DrawKind, day time.Time) (*model.Draw, error)
	SaveDrawn(ctx context.Context, draw *model.Draw, winners []model.Winner) error
	EntryUserIDs(ctx context.Context, drawID uuid.UUID) ([]uuid.UUID, error)
}

// RaffleChances lists the tickets of the final draw.
type RaffleChances interface {
	UserTickets(ctx context.Context) ([]uuid.UUID, error)
}

// FinalSettings configures the final draw.
type FinalSettings struct {
	Winners     int
	TicketsEach int
	Reserve     int
	MainPrize   string
}

// RaffleService runs the daily partner draws and the final draw. Winners are
// picked uniformly at random and stored for admin approval; notifications
// are sent only after the approval, not by this service.
type RaffleService struct {
	repo         RaffleRepository
	partners     PartnerOfTheDay
	chances      RaffleChances
	final        FinalSettings
	shuffle      func([]uuid.UUID) error
	queryTimeout time.Duration
}

// NewRaffleService constructs the raffle service.
func NewRaffleService(
	repo RaffleRepository,
	partners PartnerOfTheDay,
	chances RaffleChances,
	final FinalSettings,
	queryTimeout time.Duration,
) *RaffleService {
	return &RaffleService{
		repo:         repo,
		partners:     partners,
		chances:      chances,
		final:        final,
		shuffle:      shuffleIDs,
		queryTimeout: queryTimeout,
	}
}

// RunDaily randomly picks the winners of the partner day among the verified
// participants and stores the result for admin approval. It returns
// model.ErrDrawNotFound when nobody registered that day.
func (s *RaffleService) RunDaily(ctx context.Context, day time.Time) (*model.DrawResult, error) {
	queryCtx, cancel := context.WithTimeout(ctx, s.queryTimeout)
	defer cancel()

	partner, err := s.partners.ByDay(queryCtx, day)
	if err != nil {
		return nil, fmt.Errorf("find partner of the day: %w", err)
	}

	draw, err := s.repo.ByDate(queryCtx, model.DrawKindDaily, day)
	if err != nil {
		return nil, fmt.Errorf("find daily draw: %w", err)
	}
	if draw.Status != model.DrawStatusPending {
		return nil, model.ErrInvalidDrawStatus
	}

	userIDs, err := s.repo.EntryUserIDs(queryCtx, draw.ID)
	if err != nil {
		return nil, fmt.Errorf("list draw entries: %w", err)
	}
	if len(userIDs) == 0 {
		return nil, model.ErrNoEntries
	}

	// Fewer participants than prizes: every participant wins.
	winnersCount := min(partner.WinnersCount, len(userIDs))
	order, err := s.pickUnique(queryCtx, userIDs, winnersCount)
	if err != nil {
		return nil, err
	}

	winners := make([]model.Winner, 0, winnersCount)
	for i, userID := range order {
		winners = append(winners, model.Winner{
			DrawID:   draw.ID,
			UserID:   userID,
			Position: i + 1,
			Kind:     model.WinnerKindWinner,
			Prize:    partner.PrizeName,
			Tickets:  1,
			Status:   model.WinnerStatusAwaitingResponse,
		})
	}

	if err = s.repo.SaveDrawn(queryCtx, draw, winners); err != nil {
		return nil, fmt.Errorf("save daily draw results: %w", err)
	}
	return &model.DrawResult{Draw: *draw, Participants: len(userIDs), Winners: winners}, nil
}

// RunFinal picks the final winners among all accumulated chances: every
// chance is one ticket, and one user wins at most one prize. The configured
// number of reserve winners follows the prize winners.
func (s *RaffleService) RunFinal(ctx context.Context, day time.Time) (*model.DrawResult, error) {
	queryCtx, cancel := context.WithTimeout(ctx, s.queryTimeout)
	defer cancel()

	draw, err := s.repo.GetOrCreateFinal(queryCtx, day, s.final.Winners, s.final.Reserve)
	if err != nil {
		return nil, fmt.Errorf("get final draw: %w", err)
	}
	if draw.Status != model.DrawStatusPending {
		return nil, model.ErrInvalidDrawStatus
	}

	tickets, err := s.chances.UserTickets(queryCtx)
	if err != nil {
		return nil, fmt.Errorf("list final tickets: %w", err)
	}
	if len(tickets) == 0 {
		return nil, model.ErrNoEntries
	}

	needed := s.final.Winners + s.final.Reserve
	order, err := s.pickUnique(queryCtx, tickets, needed)
	if err != nil {
		return nil, err
	}

	winners := make([]model.Winner, 0, len(order))
	for i, userID := range order {
		winner := model.Winner{
			DrawID:   draw.ID,
			UserID:   userID,
			Position: i + 1,
			Prize:    s.final.MainPrize,
			Tickets:  s.final.TicketsEach,
			Status:   model.WinnerStatusAwaitingResponse,
		}
		if i < s.final.Winners {
			winner.Kind = model.WinnerKindWinner
		} else {
			winner.Kind = model.WinnerKindReserve
		}
		winners = append(winners, winner)
	}

	if err = s.repo.SaveDrawn(queryCtx, draw, winners); err != nil {
		return nil, fmt.Errorf("save final draw results: %w", err)
	}
	return &model.DrawResult{Draw: *draw, Participants: countUnique(tickets), Winners: winners}, nil
}

// pickUnique shuffles the ids with crypto randomness and returns the first
// n distinct ones: every id keeps an equal chance of every position.
func (s *RaffleService) pickUnique(_ context.Context, ids []uuid.UUID, n int) ([]uuid.UUID, error) {
	if n <= 0 {
		return []uuid.UUID{}, nil
	}

	pool := slices.Clone(ids)
	if err := s.shuffle(pool); err != nil {
		return nil, fmt.Errorf("shuffle draw tickets: %w", err)
	}

	picked := make([]uuid.UUID, 0, n)
	seen := make(map[uuid.UUID]struct{}, n)
	for _, id := range pool {
		if _, duplicate := seen[id]; duplicate {
			continue
		}
		seen[id] = struct{}{}
		picked = append(picked, id)
		if len(picked) == n {
			break
		}
	}
	return picked, nil
}

// countUnique counts the distinct users behind the tickets.
func countUnique(ids []uuid.UUID) int {
	unique := make(map[uuid.UUID]struct{}, len(ids))
	for _, id := range ids {
		unique[id] = struct{}{}
	}
	return len(unique)
}

// IsSkippableDrawError reports whether a raffle error means the day simply
// has nothing to draw, which is a normal outcome for the scheduler.
func IsSkippableDrawError(err error) bool {
	return errors.Is(err, model.ErrPartnerNotFound) ||
		errors.Is(err, model.ErrDrawNotFound) ||
		errors.Is(err, model.ErrNoEntries) ||
		errors.Is(err, model.ErrInvalidDrawStatus)
}
