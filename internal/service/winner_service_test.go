package service

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/Senpa1k/Giveaway-bot-vk/internal/model"

	"github.com/google/uuid"
)

func newWinnerEnv() (*WinnerService, *fakeWinnerRepo, *fakeUsersRepo, *fakeDrawsRepo, *fakeSender) {
	repo := &fakeWinnerRepo{}
	users := newFakeUsersRepo()
	draws := newFakeDrawsRepo()
	sender := &fakeSender{}
	partners := &fakePartnerRepo{}
	service := NewWinnerService(repo, users, draws, partners, sender, time.Second, testLogger())
	return service, repo, users, draws, sender
}

func approvedDraw() (*model.Draw, uuid.UUID) {
	partnerID := newUserID()
	return &model.Draw{
		ID:        newUserID(),
		Kind:      model.DrawKindDaily,
		PartnerID: &partnerID,
		Status:    model.DrawStatusApproved,
	}, partnerID
}

func TestStartClaimWithoutPrize(t *testing.T) {
	service, _, _, _, _ := newWinnerEnv()
	user := model.User{ID: newUserID(), MaxID: 1}

	reply, err := service.StartClaim(context.Background(), &user)
	if err != nil {
		t.Fatalf("StartClaim() error: %v", err)
	}
	if !strings.Contains(reply.Text, "Приза за тобой пока не числится") {
		t.Fatalf("reply = %q, want the no-prize message", reply.Text)
	}
}

func TestClaimFlowCollectsContactData(t *testing.T) {
	service, repo, users, _, _ := newWinnerEnv()
	winner := model.Winner{ID: newUserID(), UserID: newUserID(), Status: model.WinnerStatusAwaitingResponse}
	repo.winners = append(repo.winners, winner)
	user := model.User{ID: winner.UserID, MaxID: 7}

	// «Получить приз»: the bot asks for the name.
	reply, err := service.StartClaim(context.Background(), &user)
	if err != nil {
		t.Fatalf("StartClaim() error: %v", err)
	}
	if !strings.Contains(reply.Text, "имя и фамилию") {
		t.Fatalf("reply = %q, want the name question", reply.Text)
	}
	if state := users.states[user.ID]; state.State != model.StateAwaitingName || state.ID != winner.ID {
		t.Fatalf("state = %+v, want awaiting_name with the winner id", state)
	}

	// The user sends the name; the router reloads the dialog state first.
	user.State = users.states[user.ID]
	reply, err = service.SubmitName(context.Background(), &user, "  Иван   Петров ")
	if err != nil {
		t.Fatalf("SubmitName() error: %v", err)
	}
	if !strings.Contains(reply.Text, "Иван Петров") {
		t.Fatalf("reply = %q, want the consent prompt with the name", reply.Text)
	}
	if state := users.states[user.ID]; state.State != model.StateAwaitingConsent || state.Name != "Иван Петров" {
		t.Fatalf("state = %+v, want awaiting_consent with the name", state)
	}

	// The user agrees to share the data.
	user.State = users.states[user.ID]
	reply, err = service.ProcessConsent(context.Background(), &user, true)
	if err != nil {
		t.Fatalf("ProcessConsent() error: %v", err)
	}
	if !strings.Contains(reply.Text, "Готово") {
		t.Fatalf("reply = %q, want the done message", reply.Text)
	}
	saved, err := repo.WinnerByID(context.Background(), winner.ID)
	if err != nil {
		t.Fatalf("WinnerByID() error: %v", err)
	}
	if saved.ContactName != "Иван Петров" || !saved.ConsentGiven || saved.Status != model.WinnerStatusConfirmed {
		t.Fatalf("saved winner = %+v, want name, consent and confirmed status", saved)
	}
	if state := users.states[user.ID]; state.State != model.StateNone {
		t.Fatalf("state after consent = %q, want cleared", state.State)
	}
}

func TestSubmitNameRejectsTooShortAnswer(t *testing.T) {
	service, _, users, _, _ := newWinnerEnv()
	user := model.User{ID: newUserID(), MaxID: 1}
	users.states[user.ID] = model.UserState{State: model.StateAwaitingName, ID: newUserID()}

	reply, err := service.SubmitName(context.Background(), &user, "Я")
	if err != nil {
		t.Fatalf("SubmitName() error: %v", err)
	}
	if !strings.Contains(reply.Text, "Напиши, пожалуйста") {
		t.Fatalf("reply = %q, want the retry message", reply.Text)
	}
	if state := users.states[user.ID]; state.State != model.StateAwaitingName {
		t.Fatalf("state = %q, want unchanged awaiting_name", state.State)
	}
}

func TestProcessConsentRefusalKeepsPrizeOpen(t *testing.T) {
	service, repo, users, _, _ := newWinnerEnv()
	winner := model.Winner{ID: newUserID(), UserID: newUserID(), Status: model.WinnerStatusContacted}
	repo.winners = append(repo.winners, winner)
	user := model.User{ID: winner.UserID, MaxID: 7}
	users.states[user.ID] = model.UserState{State: model.StateAwaitingConsent, ID: winner.ID, Name: "Иван Петров"}

	reply, err := service.ProcessConsent(context.Background(), &user, false)
	if err != nil {
		t.Fatalf("ProcessConsent() error: %v", err)
	}
	if !strings.Contains(reply.Text, "Если передумаешь") {
		t.Fatalf("reply = %q, want the refusal message", reply.Text)
	}
	saved, _ := repo.WinnerByID(context.Background(), winner.ID)
	if saved.ConsentGiven || saved.Status != model.WinnerStatusContacted {
		t.Fatalf("saved winner = %+v, want the prize to stay open", saved)
	}
}

func TestNotifyDrawAfterApproval(t *testing.T) {
	service, repo, users, draws, sender := newWinnerEnv()
	draw, partnerID := approvedDraw()
	draws.draws[draw.ID] = draw

	winner := model.Winner{
		ID:     newUserID(),
		DrawID: draw.ID,
		UserID: newUserID(),
		Kind:   model.WinnerKindWinner,
		Prize:  "мерч",
		Status: model.WinnerStatusAwaitingResponse,
	}
	reserve := model.Winner{ID: newUserID(), DrawID: draw.ID, UserID: newUserID(), Kind: model.WinnerKindReserve, Prize: "мерч"}
	repo.winners = []model.Winner{winner, reserve}
	users.users[winner.UserID] = model.User{ID: winner.UserID, MaxID: 11}
	users.users[reserve.UserID] = model.User{ID: reserve.UserID, MaxID: 12}
	service.partners = &fakePartnerRepo{partner: &model.Partner{
		ID: partnerID, Name: "Партнер", PrizeInstruction: "Забери приз на стойке.",
	}}

	if err := service.NotifyDraw(context.Background(), draw.ID); err != nil {
		t.Fatalf("NotifyDraw() error: %v", err)
	}
	if len(sender.messages) != 1 {
		t.Fatalf("sent messages = %d, want only the prize winner", len(sender.messages))
	}
	if sender.messages[0].userID != 11 {
		t.Fatalf("message peer = %d, want the winner", sender.messages[0].userID)
	}
	if !strings.Contains(sender.messages[0].text, "Ты в числе хедлайнеров дня") ||
		!strings.Contains(sender.messages[0].text, "Забери приз на стойке.") {
		t.Fatalf("message = %q, want the daily winner text", sender.messages[0].text)
	}
	if !draws.drawNotified[draw.ID] {
		t.Fatal("the draw was not marked notified")
	}
}

func TestNotifyDrawRequiresApproval(t *testing.T) {
	service, _, _, draws, _ := newWinnerEnv()
	draw, _ := approvedDraw()
	draw.Status = model.DrawStatusDrawn
	draws.draws[draw.ID] = draw

	if err := service.NotifyDraw(context.Background(), draw.ID); !errors.Is(err, model.ErrInvalidDrawStatus) {
		t.Fatalf("NotifyDraw() error = %v, want ErrInvalidDrawStatus", err)
	}
}

func TestMissPromotesReserveWinner(t *testing.T) {
	service, repo, users, draws, sender := newWinnerEnv()
	draw, partnerID := approvedDraw()
	draws.draws[draw.ID] = draw

	winner := model.Winner{ID: newUserID(), DrawID: draw.ID, UserID: newUserID(), Prize: "мерч", Status: model.WinnerStatusContacted}
	reserve := model.Winner{ID: newUserID(), DrawID: draw.ID, UserID: newUserID(), Kind: model.WinnerKindReserve, Prize: "мерч", Status: model.WinnerStatusAwaitingResponse}
	repo.winners = []model.Winner{winner, reserve}
	users.users[reserve.UserID] = model.User{ID: reserve.UserID, MaxID: 21}
	service.partners = &fakePartnerRepo{partner: &model.Partner{ID: partnerID, Name: "Партнер"}}

	promoted, err := service.Miss(context.Background(), winner.ID)
	if err != nil {
		t.Fatalf("Miss() error: %v", err)
	}
	if promoted == nil || promoted.ID != reserve.ID {
		t.Fatalf("promoted = %+v, want the reserve winner", promoted)
	}
	if saved, _ := repo.WinnerByID(context.Background(), winner.ID); saved.Status != model.WinnerStatusMissed {
		t.Fatalf("missed winner status = %q, want missed", saved.Status)
	}
	if len(sender.messages) != 1 || sender.messages[0].userID != 21 {
		t.Fatalf("sent = %+v, want one message to the promoted winner", sender.messages)
	}
}

func TestMissWithoutReserve(t *testing.T) {
	service, repo, _, draws, _ := newWinnerEnv()
	draw, _ := approvedDraw()
	draws.draws[draw.ID] = draw
	winner := model.Winner{ID: newUserID(), DrawID: draw.ID, UserID: newUserID(), Status: model.WinnerStatusAwaitingResponse}
	repo.winners = []model.Winner{winner}

	promoted, err := service.Miss(context.Background(), winner.ID)
	if err != nil {
		t.Fatalf("Miss() error: %v", err)
	}
	if promoted != nil {
		t.Fatalf("promoted = %+v, want nil without reserves", promoted)
	}
}

// testLogger returns a quiet logger for services under test.
func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}
