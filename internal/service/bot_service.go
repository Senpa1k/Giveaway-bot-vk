package service

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/Senpa1k/Giveaway-bot-vk/internal/lib/logger"
	"github.com/Senpa1k/Giveaway-bot-vk/internal/model"
	"github.com/Senpa1k/Giveaway-bot-vk/internal/vkapi"

	"github.com/google/uuid"
)

// eventQueueSize bounds the events waiting for the single worker.
const eventQueueSize = 1024

// BotRepository is the user storage used by the bot router.
type BotRepository interface {
	UpsertByVKID(ctx context.Context, vkID int64) (*model.User, error)
	SetState(ctx context.Context, id uuid.UUID, state model.UserState) error
}

// BotClaims is the winner contact flow used by the bot router.
type BotClaims interface {
	StartClaim(ctx context.Context, user *model.User) (*Reply, error)
	SubmitName(ctx context.Context, user *model.User, name string) (*Reply, error)
	ProcessConsent(ctx context.Context, user *model.User, agreed bool) (*Reply, error)
}

// BotService routes incoming messages to the user flows of the technical
// spec: menu, promocodes, daily draw, chances, rules and prize claims.
type BotService struct {
	users      BotRepository
	sender     Sender
	promo      *PromoService
	join       *JoinService
	chances    *ChancesService
	claims     BotClaims
	banner     string
	maxChances int
	today      func() time.Time
	events     chan vkapi.IncomingMessage
	log        *slog.Logger
}

// NewBotService constructs the bot router. The today function returns the
// current campaign day; the service wires it from the application config.
func NewBotService(
	users BotRepository,
	sender Sender,
	promo *PromoService,
	join *JoinService,
	chances *ChancesService,
	claims BotClaims,
	banner string,
	maxChances int,
	today func() time.Time,
	log *slog.Logger,
) *BotService {
	return &BotService{
		users:      users,
		sender:     sender,
		promo:      promo,
		join:       join,
		chances:    chances,
		claims:     claims,
		banner:     banner,
		maxChances: maxChances,
		today:      today,
		events:     make(chan vkapi.IncomingMessage, eventQueueSize),
		log:        log,
	}
}

// Start runs the single event worker. One worker keeps the messages of a
// user ordered; recoveries isolate broken messages from the loop.
func (s *BotService) Start(ctx context.Context) {
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case msg := <-s.events:
				s.handleSafely(ctx, msg)
			}
		}
	}()
}

// Enqueue takes an incoming message for processing; false means the queue
// is full and the message is dropped.
func (s *BotService) Enqueue(msg vkapi.IncomingMessage) bool {
	select {
	case s.events <- msg:
		return true
	default:
		s.log.Error("event queue is full, message dropped",
			slog.Int64("vk_id", msg.FromID),
			slog.String("text", msg.TrimmedText()),
		)
		return false
	}
}

// handleSafely processes one message, recovering a panic so the worker
// keeps serving the queue.
func (s *BotService) handleSafely(ctx context.Context, msg vkapi.IncomingMessage) {
	defer func() {
		if recovered := recover(); recovered != nil {
			s.log.Error("message handling panicked",
				slog.Any("panic", recovered),
				slog.Int64("vk_id", msg.FromID),
			)
		}
	}()
	if err := s.handle(ctx, msg); err != nil {
		s.log.Error("message handling failed", logger.Err(err), slog.Int64("vk_id", msg.FromID))
	}
}

// handle routes one message to its flow and sends the reply.
func (s *BotService) handle(ctx context.Context, msg vkapi.IncomingMessage) error {
	if !msg.IsPrivate() {
		return nil
	}

	user, err := s.users.UpsertByVKID(ctx, msg.FromID)
	if err != nil {
		return fmt.Errorf("upsert user %d: %w", msg.FromID, err)
	}

	cmd := msg.Command()
	if cmd == "" {
		cmd = CommandByText(msg.Text)
	}

	reply, err := s.route(ctx, user, msg, cmd)
	if err != nil {
		return err
	}
	s.send(ctx, user, reply)
	return nil
}

// route resolves the reply of a message: contact-flow states first, then
// button commands and text aliases.
func (s *BotService) route(
	ctx context.Context,
	user *model.User,
	msg vkapi.IncomingMessage,
	cmd string,
) (*Reply, error) {
	switch user.State.State {
	case model.StateAwaitingName:
		if cmd == "" {
			if text := msg.TrimmedText(); text != "" {
				return s.claims.SubmitName(ctx, user, text)
			}
		}
		// A button press restarts the claim dialog from the name question.
		return s.claims.StartClaim(ctx, user)

	case model.StateAwaitingConsent:
		switch cmd {
		case CmdConsentYes:
			return s.claims.ProcessConsent(ctx, user, true)
		case CmdConsentNo:
			return s.claims.ProcessConsent(ctx, user, false)
		default:
			return claimPrompt(user.State.Name), nil
		}
	case model.StateNone:
		// Regular command routing below.
	default:
		// Unknown state values fall back to a clean menu.
	}

	switch cmd {
	case CmdStart:
		return welcomeReply(s.banner), nil
	case CmdPromo:
		return s.promo.Handle(ctx, user, s.today())
	case CmdJoin:
		return s.join.Screen(ctx, s.today())
	case CmdCheck:
		return s.join.Check(ctx, user, s.today())
	case CmdHowTo:
		return howToReply(s.maxChances), nil
	case CmdChances:
		return s.chances.MyChances(ctx, user.ID, s.today())
	case CmdRules:
		return rulesReply(s.maxChances), nil
	case CmdClaim:
		return s.claims.StartClaim(ctx, user)
	default:
		return helpReply(), nil
	}
}

// send delivers the reply, tolerating users who cannot receive messages.
func (s *BotService) send(ctx context.Context, user *model.User, reply *Reply) {
	if err := s.sender.MessagesSend(ctx, user.VKID, reply.Text, reply.Keyboard, reply.Attachment); err != nil {
		if vkapi.IsCantSend(err) {
			s.log.Warn("cannot message the user", logger.Err(err), slog.Int64("vk_id", user.VKID))
			return
		}
		s.log.Error("send message failed", logger.Err(err), slog.Int64("vk_id", user.VKID))
	}
}
