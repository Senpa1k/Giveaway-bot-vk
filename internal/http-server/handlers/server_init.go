// Package handlers implements the VK callback endpoint and the admin API.
package handlers

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/Senpa1k/Giveaway-bot-vk/internal/model"
	"github.com/Senpa1k/Giveaway-bot-vk/internal/vkapi"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// Health checks process dependencies.
type Health interface {
	Ping(ctx context.Context) error
}

// BotQueue takes incoming bot messages for processing.
type BotQueue interface {
	Enqueue(msg vkapi.IncomingMessage) bool
}

// PartnerAdmin manages the action partners.
type PartnerAdmin interface {
	Create(ctx context.Context, partner *model.Partner) error
	PartnerByID(ctx context.Context, id uuid.UUID) (*model.Partner, error)
	ListPartners(ctx context.Context) ([]model.Partner, error)
}

// PromoImporter uploads promocodes of a partner.
type PromoImporter interface {
	ImportCodes(ctx context.Context, partnerID uuid.UUID, codes []string) (int64, error)
}

// DrawAdmin reads draws and confirms their results.
type DrawAdmin interface {
	ListDraws(ctx context.Context) ([]model.Draw, error)
	DrawByID(ctx context.Context, id uuid.UUID) (*model.Draw, error)
	Entries(ctx context.Context, drawID uuid.UUID) ([]model.DrawEntry, error)
	Approve(ctx context.Context, id uuid.UUID, approvedBy string) (*model.Draw, error)
}

// RaffleRunner runs the daily and the final draws.
type RaffleRunner interface {
	RunDaily(ctx context.Context, day time.Time) (*model.DrawResult, error)
	RunFinal(ctx context.Context, day time.Time) (*model.DrawResult, error)
}

// WinnerNotifier sends winner messages of an approved draw.
type WinnerNotifier interface {
	NotifyDraw(ctx context.Context, drawID uuid.UUID) error
}

// WinnerAdmin reviews winners and moves missed prizes to reserves.
type WinnerAdmin interface {
	ListByDraw(ctx context.Context, drawID uuid.UUID) ([]model.Winner, error)
	Miss(ctx context.Context, winnerID uuid.UUID) (*model.Winner, error)
}

// UserAdmin loads users for exports.
type UserAdmin interface {
	ByIDs(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]model.User, error)
}

// StatsProvider returns the analytics snapshot.
type StatsProvider interface {
	Get(ctx context.Context) (*model.Stats, error)
}

// Broadcaster pushes a message to every user.
type Broadcaster interface {
	Send(ctx context.Context, text string) (sent, failed int, err error)
}

// Services lists the operations used by the HTTP handlers.
//
// Здесь собираются все сервисы воедино
type Services interface {
	Health
	BotQueue
	PartnerAdmin
	PromoImporter
	DrawAdmin
	RaffleRunner
	WinnerNotifier
	WinnerAdmin
	UserAdmin
	StatsProvider
	Broadcaster
}

// Server implements the VK callback endpoint and the admin API.
type Server struct {
	log          *slog.Logger
	services     Services
	confirmation string
	secret       string
	adminToken   string
	today        func() time.Time
}

// NewServer constructs HTTP handlers with all application services. The
// confirmation and secret configure the VK Callback API endpoint; the token
// guards the admin API; today resolves the current campaign day.
func NewServer(log *slog.Logger, services Services, confirmation, secret, adminToken string, today func() time.Time) *Server {
	return &Server{
		log:          log,
		services:     services,
		confirmation: confirmation,
		secret:       secret,
		adminToken:   adminToken,
		today:        today,
	}
}

// Routes builds the HTTP router of the bot.
func (s *Server) Routes() http.Handler {
	r := chi.NewRouter()
	r.NotFound(s.notFound)
	r.MethodNotAllowed(s.methodNotAllowed)

	r.Get("/health", s.health)
	r.Get("/ready", s.readiness)
	r.Post("/callback", s.callback)

	r.Route("/api/admin", func(r chi.Router) {
		r.Use(s.adminAuth)
		r.Post("/partners", s.createPartner)
		r.Get("/partners", s.listPartners)
		r.Post("/partners/{partnerID}/promocodes", s.importPromocodes)
		r.Get("/stats", s.getStats)
		r.Get("/draws", s.listDraws)
		r.Post("/draws/daily", s.runDailyDraw)
		r.Post("/draws/final", s.runFinalDraw)
		r.Get("/draws/{drawID}", s.getDraw)
		r.Get("/draws/{drawID}/export", s.exportDraw)
		r.Post("/draws/{drawID}/approve", s.approveDraw)
		r.Post("/winners/{winnerID}/miss", s.missWinner)
		r.Post("/broadcast", s.broadcast)
	})

	return r
}
