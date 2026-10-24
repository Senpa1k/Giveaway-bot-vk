package service

import (
	"context"
	"fmt"
	"time"
)

// HealthRepository checks storage availability.
type HealthRepository interface {
	Ping(ctx context.Context) error
}

// HealthService checks that the process dependencies answer.
type HealthService struct {
	repo         HealthRepository
	queryTimeout time.Duration
}

// NewHealthService constructs the health service.
func NewHealthService(repo HealthRepository, queryTimeout time.Duration) *HealthService {
	return &HealthService{repo: repo, queryTimeout: queryTimeout}
}

// Ping reports whether PostgreSQL is reachable.
func (s *HealthService) Ping(ctx context.Context) error {
	pingCtx, cancel := context.WithTimeout(ctx, s.queryTimeout)
	defer cancel()

	if err := s.repo.Ping(pingCtx); err != nil {
		return fmt.Errorf("ping storage: %w", err)
	}
	return nil
}
