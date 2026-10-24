package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Senpa1k/Giveaway-bot-vk/internal/model"

	"github.com/google/uuid"
)

// PromoRepository is the storage used by the promo service.
type PromoRepository interface {
	LogRequest(ctx context.Context, userID, partnerID uuid.UUID) error
	ActiveForUser(ctx context.Context, userID, partnerID uuid.UUID) (*model.Promocode, error)
	AssignFree(ctx context.Context, partnerID, userID uuid.UUID) (*model.Promocode, error)
}

// PartnerOfTheDay resolves the partner of a day.
type PartnerOfTheDay interface {
	ByDay(ctx context.Context, day time.Time) (*model.Partner, error)
}

// PromoService issues partner promocodes. A repeated press of the day shows
// the code already bound to the user; exhausted codes do not block the draw.
type PromoService struct {
	repo         PromoRepository
	partners     PartnerOfTheDay
	chances      *ChancesService
	queryTimeout time.Duration
}

// NewPromoService constructs the promo service.
func NewPromoService(repo PromoRepository, partners PartnerOfTheDay, chances *ChancesService, queryTimeout time.Duration) *PromoService {
	return &PromoService{repo: repo, partners: partners, chances: chances, queryTimeout: queryTimeout}
}

// Handle serves the «Получить промокод» press of the user.
func (s *PromoService) Handle(ctx context.Context, user *model.User, today time.Time) (*Reply, error) {
	queryCtx, cancel := context.WithTimeout(ctx, s.queryTimeout)
	defer cancel()

	partner, err := s.partners.ByDay(queryCtx, today)
	if err != nil {
		if errors.Is(err, model.ErrPartnerNotFound) {
			return noPartnerReply(), nil
		}
		return nil, fmt.Errorf("find partner of the day: %w", err)
	}

	if err = s.repo.LogRequest(queryCtx, user.ID, partner.ID); err != nil {
		return nil, fmt.Errorf("log promo request: %w", err)
	}

	// A code already bound to the user is shown again instead of issuing
	// a new one, including on repeated presses of the same day.
	existing, err := s.repo.ActiveForUser(queryCtx, user.ID, partner.ID)
	if err == nil {
		return promoSuccessReply(partner, existing.Code), nil
	}
	if !errors.Is(err, model.ErrPromocodeNotFound) {
		return nil, fmt.Errorf("find user promocode: %w", err)
	}

	code, err := s.repo.AssignFree(queryCtx, partner.ID, user.ID)
	if err != nil {
		if errors.Is(err, model.ErrPromocodesDepleted) {
			return promoDepletedReply(partner), nil
		}
		return nil, fmt.Errorf("assign promocode: %w", err)
	}

	if err = s.chances.Add(ctx, user.ID, today, model.ChanceSourcePromo); err != nil {
		return nil, err
	}
	return promoSuccessReply(partner, code.Code), nil
}
