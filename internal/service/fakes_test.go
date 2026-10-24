package service

import (
	"context"
	"time"

	"github.com/Senpa1k/Giveaway-bot-vk/internal/model"
	"github.com/Senpa1k/Giveaway-bot-vk/internal/vkapi"

	"github.com/google/uuid"
)

// newUserID returns a fresh user id for tests.
func newUserID() uuid.UUID {
	return uuid.New()
}

// sentMessage records one outgoing bot message.
type sentMessage struct {
	peerID   int64
	text     string
	keyboard *vkapi.Keyboard
}

// fakeSender records the messages instead of sending them.
type fakeSender struct {
	messages []sentMessage
	err      error
}

func (f *fakeSender) MessagesSend(_ context.Context, peerID int64, text string, keyboard *vkapi.Keyboard, _ string) error {
	if f.err != nil {
		return f.err
	}
	f.messages = append(f.messages, sentMessage{peerID: peerID, text: text, keyboard: keyboard})
	return nil
}

// fakeSubscriber answers VK subscription checks with a fixed result.
type fakeSubscriber struct {
	member bool
	err    error
}

func (f *fakeSubscriber) IsMember(_ context.Context, _, _ int64) (bool, error) {
	return f.member, f.err
}

// fakeJoinRepo stores daily draw entries in memory.
type fakeJoinRepo struct {
	draw    *model.Draw
	entries map[uuid.UUID]bool
	created int
}

func (f *fakeJoinRepo) GetOrCreateDaily(_ context.Context, partner *model.Partner, day time.Time) (*model.Draw, error) {
	if f.draw == nil {
		partnerID := partner.ID
		f.draw = &model.Draw{
			ID:        uuid.New(),
			Kind:      model.DrawKindDaily,
			PartnerID: &partnerID,
			DrawDate:  day,
			Status:    model.DrawStatusPending,
		}
		f.created++
	}
	return f.draw, nil
}

func (f *fakeJoinRepo) HasEntry(_ context.Context, _, userID uuid.UUID) (bool, error) {
	return f.entries[userID], nil
}

func (f *fakeJoinRepo) CreateEntry(_ context.Context, _, userID uuid.UUID) error {
	if f.entries[userID] {
		return model.ErrAlreadyRegistered
	}
	if f.entries == nil {
		f.entries = make(map[uuid.UUID]bool)
	}
	f.entries[userID] = true
	return nil
}

// fakeUsersRepo stores users and their dialog states.
type fakeUsersRepo struct {
	users  map[uuid.UUID]model.User
	states map[uuid.UUID]model.UserState
}

func newFakeUsersRepo(users ...model.User) *fakeUsersRepo {
	repo := &fakeUsersRepo{
		users:  make(map[uuid.UUID]model.User, len(users)),
		states: make(map[uuid.UUID]model.UserState),
	}
	for _, user := range users {
		repo.users[user.ID] = user
	}
	return repo
}

func (f *fakeUsersRepo) ByIDs(_ context.Context, ids []uuid.UUID) (map[uuid.UUID]model.User, error) {
	found := make(map[uuid.UUID]model.User, len(ids))
	for _, id := range ids {
		if user, ok := f.users[id]; ok {
			found[id] = user
		}
	}
	return found, nil
}

func (f *fakeUsersRepo) SetState(_ context.Context, id uuid.UUID, state model.UserState) error {
	f.states[id] = state
	return nil
}

// fakeWinnerRepo stores winners and their delivery state.
type fakeWinnerRepo struct {
	winners []model.Winner
}

func (f *fakeWinnerRepo) ListByDraw(_ context.Context, drawID uuid.UUID) ([]model.Winner, error) {
	result := make([]model.Winner, 0, len(f.winners))
	for _, winner := range f.winners {
		if winner.DrawID == drawID {
			result = append(result, winner)
		}
	}
	return result, nil
}

func (f *fakeWinnerRepo) ActiveClaim(_ context.Context, userID uuid.UUID) (*model.Winner, error) {
	for i := len(f.winners) - 1; i >= 0; i-- {
		if f.winners[i].UserID == userID &&
			(f.winners[i].Status == model.WinnerStatusAwaitingResponse || f.winners[i].Status == model.WinnerStatusContacted) {
			return &f.winners[i], nil
		}
	}
	return nil, model.ErrWinnerNotFound
}

func (f *fakeWinnerRepo) WinnerByID(_ context.Context, id uuid.UUID) (*model.Winner, error) {
	for i := range f.winners {
		if f.winners[i].ID == id {
			return &f.winners[i], nil
		}
	}
	return nil, model.ErrWinnerNotFound
}

func (f *fakeWinnerRepo) MarkContacted(_ context.Context, id uuid.UUID) error {
	for i := range f.winners {
		if f.winners[i].ID == id {
			f.winners[i].Status = model.WinnerStatusContacted
		}
	}
	return nil
}

func (f *fakeWinnerRepo) ConfirmContact(_ context.Context, id uuid.UUID, name string) error {
	for i := range f.winners {
		if f.winners[i].ID == id {
			if f.winners[i].Status != model.WinnerStatusAwaitingResponse && f.winners[i].Status != model.WinnerStatusContacted {
				return model.ErrInvalidWinnerStatus
			}
			f.winners[i].ContactName = name
			f.winners[i].ConsentGiven = true
			f.winners[i].Status = model.WinnerStatusConfirmed
			return nil
		}
	}
	return model.ErrWinnerNotFound
}

func (f *fakeWinnerRepo) MarkWinnersNotified(_ context.Context, ids []uuid.UUID) error {
	for i := range f.winners {
		for _, id := range ids {
			if f.winners[i].ID == id {
				now := time.Now()
				f.winners[i].NotifiedAt = &now
			}
		}
	}
	return nil
}

func (f *fakeWinnerRepo) MarkMissed(_ context.Context, id uuid.UUID) error {
	for i := range f.winners {
		if f.winners[i].ID == id {
			if f.winners[i].Status != model.WinnerStatusAwaitingResponse && f.winners[i].Status != model.WinnerStatusContacted {
				return model.ErrInvalidWinnerStatus
			}
			f.winners[i].Status = model.WinnerStatusMissed
			return nil
		}
	}
	return model.ErrWinnerNotFound
}

func (f *fakeWinnerRepo) FirstReserve(_ context.Context, drawID uuid.UUID) (*model.Winner, error) {
	for i := range f.winners {
		if f.winners[i].DrawID == drawID && f.winners[i].Kind == model.WinnerKindReserve &&
			f.winners[i].Status == model.WinnerStatusAwaitingResponse {
			return &f.winners[i], nil
		}
	}
	return nil, model.ErrWinnerNotFound
}

func (f *fakeWinnerRepo) PromoteReserve(_ context.Context, id uuid.UUID) error {
	for i := range f.winners {
		if f.winners[i].ID == id {
			f.winners[i].Kind = model.WinnerKindWinner
			return nil
		}
	}
	return model.ErrWinnerNotFound
}

// fakeDrawsRepo stores the draw lifecycle in memory.
type fakeDrawsRepo struct {
	draws        map[uuid.UUID]*model.Draw
	drawNotified map[uuid.UUID]bool
}

func newFakeDrawsRepo(draws ...*model.Draw) *fakeDrawsRepo {
	repo := &fakeDrawsRepo{
		draws:        make(map[uuid.UUID]*model.Draw, len(draws)),
		drawNotified: make(map[uuid.UUID]bool),
	}
	for _, draw := range draws {
		repo.draws[draw.ID] = draw
	}
	return repo
}

func (f *fakeDrawsRepo) DrawByID(_ context.Context, id uuid.UUID) (*model.Draw, error) {
	if draw, found := f.draws[id]; found {
		return draw, nil
	}
	return nil, model.ErrDrawNotFound
}

func (f *fakeDrawsRepo) Approve(_ context.Context, id uuid.UUID, approvedBy string) (*model.Draw, error) {
	draw, found := f.draws[id]
	if !found {
		return nil, model.ErrDrawNotFound
	}
	if draw.Status != model.DrawStatusDrawn {
		return nil, model.ErrInvalidDrawStatus
	}
	draw.Status = model.DrawStatusApproved
	draw.ApprovedBy = approvedBy
	return draw, nil
}

func (f *fakeDrawsRepo) MarkDrawNotified(_ context.Context, id uuid.UUID) error {
	f.drawNotified[id] = true
	return nil
}

// fakeRaffleRepo backs raffle tests.
type fakeRaffleRepo struct {
	draws   map[string]*model.Draw // by kind
	entries []uuid.UUID
	saved   []model.Winner
}

func (f *fakeRaffleRepo) GetOrCreateFinal(_ context.Context, day time.Time, winners, reserve int) (*model.Draw, error) {
	if draw, found := f.draws[string(model.DrawKindFinal)]; found {
		return draw, nil
	}
	draw := &model.Draw{
		ID:           uuid.New(),
		Kind:         model.DrawKindFinal,
		DrawDate:     day,
		Status:       model.DrawStatusPending,
		WinnersCount: winners,
		ReserveCount: reserve,
	}
	f.draws[string(model.DrawKindFinal)] = draw
	return draw, nil
}

func (f *fakeRaffleRepo) ByDate(_ context.Context, kind model.DrawKind, _ time.Time) (*model.Draw, error) {
	if draw, found := f.draws[string(kind)]; found {
		return draw, nil
	}
	return nil, model.ErrDrawNotFound
}

func (f *fakeRaffleRepo) EntryUserIDs(_ context.Context, _ uuid.UUID) ([]uuid.UUID, error) {
	return f.entries, nil
}

func (f *fakeRaffleRepo) SaveDrawn(_ context.Context, draw *model.Draw, winners []model.Winner) error {
	if draw.Status != model.DrawStatusPending {
		return model.ErrInvalidDrawStatus
	}
	draw.Status = model.DrawStatusDrawn
	f.saved = append(f.saved, winners...)
	return nil
}

// fakeRaffleChances lists fixed final-draw tickets.
type fakeRaffleChances struct {
	tickets []uuid.UUID
}

func (f *fakeRaffleChances) UserTickets(_ context.Context) ([]uuid.UUID, error) {
	return f.tickets, nil
}
