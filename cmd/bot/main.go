// Package main starts the giveaway bot process.
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	// The embedded time zone database keeps the campaign time zone working
	// in slim containers without tzdata.
	_ "time/tzdata"

	"github.com/Senpa1k/Giveaway-bot-vk/internal/config"
	"github.com/Senpa1k/Giveaway-bot-vk/internal/http-server/handlers"
	"github.com/Senpa1k/Giveaway-bot-vk/internal/lib/logger"
	"github.com/Senpa1k/Giveaway-bot-vk/internal/scheduler"
	"github.com/Senpa1k/Giveaway-bot-vk/internal/service"
	"github.com/Senpa1k/Giveaway-bot-vk/internal/storage"
	"github.com/Senpa1k/Giveaway-bot-vk/internal/storage/postgres"
	"github.com/Senpa1k/Giveaway-bot-vk/internal/vkapi"
)

func main() {
	if err := run(); err != nil {
		os.Exit(1)
	}
}

func run() error {
	cfg := config.MustLoad()
	log := logger.New(cfg.LogLevel, cfg.Env)

	log.Info("starting giveaway bot",
		slog.String("env", cfg.Env),
		slog.String("mode", cfg.VK.Mode),
		slog.String("log_level", cfg.LogLevel),
	)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := postgres.NewPool(ctx, cfg.Database)
	if err != nil {
		log.Error("failed to connect to PostgreSQL", logger.Err(err))
		return err
	}
	defer pool.Close()

	repo := storage.NewRepository(pool)
	vk := vkapi.New(cfg.VK.Token, cfg.VK.APIVersion, cfg.VK.SendInterval)

	today := func() time.Time {
		day, err := cfg.Today(time.Now())
		if err != nil {
			log.Error("resolve campaign day failed", logger.Err(err))
			return time.Now()
		}
		return day
	}

	services := service.New(repo, service.Dependencies{
		QueryTimeout: cfg.Database.QueryTimeout,
		Sender:       vk,
		Subscriber:   vk,
		Banner:       cfg.Banner,
		Campaign: service.CampaignView{
			MainPrize:     cfg.Campaign.MainPrize,
			MaxChances:    cfg.Campaign.MaxChances,
			RemainingDays: cfg.RemainingDays,
		},
		Final: service.FinalSettings{
			Winners:     cfg.Draw.FinalWinners,
			TicketsEach: cfg.Draw.TicketsPerWinner,
			Reserve:     cfg.Draw.ReserveCount,
			MainPrize:   cfg.Campaign.MainPrize,
		},
		StatsTimezone: cfg.Draw.Timezone,
		Today:         today,
		Log:           log,
	})

	// The single event worker keeps the messages of a user ordered.
	services.BotService.Start(ctx)

	handleEvent := func(ctx context.Context, event vkapi.Event) {
		if event.Type != vkapi.EventMessageNew {
			return
		}
		message, err := event.Message()
		if err != nil {
			log.Error("decode message event failed", logger.Err(err))
			return
		}
		services.BotService.Enqueue(message)
	}

	if cfg.VK.Mode == config.ModeLongPoll {
		longPoll := vkapi.NewLongPoll(vk, cfg.VK.GroupID, cfg.VK.LongPollWait, log)
		go func() {
			if err := longPoll.Run(ctx, handleEvent); err != nil && ctx.Err() == nil {
				log.Error("long poll stopped", logger.Err(err))
			}
		}()
	}

	// The scheduler runs the daily draw automatically; the final draw runs
	// on the last campaign day when auto_final is enabled.
	drawHour, drawMinute, err := cfg.Draw.Clock()
	if err != nil {
		log.Error("resolve draw clock failed", logger.Err(err))
		return err
	}
	drawLocation, err := cfg.Draw.Location()
	if err != nil {
		log.Error("resolve draw timezone failed", logger.Err(err))
		return err
	}
	var finalDay time.Time
	if cfg.Draw.AutoFinal {
		if _, end, spanErr := cfg.Campaign.Span(); spanErr == nil {
			finalDay = end
		}
	}
	go scheduler.New(services.RaffleService, drawHour, drawMinute, drawLocation, finalDay, log).Run(ctx)

	server := &http.Server{
		Addr:              cfg.HTTPServer.Address,
		Handler:           handlers.NewServer(log, services, cfg.VK.Confirmation, cfg.VK.Secret, cfg.Admin.Token, today).Routes(),
		ReadHeaderTimeout: cfg.HTTPServer.ReadHeaderTimeout,
		ReadTimeout:       cfg.HTTPServer.ReadTimeout,
		WriteTimeout:      cfg.HTTPServer.WriteTimeout,
		IdleTimeout:       cfg.HTTPServer.IdleTimeout,
	}

	serverErr := make(chan error, 1)
	go func() {
		log.Info("starting HTTP server", slog.String("address", cfg.HTTPServer.Address))
		serverErr <- server.ListenAndServe()
	}()

	var listenErr error
	select {
	case <-ctx.Done():
		log.Info("shutdown signal received")
	case err := <-serverErr:
		if !errors.Is(err, http.ErrServerClosed) {
			log.Error("HTTP server stopped", logger.Err(err))
			listenErr = err
		}
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Error("graceful shutdown failed", logger.Err(err))
		_ = server.Close()
		return err
	}
	log.Info("giveaway bot stopped")
	return listenErr
}
