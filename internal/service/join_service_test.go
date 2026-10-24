package service

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/Senpa1k/Giveaway-bot-vk/internal/maxapi"
	"github.com/Senpa1k/Giveaway-bot-vk/internal/model"
)

func newJoinEnv(member bool, subErr error) (*JoinService, *fakeJoinRepo, *fakeChancesRepo, *model.User, time.Time) {
	today := time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC)
	partner := &model.Partner{
		ID:           newUserID(),
		Name:         "Партнер",
		Day:          today,
		MaxChannelID: 100,
		MaxURL:       "https://max.ru/partner",
	}
	repo := &fakeJoinRepo{}
	chancesRepo := newFakeChancesRepo()
	chances := NewChancesService(chancesRepo, CampaignView{MaxChances: 10, RemainingDays: func(time.Time) int { return 5 }}, time.Second)
	join := NewJoinService(repo, &fakePartnerRepo{partner: partner}, &fakeSubscriber{member: member, err: subErr}, chances, time.Second)
	return join, repo, chancesRepo, &model.User{ID: newUserID(), MaxID: 42}, today
}

func TestJoinScreenShowsPartner(t *testing.T) {
	join, _, _, _, today := newJoinEnv(false, nil)

	reply, err := join.Screen(context.Background(), today)
	if err != nil {
		t.Fatalf("Screen() error: %v", err)
	}
	if !strings.Contains(reply.Text, "подпишись на аккаунт партнера") {
		t.Fatalf("reply = %q, want the subscription instruction", reply.Text)
	}
	if rows := keyboardRows(t, reply.Keyboard); rows != 2 {
		t.Fatalf("keyboard rows = %d, want subscribe link and check button", rows)
	}
}

func TestCheckRegistersSubscribedUser(t *testing.T) {
	join, repo, chancesRepo, user, today := newJoinEnv(true, nil)

	reply, err := join.Check(context.Background(), user, today)
	if err != nil {
		t.Fatalf("Check() error: %v", err)
	}
	if !strings.Contains(reply.Text, "Ты в лайнапе ✅") {
		t.Fatalf("reply = %q, want the registered message", reply.Text)
	}
	if !repo.entries[user.ID] {
		t.Fatal("the draw entry was not created")
	}
	if len(chancesRepo.chances[user.ID]) != 1 {
		t.Fatalf("chances after registration = %d, want 1", len(chancesRepo.chances[user.ID]))
	}
}

func TestCheckRejectsUnsubscribedUser(t *testing.T) {
	join, repo, chancesRepo, user, today := newJoinEnv(false, nil)

	reply, err := join.Check(context.Background(), user, today)
	if err != nil {
		t.Fatalf("Check() error: %v", err)
	}
	if !strings.Contains(reply.Text, "Пока не вижу подписку") {
		t.Fatalf("reply = %q, want the not-subscribed message", reply.Text)
	}
	if repo.entries[user.ID] {
		t.Fatal("an unsubscribed user must not be registered")
	}
	if len(chancesRepo.chances[user.ID]) != 0 {
		t.Fatal("an unsubscribed user must not gain a chance")
	}
}

func TestCheckToleratesSubscriptionFailure(t *testing.T) {
	join, _, chancesRepo, user, today := newJoinEnv(false, errors.New("max is down"))

	reply, err := join.Check(context.Background(), user, today)
	if err != nil {
		t.Fatalf("Check() error: %v", err)
	}
	if !strings.Contains(reply.Text, "Не получилось проверить подписку") {
		t.Fatalf("reply = %q, want the retry message", reply.Text)
	}
	if len(chancesRepo.chances[user.ID]) != 0 {
		t.Fatal("a failed check must not gain a chance")
	}
}

func TestCheckIsOncePerDay(t *testing.T) {
	join, repo, _, user, today := newJoinEnv(true, nil)

	if _, err := join.Check(context.Background(), user, today); err != nil {
		t.Fatalf("first Check() error: %v", err)
	}
	reply, err := join.Check(context.Background(), user, today)
	if err != nil {
		t.Fatalf("second Check() error: %v", err)
	}
	if !strings.Contains(reply.Text, "уже участвуешь") {
		t.Fatalf("reply = %q, want the already-registered message", reply.Text)
	}
	if len(repo.entries) != 1 {
		t.Fatalf("entries after repeat = %d, want 1", len(repo.entries))
	}
}

// keyboardRows counts the rows of a keyboard through its JSON form.
func keyboardRows(t *testing.T, keyboard *maxapi.Keyboard) int {
	t.Helper()
	encoded, err := json.Marshal(keyboard)
	if err != nil {
		t.Fatalf("marshal keyboard: %v", err)
	}
	var decoded struct {
		Buttons [][]struct {
			Type string `json:"type"`
		} `json:"buttons"`
	}
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatalf("unmarshal keyboard: %v", err)
	}
	return len(decoded.Buttons)
}
