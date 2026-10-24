package service

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/Senpa1k/Giveaway-bot-vk/internal/model"

	"github.com/google/uuid"
)

func newRaffleEnv(final FinalSettings) (*RaffleService, *fakeRaffleRepo, *fakePartnerRepo, time.Time) {
	today := time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC)
	partner := &model.Partner{
		ID:           newUserID(),
		Name:         "Партнер",
		Day:          today,
		PrizeName:    "мерч-пакет",
		WinnersCount: 2,
	}
	repo := &fakeRaffleRepo{draws: make(map[string]*model.Draw)}
	partners := &fakePartnerRepo{partner: partner}
	service := NewRaffleService(repo, partners, &fakeRaffleChances{}, final, time.Second)
	// The tests rely on the original ticket order: no shuffling.
	service.shuffle = func(_ []uuid.UUID) error { return nil }
	return service, repo, partners, today
}

func pendingDailyDraw(partner *model.Partner, day time.Time, _ ...uuid.UUID) *model.Draw {
	partnerID := partner.ID
	return &model.Draw{
		ID:        newUserID(),
		Kind:      model.DrawKindDaily,
		PartnerID: &partnerID,
		DrawDate:  day,
		Status:    model.DrawStatusPending,
	}
}

func TestRunDailyPicksDistinctWinners(t *testing.T) {
	final := FinalSettings{Winners: 5, TicketsEach: 2, Reserve: 3, MainPrize: "главный приз"}
	service, repo, partners, today := newRaffleEnv(final)

	participants := []uuid.UUID{newUserID(), newUserID(), newUserID()}
	draw := pendingDailyDraw(partners.partner, today)
	repo.draws[string(model.DrawKindDaily)] = draw
	repo.entries = participants

	result, err := service.RunDaily(context.Background(), today)
	if err != nil {
		t.Fatalf("RunDaily() error: %v", err)
	}
	if result.Participants != len(participants) {
		t.Fatalf("participants = %d, want %d", result.Participants, len(participants))
	}

	// The partner day has two prizes and three participants.
	if len(result.Winners) != 2 {
		t.Fatalf("winners = %d, want 2", len(result.Winners))
	}
	seen := make(map[uuid.UUID]bool, len(result.Winners))
	for _, winner := range result.Winners {
		if seen[winner.UserID] {
			t.Fatalf("user %s won twice", winner.UserID)
		}
		seen[winner.UserID] = true
		if winner.Prize != "мерч-пакет" {
			t.Fatalf("winner prize = %q, want the partner prize", winner.Prize)
		}
		if winner.Kind != model.WinnerKindWinner {
			t.Fatalf("winner kind = %q, want winner", winner.Kind)
		}
	}
	if draw.Status != model.DrawStatusDrawn {
		t.Fatalf("draw status = %q, want drawn", draw.Status)
	}
}

func TestRunDailyWithoutEntries(t *testing.T) {
	service, repo, partners, today := newRaffleEnv(FinalSettings{})
	repo.draws[string(model.DrawKindDaily)] = pendingDailyDraw(partners.partner, today)

	if _, err := service.RunDaily(context.Background(), today); !errors.Is(err, model.ErrNoEntries) {
		t.Fatalf("RunDaily() error = %v, want ErrNoEntries", err)
	}
}

func TestRunDailyWithoutRegistrations(t *testing.T) {
	service, _, _, today := newRaffleEnv(FinalSettings{})

	if _, err := service.RunDaily(context.Background(), today); !errors.Is(err, model.ErrDrawNotFound) {
		t.Fatalf("RunDaily() error = %v, want ErrDrawNotFound", err)
	}
}

func TestRunDailyTwiceIsRejected(t *testing.T) {
	service, repo, partners, today := newRaffleEnv(FinalSettings{})
	repo.draws[string(model.DrawKindDaily)] = pendingDailyDraw(partners.partner, today)
	repo.entries = []uuid.UUID{newUserID()}

	if _, err := service.RunDaily(context.Background(), today); err != nil {
		t.Fatalf("first RunDaily() error: %v", err)
	}
	if _, err := service.RunDaily(context.Background(), today); !errors.Is(err, model.ErrInvalidDrawStatus) {
		t.Fatalf("second RunDaily() error = %v, want ErrInvalidDrawStatus", err)
	}
}

func TestRunFinalWeighsChancesAndPicksReserves(t *testing.T) {
	final := FinalSettings{Winners: 2, TicketsEach: 2, Reserve: 1, MainPrize: "2 билета"}
	service, _, _, today := newRaffleEnv(final)

	// Three tickets of one user, one ticket each of two others.
	heavy, light, third := newUserID(), newUserID(), newUserID()
	service.chances = &fakeRaffleChances{tickets: []uuid.UUID{heavy, heavy, heavy, light, third}}

	result, err := service.RunFinal(context.Background(), today)
	if err != nil {
		t.Fatalf("RunFinal() error: %v", err)
	}

	// With the identity shuffle the first distinct users win: two winners
	// plus one reserve, all distinct, winners carry two tickets.
	if len(result.Winners) != 3 {
		t.Fatalf("winners = %d, want 2 winners + 1 reserve", len(result.Winners))
	}
	if result.Winners[0].UserID != heavy || result.Winners[1].UserID != light {
		t.Fatalf("winner order = %s, %s; want ticket order", result.Winners[0].UserID, result.Winners[1].UserID)
	}
	if result.Winners[0].Tickets != 2 || result.Winners[1].Tickets != 2 {
		t.Fatalf("winner tickets = %d, %d; want 2, 2", result.Winners[0].Tickets, result.Winners[1].Tickets)
	}
	if result.Winners[2].UserID != third || result.Winners[2].Kind != model.WinnerKindReserve {
		t.Fatalf("third pick = %s %q, want the third user as reserve", result.Winners[2].UserID, result.Winners[2].Kind)
	}
	if result.Participants != 3 {
		t.Fatalf("distinct participants = %d, want 3", result.Participants)
	}
	if result.Winners[0].Prize != "2 билета" {
		t.Fatalf("final prize = %q, want the configured main prize", result.Winners[0].Prize)
	}
}

func TestPickUniqueKeepsEveryPickDistinct(t *testing.T) {
	service, _, _, _ := newRaffleEnv(FinalSettings{})

	ids := []uuid.UUID{newUserID(), newUserID(), newUserID()}
	picked, err := service.pickUnique(context.Background(), ids, 3)
	if err != nil {
		t.Fatalf("pickUnique() error: %v", err)
	}
	byString := func(a, b uuid.UUID) int { return strings.Compare(a.String(), b.String()) }
	slices.SortFunc(ids, byString)
	sorted := slices.Clone(picked)
	slices.SortFunc(sorted, byString)
	if !slices.EqualFunc(ids, sorted, func(a, b uuid.UUID) bool { return a == b }) {
		t.Fatalf("picked %v, want the same set as %v", sorted, ids)
	}
}
