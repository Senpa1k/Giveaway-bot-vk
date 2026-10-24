package service

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/Senpa1k/Giveaway-bot-vk/internal/model"
)

func newChancesService(maxChances int) (*ChancesService, *fakeChancesRepo) {
	repo := newFakeChancesRepo()
	service := NewChancesService(repo, CampaignView{
		MainPrize:     "2 билета на концерт",
		MaxChances:    maxChances,
		RemainingDays: func(time.Time) int { return 7 },
	}, time.Second)
	return service, repo
}

// The chance total is capped: an active day after the cap changes nothing.
func TestChancesCappedAtMax(t *testing.T) {
	service, repo := newChancesService(3)
	user := model.User{ID: newUserID(), MaxID: 1}

	for day := 1; day <= 5; day++ {
		today := time.Date(2026, 11, day, 12, 0, 0, 0, time.UTC)
		if err := service.Add(context.Background(), user.ID, today, model.ChanceSourceDraw); err != nil {
			t.Fatalf("Add() day %d error: %v", day, err)
		}
	}

	if count := len(repo.chances[user.ID]); count != 3 {
		t.Fatalf("stored chances = %d, want 3", count)
	}
	if count, err := service.Count(context.Background(), user.ID); err != nil || count != 3 {
		t.Fatalf("Count() = %d, %v; want 3, nil", count, err)
	}
}

func TestMyChancesScreen(t *testing.T) {
	service, _ := newChancesService(10)
	user := model.User{ID: newUserID(), MaxID: 1}

	// Two active days out of ten.
	for _, day := range []int{1, 2} {
		today := time.Date(2026, 11, day, 12, 0, 0, 0, time.UTC)
		if err := service.Add(context.Background(), user.ID, today, model.ChanceSourcePromo); err != nil {
			t.Fatalf("Add() day %d error: %v", day, err)
		}
	}

	reply, err := service.MyChances(context.Background(), user.ID, time.Date(2026, 11, 3, 12, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("MyChances() error: %v", err)
	}
	for _, want := range []string{"Накоплено шансов: 2 из 10", "Пройдено дней: 2", "Осталось дней акции: 7", "2 билета на концерт"} {
		if !strings.Contains(reply.Text, want) {
			t.Fatalf("reply = %q, want %q inside", reply.Text, want)
		}
	}
}
