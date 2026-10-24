package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/Senpa1k/Giveaway-bot-vk/internal/lib/logger"
	"github.com/Senpa1k/Giveaway-bot-vk/internal/model"
	"github.com/Senpa1k/Giveaway-bot-vk/internal/vkapi"

	"github.com/google/uuid"
)

// maxContactNameLen bounds the name answer of the claim dialog.
const maxContactNameLen = 100

// WinnerRepository is the storage used by the winner flow.
type WinnerRepository interface {
	ListByDraw(ctx context.Context, drawID uuid.UUID) ([]model.Winner, error)
	ActiveClaim(ctx context.Context, userID uuid.UUID) (*model.Winner, error)
	WinnerByID(ctx context.Context, id uuid.UUID) (*model.Winner, error)
	MarkContacted(ctx context.Context, id uuid.UUID) error
	ConfirmContact(ctx context.Context, id uuid.UUID, name string) error
	MarkWinnersNotified(ctx context.Context, ids []uuid.UUID) error
	MarkMissed(ctx context.Context, id uuid.UUID) error
	FirstReserve(ctx context.Context, drawID uuid.UUID) (*model.Winner, error)
	PromoteReserve(ctx context.Context, id uuid.UUID) error
}

// WinnerUsers loads users and stores dialog states for the claim flow.
type WinnerUsers interface {
	ByIDs(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]model.User, error)
	SetState(ctx context.Context, id uuid.UUID, state model.UserState) error
}

// WinnerDraws completes the draw lifecycle around notifications.
type WinnerDraws interface {
	DrawByID(ctx context.Context, id uuid.UUID) (*model.Draw, error)
	Approve(ctx context.Context, id uuid.UUID, approvedBy string) (*model.Draw, error)
	MarkDrawNotified(ctx context.Context, id uuid.UUID) error
}

// WinnerPartners loads draw partners for winner messages.
type WinnerPartners interface {
	PartnerByID(ctx context.Context, id uuid.UUID) (*model.Partner, error)
}

// WinnerService notifies winners, collects their contact data and moves
// missed prizes to reserve winners.
type WinnerService struct {
	repo         WinnerRepository
	users        WinnerUsers
	draws        WinnerDraws
	partners     WinnerPartners
	sender       Sender
	queryTimeout time.Duration
	log          *slog.Logger
}

// NewWinnerService constructs the winner service.
func NewWinnerService(
	repo WinnerRepository,
	users WinnerUsers,
	draws WinnerDraws,
	partners WinnerPartners,
	sender Sender,
	queryTimeout time.Duration,
	log *slog.Logger,
) *WinnerService {
	return &WinnerService{
		repo:         repo,
		users:        users,
		draws:        draws,
		partners:     partners,
		sender:       sender,
		queryTimeout: queryTimeout,
		log:          log,
	}
}

// NotifyDraw sends the winner messages of an approved draw and completes its
// lifecycle. Results must be confirmed by an administrator first; reserve
// winners stay silent until they are promoted.
func (s *WinnerService) NotifyDraw(ctx context.Context, drawID uuid.UUID) error {
	queryCtx, cancel := context.WithTimeout(ctx, s.queryTimeout)
	defer cancel()

	draw, err := s.draws.DrawByID(queryCtx, drawID)
	if err != nil {
		return fmt.Errorf("find draw: %w", err)
	}
	if draw.Status != model.DrawStatusApproved {
		return model.ErrInvalidDrawStatus
	}

	winners, err := s.repo.ListByDraw(queryCtx, drawID)
	if err != nil {
		return fmt.Errorf("list winners: %w", err)
	}

	toNotify := make([]model.Winner, 0, len(winners))
	for _, winner := range winners {
		if winner.Kind == model.WinnerKindWinner && winner.Status == model.WinnerStatusAwaitingResponse {
			toNotify = append(toNotify, winner)
		}
	}

	userIDs := make([]uuid.UUID, 0, len(toNotify))
	for _, winner := range toNotify {
		userIDs = append(userIDs, winner.UserID)
	}
	users, err := s.users.ByIDs(queryCtx, userIDs)
	if err != nil {
		return fmt.Errorf("load winners: %w", err)
	}

	notified := make([]uuid.UUID, 0, len(toNotify))
	for _, winner := range toNotify {
		user, found := users[winner.UserID]
		if !found {
			s.log.Error("winner has no user row", slog.String("winner_id", winner.ID.String()))
			continue
		}
		reply, err := s.winnerReply(queryCtx, draw, &winner)
		if err != nil {
			s.log.Error("build winner message failed", logger.Err(err), slog.String("winner_id", winner.ID.String()))
			continue
		}
		if err = s.sender.MessagesSend(ctx, user.VKID, reply.Text, reply.Keyboard, ""); err != nil {
			if vkapi.IsCantSend(err) {
				s.log.Warn("cannot message the winner", logger.Err(err), slog.Int64("vk_id", user.VKID))
				continue
			}
			s.log.Error("notify winner failed", logger.Err(err), slog.String("winner_id", winner.ID.String()))
			continue
		}
		notified = append(notified, winner.ID)
	}

	if err = s.repo.MarkWinnersNotified(queryCtx, notified); err != nil {
		return fmt.Errorf("mark winners notified: %w", err)
	}
	if err = s.draws.MarkDrawNotified(queryCtx, drawID); err != nil {
		return fmt.Errorf("mark draw notified: %w", err)
	}
	return nil
}

// StartClaim begins the contact dialog from the «Получить приз» button.
func (s *WinnerService) StartClaim(ctx context.Context, user *model.User) (*Reply, error) {
	queryCtx, cancel := context.WithTimeout(ctx, s.queryTimeout)
	defer cancel()

	claim, err := s.repo.ActiveClaim(queryCtx, user.ID)
	if err != nil {
		if errors.Is(err, model.ErrWinnerNotFound) {
			return noClaimReply(), nil
		}
		return nil, fmt.Errorf("find active claim: %w", err)
	}

	if claim.Status == model.WinnerStatusAwaitingResponse {
		if err = s.repo.MarkContacted(queryCtx, claim.ID); err != nil {
			return nil, fmt.Errorf("mark claim contacted: %w", err)
		}
	}

	if err = s.users.SetState(queryCtx, user.ID, model.UserState{State: model.StateAwaitingName, ID: claim.ID}); err != nil {
		return nil, fmt.Errorf("set claim state: %w", err)
	}
	return claimStartReply(), nil
}

// SubmitName validates the name answer, stores it in the dialog state and
// asks for the data-transfer consent.
func (s *WinnerService) SubmitName(ctx context.Context, user *model.User, name string) (*Reply, error) {
	name = normalizeName(name)
	if utf8.RuneCountInString(name) < 2 {
		return claimNameRetryReply(), nil
	}

	queryCtx, cancel := context.WithTimeout(ctx, s.queryTimeout)
	defer cancel()

	state := model.UserState{State: model.StateAwaitingConsent, ID: user.State.ID, Name: name}
	if err := s.users.SetState(queryCtx, user.ID, state); err != nil {
		return nil, fmt.Errorf("store claim name: %w", err)
	}
	return claimPrompt(name), nil
}

// ProcessConsent completes the claim with the saved contact data or cancels
// it, leaving the prize available for the deadline transfer.
func (s *WinnerService) ProcessConsent(ctx context.Context, user *model.User, agreed bool) (*Reply, error) {
	queryCtx, cancel := context.WithTimeout(ctx, s.queryTimeout)
	defer cancel()

	if agreed {
		if err := s.repo.ConfirmContact(queryCtx, user.State.ID, user.State.Name); err != nil {
			return nil, fmt.Errorf("confirm winner contact: %w", err)
		}
	}
	if err := s.users.SetState(queryCtx, user.ID, model.UserState{State: model.StateNone}); err != nil {
		return nil, fmt.Errorf("clear claim state: %w", err)
	}

	if agreed {
		return claimDoneReply(), nil
	}
	return claimRefusedReply(), nil
}

// Miss closes the prize of a winner who missed the response deadline and
// promotes the first reserve winner. It returns the promoted winner, or nil
// when the reserves are exhausted.
func (s *WinnerService) Miss(ctx context.Context, winnerID uuid.UUID) (*model.Winner, error) {
	queryCtx, cancel := context.WithTimeout(ctx, s.queryTimeout)
	defer cancel()

	missed, err := s.repo.WinnerByID(queryCtx, winnerID)
	if err != nil {
		return nil, fmt.Errorf("find winner: %w", err)
	}
	if err = s.repo.MarkMissed(queryCtx, winnerID); err != nil {
		return nil, fmt.Errorf("mark winner missed: %w", err)
	}

	reserve, err := s.repo.FirstReserve(queryCtx, missed.DrawID)
	if err != nil {
		if errors.Is(err, model.ErrWinnerNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("find reserve winner: %w", err)
	}
	if err = s.repo.PromoteReserve(queryCtx, reserve.ID); err != nil {
		return nil, fmt.Errorf("promote reserve winner: %w", err)
	}

	s.notifyPromoted(ctx, *missed, *reserve)
	reserve.Kind = model.WinnerKindWinner
	return reserve, nil
}

// notifyPromoted sends the winner message to a promoted reserve winner; the
// draw lifecycle is already complete at this point.
func (s *WinnerService) notifyPromoted(ctx context.Context, missed, reserve model.Winner) {
	queryCtx, cancel := context.WithTimeout(ctx, s.queryTimeout)
	defer cancel()

	draw, err := s.draws.DrawByID(queryCtx, missed.DrawID)
	if err != nil {
		s.log.Error("load draw for promoted winner", logger.Err(err))
		return
	}
	reply, err := s.winnerReply(queryCtx, draw, &reserve)
	if err != nil {
		s.log.Error("build promoted winner message", logger.Err(err))
		return
	}
	users, err := s.users.ByIDs(queryCtx, []uuid.UUID{reserve.UserID})
	if err != nil {
		s.log.Error("load promoted winner", logger.Err(err))
		return
	}
	user, found := users[reserve.UserID]
	if !found {
		s.log.Error("promoted winner has no user row", slog.String("winner_id", reserve.ID.String()))
		return
	}
	if err = s.sender.MessagesSend(ctx, user.VKID, reply.Text, reply.Keyboard, ""); err != nil {
		s.log.Error("notify promoted winner failed", logger.Err(err), slog.Int64("vk_id", user.VKID))
	}
	if err = s.repo.MarkWinnersNotified(queryCtx, []uuid.UUID{reserve.ID}); err != nil {
		s.log.Error("mark promoted winner notified", logger.Err(err))
	}
}

// winnerReply builds the winner message of one prize.
func (s *WinnerService) winnerReply(ctx context.Context, draw *model.Draw, winner *model.Winner) (*Reply, error) {
	keyboard := vkapi.NewInlineKeyboard().
		TextRow("Получить приз", CmdClaim, vkapi.ColorPositive)

	if draw.Kind == model.DrawKindFinal {
		return &Reply{
			Text: fmt.Sprintf(
				"Твой главный выход на сцену 🔥\n\n"+
					"Ты выиграл %s!\n\n"+
					"Чтобы подтвердить получение приза, нажми кнопку ниже и оставь необходимые контактные данные.",
				winner.Prize,
			),
			Keyboard: keyboard,
		}, nil
	}

	if draw.PartnerID == nil {
		return nil, fmt.Errorf("daily draw %s has no partner", draw.ID)
	}
	partner, err := s.partners.PartnerByID(ctx, *draw.PartnerID)
	if err != nil {
		return nil, fmt.Errorf("find draw partner: %w", err)
	}
	return &Reply{
		Text: fmt.Sprintf(
			"Ты в числе хедлайнеров дня 🔥\n\n"+
				"Ты выиграл %s от %s.\n\n"+
				"%s",
			winner.Prize, partner.Name, partner.PrizeInstruction,
		),
		Keyboard: keyboard,
	}, nil
}

// normalizeName trims and collapses whitespace of the name answer.
func normalizeName(name string) string {
	return strings.Join(strings.Fields(name), " ")
}

func noClaimReply() *Reply {
	return &Reply{
		Text:     "Приза за тобой пока не числится 🤔\n\nНо розыгрыши идут каждый день — не пропускай!",
		Keyboard: mainMenu(),
	}
}

func claimStartReply() *Reply {
	return &Reply{
		Text: "Поздравляем! 🎉\n\nЧтобы передать тебе приз, напиши имя и фамилию одним сообщением.",
	}
}

func claimNameRetryReply() *Reply {
	return &Reply{
		Text: "Напиши, пожалуйста, имя и фамилию одним сообщением.",
	}
}

func claimDoneReply() *Reply {
	return &Reply{
		Text:     "Готово! ✅\n\nМы передадим твои данные для получения приза — организатор свяжется с тобой.",
		Keyboard: mainMenu(),
	}
}

func claimRefusedReply() *Reply {
	return &Reply{
		Text:     "Хорошо! Если передумаешь — нажми «Получить приз» еще раз.",
		Keyboard: mainMenu(),
	}
}
