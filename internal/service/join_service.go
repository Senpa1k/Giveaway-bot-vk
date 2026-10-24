package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Senpa1k/Giveaway-bot-vk/internal/model"

	"github.com/google/uuid"
)

// JoinRepository is the storage used by the daily draw participation.
type JoinRepository interface {
	GetOrCreateDaily(ctx context.Context, partner *model.Partner, day time.Time) (*model.Draw, error)
	HasEntry(ctx context.Context, drawID, userID uuid.UUID) (bool, error)
	CreateEntry(ctx context.Context, drawID, userID uuid.UUID) error
}

// JoinService runs the daily draw flow: show the partner, verify the
// subscription and register the user exactly once per day.
type JoinService struct {
	repo          JoinRepository
	partners      PartnerOfTheDay
	subscriptions Subscriber
	chances       *ChancesService
	queryTimeout  time.Duration
}

// NewJoinService constructs the daily draw participation service.
func NewJoinService(
	repo JoinRepository,
	partners PartnerOfTheDay,
	subscriptions Subscriber,
	chances *ChancesService,
	queryTimeout time.Duration,
) *JoinService {
	return &JoinService{
		repo:          repo,
		partners:      partners,
		subscriptions: subscriptions,
		chances:       chances,
		queryTimeout:  queryTimeout,
	}
}

// Screen shows the subscription instruction of the day partner.
func (s *JoinService) Screen(ctx context.Context, today time.Time) (*Reply, error) {
	partner, err := s.partners.ByDay(ctx, today)
	if err != nil {
		if errors.Is(err, model.ErrPartnerNotFound) {
			return noPartnerReply(), nil
		}
		return nil, fmt.Errorf("find partner of the day: %w", err)
	}
	return joinScreenReply(partner), nil
}

// Check verifies the subscription and registers the user in the draw of the
// day. The number of retries is unlimited.
func (s *JoinService) Check(ctx context.Context, user *model.User, today time.Time) (*Reply, error) {
	queryCtx, cancel := context.WithTimeout(ctx, s.queryTimeout)
	defer cancel()

	partner, err := s.partners.ByDay(queryCtx, today)
	if err != nil {
		if errors.Is(err, model.ErrPartnerNotFound) {
			return noPartnerReply(), nil
		}
		return nil, fmt.Errorf("find partner of the day: %w", err)
	}

	draw, err := s.repo.GetOrCreateDaily(queryCtx, partner, today)
	if err != nil {
		return nil, fmt.Errorf("get daily draw: %w", err)
	}

	registered, err := s.repo.HasEntry(queryCtx, draw.ID, user.ID)
	if err != nil {
		return nil, fmt.Errorf("check draw entry: %w", err)
	}
	if registered {
		return alreadyJoinedReply(), nil
	}

	member, err := s.subscriptions.IsMember(ctx, partner.VKGroupID, user.VKID)
	if err != nil {
		// A VK failure is not the user's fault: ask to retry shortly.
		return subscriptionFailedReply(), nil
	}
	if !member {
		return notSubscribedReply(partner), nil
	}

	if err = s.repo.CreateEntry(queryCtx, draw.ID, user.ID); err != nil {
		if errors.Is(err, model.ErrAlreadyRegistered) {
			return alreadyJoinedReply(), nil
		}
		return nil, fmt.Errorf("create draw entry: %w", err)
	}

	if err = s.chances.Add(ctx, user.ID, today, model.ChanceSourceDraw); err != nil {
		return nil, err
	}
	total, err := s.chances.Count(ctx, user.ID)
	if err != nil {
		return nil, err
	}
	return joinedReply(partner, total), nil
}
