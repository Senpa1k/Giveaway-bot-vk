// Package scheduler runs the daily draw automatically at the configured
// time, so winners are determined without a manual admin action.
package scheduler

import (
	"context"
	"log/slog"
	"time"

	"github.com/Senpa1k/Giveaway-bot-vk/internal/lib/logger"
	"github.com/Senpa1k/Giveaway-bot-vk/internal/model"
	"github.com/Senpa1k/Giveaway-bot-vk/internal/service"
)

// tickInterval is how often the scheduler checks the wall clock; two ticks
// per draw minute are enough and the last-run guard keeps it idempotent.
const tickInterval = 30 * time.Second

// Raffle runs the draws of a day.
type Raffle interface {
	RunDaily(ctx context.Context, day time.Time) (*model.DrawResult, error)
	RunFinal(ctx context.Context, day time.Time) (*model.DrawResult, error)
}

// Scheduler triggers the daily draw at the configured time and, when
// enabled, the final draw on the last campaign day.
type Scheduler struct {
	raffle    Raffle
	hour      int
	minute    int
	loc       *time.Location
	finalDay  time.Time
	lastRunAt string
	log       *slog.Logger
}

// New creates the draw scheduler. A zero finalDay disables the automatic
// final draw, leaving it to the admin API.
func New(raffle Raffle, hour, minute int, loc *time.Location, finalDay time.Time, log *slog.Logger) *Scheduler {
	return &Scheduler{raffle: raffle, hour: hour, minute: minute, loc: loc, finalDay: finalDay, log: log}
}

// Run blocks until the context ends, running the draws of every day.
func (s *Scheduler) Run(ctx context.Context) {
	s.log.Info("draw scheduler started",
		slog.Int("hour", s.hour), slog.Int("minute", s.minute), slog.String("timezone", s.loc.String()))

	ticker := time.NewTicker(tickInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.tick(ctx)
		}
	}
}

// tick runs the draws when the wall clock reaches the configured time.
func (s *Scheduler) tick(ctx context.Context) {
	now := time.Now().In(s.loc)
	if now.Hour() != s.hour || now.Minute() != s.minute {
		return
	}

	today := now.Format(time.DateOnly)
	if s.lastRunAt == today {
		return
	}
	s.lastRunAt = today

	day := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, s.loc)
	s.runDaily(ctx, day)
	if !s.finalDay.IsZero() && !day.Before(s.finalDay) {
		s.runFinal(ctx, s.finalDay)
	}
}

// runDaily runs the day draw, treating empty days as a normal outcome.
func (s *Scheduler) runDaily(ctx context.Context, day time.Time) {
	if _, err := s.raffle.RunDaily(ctx, day); err != nil {
		if service.IsSkippableDrawError(err) {
			s.log.Info("daily draw skipped", logger.Err(err), slog.String("day", day.Format(time.DateOnly)))
			return
		}
		s.log.Error("daily draw failed", logger.Err(err), slog.String("day", day.Format(time.DateOnly)))
	}
}

// runFinal runs the final draw of the campaign.
func (s *Scheduler) runFinal(ctx context.Context, day time.Time) {
	if _, err := s.raffle.RunFinal(ctx, day); err != nil {
		if service.IsSkippableDrawError(err) {
			s.log.Info("final draw skipped", logger.Err(err), slog.String("day", day.Format(time.DateOnly)))
			return
		}
		s.log.Error("final draw failed", logger.Err(err), slog.String("day", day.Format(time.DateOnly)))
	}
}
