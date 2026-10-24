// Package service contains application logic between HTTP and storage.
package service

import (
	"log/slog"
	"time"
)

// Repositories lists the storage operations needed to build all services.
type Repositories interface {
	HealthRepository
	BotRepository
	PartnerOfTheDay
	PromoRepository
	JoinRepository
	ChancesRepository
	RaffleRepository
	RaffleChances
	WinnerRepository
	WinnerUsers
	WinnerDraws
	WinnerPartners
	StatsRepository
	BroadcastUsers
	AdminRepository
}

// Dependencies carries the settings shared by the service constructors.
type Dependencies struct {
	// QueryTimeout bounds single storage operations.
	QueryTimeout time.Duration
	// Sender delivers bot messages; Subscriber checks VK subscriptions.
	Sender     Sender
	Subscriber Subscriber
	// Banner is the VK attachment of the welcome message.
	Banner string
	// Campaign describes the action for the «Мои шансы» screen.
	Campaign CampaignView
	// Final configures the final draw.
	Final FinalSettings
	// StatsTimezone fixes the day boundaries of the analytics series.
	StatsTimezone string
	// Today returns the current campaign day.
	Today func() time.Time
	// Log records background outcome of winner notifications.
	Log *slog.Logger
}

// Services groups application services for HTTP wiring.
type Services struct {
	*HealthService
	*AdminService
	*BotService
	*PromoService
	*JoinService
	*ChancesService
	*RaffleService
	*WinnerService
	*StatsService
	*BroadcastService
}

// New constructs all services from the repository bundle.
func New(repo Repositories, deps Dependencies) *Services {
	chances := NewChancesService(repo, deps.Campaign, deps.QueryTimeout)
	promo := NewPromoService(repo, repo, chances, deps.QueryTimeout)
	join := NewJoinService(repo, repo, deps.Subscriber, chances, deps.QueryTimeout)
	winners := NewWinnerService(repo, repo, repo, repo, deps.Sender, deps.QueryTimeout, deps.Log)
	bot := NewBotService(repo, deps.Sender, promo, join, chances, winners,
		deps.Banner, deps.Campaign.MaxChances, deps.Today, deps.Log)
	raffle := NewRaffleService(repo, repo, repo, deps.Final, deps.QueryTimeout)

	return &Services{
		HealthService:    NewHealthService(repo, deps.QueryTimeout),
		AdminService:     NewAdminService(repo, deps.QueryTimeout),
		BotService:       bot,
		PromoService:     promo,
		JoinService:      join,
		ChancesService:   chances,
		RaffleService:    raffle,
		WinnerService:    winners,
		StatsService:     NewStatsService(repo, deps.StatsTimezone, deps.QueryTimeout),
		BroadcastService: NewBroadcastService(repo, deps.Sender, deps.Log),
	}
}
