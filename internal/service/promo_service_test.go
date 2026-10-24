package service

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/Senpa1k/Giveaway-bot-vk/internal/model"

	"github.com/google/uuid"
)

// promoTestEnv collects the fakes behind the promo and chances services.
type promoTestEnv struct {
	promo   *PromoService
	chances *ChancesService
	repo    *fakePromoRepo
	partner *fakePartnerRepo
	user    *model.User
	today   time.Time
}

func newPromoTestEnv(maxChances int) *promoTestEnv {
	today := time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC)
	partner := &model.Partner{
		ID:        uuid.New(),
		Name:      "Партнер",
		Day:       today,
		MaxURL:    "https://max.ru/partner",
		PrizeName: "приз дня",
	}
	env := &promoTestEnv{
		repo:    &fakePromoRepo{assigned: make(map[uuid.UUID]*model.Promocode)},
		partner: &fakePartnerRepo{partner: partner},
		user:    &model.User{ID: uuid.New(), MaxID: 1},
		today:   today,
	}
	env.chances = NewChancesService(newFakeChancesRepo(), CampaignView{
		MainPrize:     "главный приз",
		MaxChances:    maxChances,
		RemainingDays: func(time.Time) int { return 5 },
	}, time.Second)
	env.promo = NewPromoService(env.repo, env.partner, env.chances, time.Second)
	return env
}

type fakePromoRepo struct {
	codes    []*model.Promocode
	assigned map[uuid.UUID]*model.Promocode
	requests int
}

func (f *fakePromoRepo) LogRequest(_ context.Context, _, _ uuid.UUID) error {
	f.requests++
	return nil
}

func (f *fakePromoRepo) ActiveForUser(_ context.Context, userID, partnerID uuid.UUID) (*model.Promocode, error) {
	if promo, found := f.assigned[userID]; found && promo.PartnerID == partnerID {
		return promo, nil
	}
	return nil, model.ErrPromocodeNotFound
}

func (f *fakePromoRepo) AssignFree(_ context.Context, partnerID, userID uuid.UUID) (*model.Promocode, error) {
	for _, code := range f.codes {
		if code.PartnerID == partnerID && code.UserID == nil {
			code.UserID = &userID
			f.assigned[userID] = code
			return code, nil
		}
	}
	return nil, model.ErrPromocodesDepleted
}

type fakePartnerRepo struct {
	partner *model.Partner
}

func (f *fakePartnerRepo) ByDay(_ context.Context, day time.Time) (*model.Partner, error) {
	if f.partner != nil && f.partner.Day.Equal(day) {
		return f.partner, nil
	}
	return nil, model.ErrPartnerNotFound
}

func (f *fakePartnerRepo) PartnerByID(_ context.Context, id uuid.UUID) (*model.Partner, error) {
	if f.partner != nil && f.partner.ID == id {
		return f.partner, nil
	}
	return nil, model.ErrPartnerNotFound
}

type fakeChancesRepo struct {
	chances map[uuid.UUID]map[string]string
}

func newFakeChancesRepo() *fakeChancesRepo {
	return &fakeChancesRepo{chances: make(map[uuid.UUID]map[string]string)}
}

func (f *fakeChancesRepo) AddChance(_ context.Context, userID uuid.UUID, day time.Time, source string) (bool, error) {
	days, found := f.chances[userID]
	if !found {
		days = make(map[string]string)
		f.chances[userID] = days
	}
	key := day.Format(time.DateOnly)
	if _, duplicate := days[key]; duplicate {
		return false, nil
	}
	days[key] = source
	return true, nil
}

func (f *fakeChancesRepo) CountByUser(_ context.Context, userID uuid.UUID) (int, error) {
	return len(f.chances[userID]), nil
}

func (f *fakeChancesRepo) PartnersByUser(_ context.Context, _ uuid.UUID) ([]string, error) {
	return nil, nil
}

func addFreeCode(env *promoTestEnv, code string) {
	env.repo.codes = append(env.repo.codes, &model.Promocode{
		ID:        uuid.New(),
		PartnerID: env.partner.partner.ID,
		Code:      code,
	})
}

func TestPromoIssuesCodeAndAddsChance(t *testing.T) {
	env := newPromoTestEnv(10)
	addFreeCode(env, "FEST-001")
	addFreeCode(env, "FEST-002")

	reply, err := env.promo.Handle(context.Background(), env.user, env.today)
	if err != nil {
		t.Fatalf("Handle() error: %v", err)
	}
	if !strings.Contains(reply.Text, "FEST-001") {
		t.Fatalf("reply = %q, want the issued code", reply.Text)
	}
	if !strings.Contains(reply.Text, "Партнер") {
		t.Fatalf("reply = %q, want the partner name", reply.Text)
	}
	if count, _ := env.chances.repo.CountByUser(context.Background(), env.user.ID); count != 1 {
		t.Fatalf("chances after issue = %d, want 1", count)
	}
	if env.repo.requests != 1 {
		t.Fatalf("promo requests = %d, want 1", env.repo.requests)
	}
}

func TestPromoRepeatShowsSameCode(t *testing.T) {
	env := newPromoTestEnv(10)
	addFreeCode(env, "FEST-001")
	addFreeCode(env, "FEST-002")

	if _, err := env.promo.Handle(context.Background(), env.user, env.today); err != nil {
		t.Fatalf("first Handle() error: %v", err)
	}
	reply, err := env.promo.Handle(context.Background(), env.user, env.today)
	if err != nil {
		t.Fatalf("second Handle() error: %v", err)
	}

	if !strings.Contains(reply.Text, "FEST-001") {
		t.Fatalf("repeat reply = %q, want the same code again", reply.Text)
	}
	// A new code must not be taken, and the day still counts as one chance.
	if code, _ := env.repo.ActiveForUser(context.Background(), env.user.ID, env.partner.partner.ID); code.Code != "FEST-001" {
		t.Fatalf("assigned code changed to %q", code.Code)
	}
	if count, _ := env.chances.repo.CountByUser(context.Background(), env.user.ID); count != 1 {
		t.Fatalf("chances after repeat = %d, want 1", count)
	}
}

func TestPromoDepletedDoesNotBlockDraw(t *testing.T) {
	env := newPromoTestEnv(10)

	reply, err := env.promo.Handle(context.Background(), env.user, env.today)
	if err != nil {
		t.Fatalf("Handle() error: %v", err)
	}
	if !strings.Contains(reply.Text, "закончились") {
		t.Fatalf("reply = %q, want the depleted message", reply.Text)
	}
	if reply.Keyboard == nil {
		t.Fatal("depleted reply must keep the draw button")
	}
	if count, _ := env.chances.repo.CountByUser(context.Background(), env.user.ID); count != 0 {
		t.Fatalf("chances after depletion = %d, want 0", count)
	}
}

func TestPromoNoPartnerToday(t *testing.T) {
	env := newPromoTestEnv(10)
	env.partner.partner = nil

	reply, err := env.promo.Handle(context.Background(), env.user, env.today)
	if err != nil {
		t.Fatalf("Handle() error: %v", err)
	}
	if !strings.Contains(reply.Text, "Сегодня партнеров") {
		t.Fatalf("reply = %q, want the no-partner message", reply.Text)
	}
}

// One action day must count as one chance even when both the promocode and
// the draw registration happen.
func TestPromoAndDrawSameDayOneChance(t *testing.T) {
	env := newPromoTestEnv(10)
	addFreeCode(env, "FEST-001")

	if _, err := env.promo.Handle(context.Background(), env.user, env.today); err != nil {
		t.Fatalf("promo Handle() error: %v", err)
	}
	if err := env.chances.Add(context.Background(), env.user.ID, env.today, model.ChanceSourceDraw); err != nil {
		t.Fatalf("draw chance Add() error: %v", err)
	}
	if count, _ := env.chances.repo.CountByUser(context.Background(), env.user.ID); count != 1 {
		t.Fatalf("chances after promo and draw = %d, want 1", count)
	}
}
