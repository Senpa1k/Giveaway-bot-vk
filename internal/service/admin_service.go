package service

import (
	"context"
	"fmt"
	"time"

	"github.com/Senpa1k/Giveaway-bot-vk/internal/model"

	"github.com/google/uuid"
)

// AdminRepository is the storage used by the admin API operations.
type AdminRepository interface {
	Create(ctx context.Context, partner *model.Partner) error
	PartnerByID(ctx context.Context, id uuid.UUID) (*model.Partner, error)
	ListPartners(ctx context.Context) ([]model.Partner, error)
	ImportCodes(ctx context.Context, partnerID uuid.UUID, codes []string) (int64, error)
	ListDraws(ctx context.Context) ([]model.Draw, error)
	DrawByID(ctx context.Context, id uuid.UUID) (*model.Draw, error)
	Entries(ctx context.Context, drawID uuid.UUID) ([]model.DrawEntry, error)
	Approve(ctx context.Context, id uuid.UUID, approvedBy string) (*model.Draw, error)
	ByIDs(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]model.User, error)
	ListByDraw(ctx context.Context, drawID uuid.UUID) ([]model.Winner, error)
}

// AdminService backs the admin API: partner setup, promocode uploads and
// draw review. The operations are thin, but keep the storage behind the
// service layer like every other handler dependency.
type AdminService struct {
	repo         AdminRepository
	queryTimeout time.Duration
}

// NewAdminService constructs the admin service.
func NewAdminService(repo AdminRepository, queryTimeout time.Duration) *AdminService {
	return &AdminService{repo: repo, queryTimeout: queryTimeout}
}

// Create registers the partner of one action day.
func (s *AdminService) Create(ctx context.Context, partner *model.Partner) error {
	queryCtx, cancel := context.WithTimeout(ctx, s.queryTimeout)
	defer cancel()

	if err := s.repo.Create(queryCtx, partner); err != nil {
		return fmt.Errorf("create partner: %w", err)
	}
	return nil
}

// PartnerByID returns the partner or model.ErrPartnerNotFound.
func (s *AdminService) PartnerByID(ctx context.Context, id uuid.UUID) (*model.Partner, error) {
	queryCtx, cancel := context.WithTimeout(ctx, s.queryTimeout)
	defer cancel()

	partner, err := s.repo.PartnerByID(queryCtx, id)
	if err != nil {
		return nil, fmt.Errorf("find partner: %w", err)
	}
	return partner, nil
}

// ListPartners returns all partners ordered by their day.
func (s *AdminService) ListPartners(ctx context.Context) ([]model.Partner, error) {
	queryCtx, cancel := context.WithTimeout(ctx, s.queryTimeout)
	defer cancel()

	partners, err := s.repo.ListPartners(queryCtx)
	if err != nil {
		return nil, fmt.Errorf("list partners: %w", err)
	}
	return partners, nil
}

// ImportCodes uploads a batch of free partner codes.
func (s *AdminService) ImportCodes(ctx context.Context, partnerID uuid.UUID, codes []string) (int64, error) {
	queryCtx, cancel := context.WithTimeout(ctx, s.queryTimeout)
	defer cancel()

	imported, err := s.repo.ImportCodes(queryCtx, partnerID, codes)
	if err != nil {
		return 0, fmt.Errorf("import promocodes: %w", err)
	}
	return imported, nil
}

// ListDraws returns all draws newest first.
func (s *AdminService) ListDraws(ctx context.Context) ([]model.Draw, error) {
	queryCtx, cancel := context.WithTimeout(ctx, s.queryTimeout)
	defer cancel()

	draws, err := s.repo.ListDraws(queryCtx)
	if err != nil {
		return nil, fmt.Errorf("list draws: %w", err)
	}
	return draws, nil
}

// DrawByID returns the draw or model.ErrDrawNotFound.
func (s *AdminService) DrawByID(ctx context.Context, id uuid.UUID) (*model.Draw, error) {
	queryCtx, cancel := context.WithTimeout(ctx, s.queryTimeout)
	defer cancel()

	draw, err := s.repo.DrawByID(queryCtx, id)
	if err != nil {
		return nil, fmt.Errorf("find draw: %w", err)
	}
	return draw, nil
}

// Entries lists the participants of a draw for review and exports.
func (s *AdminService) Entries(ctx context.Context, drawID uuid.UUID) ([]model.DrawEntry, error) {
	queryCtx, cancel := context.WithTimeout(ctx, s.queryTimeout)
	defer cancel()

	entries, err := s.repo.Entries(queryCtx, drawID)
	if err != nil {
		return nil, fmt.Errorf("list draw entries: %w", err)
	}
	return entries, nil
}

// Approve confirms the drawn results on behalf of an administrator.
func (s *AdminService) Approve(ctx context.Context, id uuid.UUID, approvedBy string) (*model.Draw, error) {
	queryCtx, cancel := context.WithTimeout(ctx, s.queryTimeout)
	defer cancel()

	draw, err := s.repo.Approve(queryCtx, id, approvedBy)
	if err != nil {
		return nil, fmt.Errorf("approve draw: %w", err)
	}
	return draw, nil
}

// ByIDs loads users for exports and winner review.
func (s *AdminService) ByIDs(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]model.User, error) {
	queryCtx, cancel := context.WithTimeout(ctx, s.queryTimeout)
	defer cancel()

	users, err := s.repo.ByIDs(queryCtx, ids)
	if err != nil {
		return nil, fmt.Errorf("load users: %w", err)
	}
	return users, nil
}

// ListByDraw returns the winners of a draw for review and exports.
func (s *AdminService) ListByDraw(ctx context.Context, drawID uuid.UUID) ([]model.Winner, error) {
	queryCtx, cancel := context.WithTimeout(ctx, s.queryTimeout)
	defer cancel()

	winners, err := s.repo.ListByDraw(queryCtx, drawID)
	if err != nil {
		return nil, fmt.Errorf("list winners: %w", err)
	}
	return winners, nil
}
