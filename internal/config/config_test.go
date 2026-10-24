package config

import (
	"testing"
	"time"
)

func validConfig() *Config {
	return &Config{
		VK: VK{
			Mode:    ModeLongPoll,
			GroupID: 1,
			Token:   "token",
		},
		Campaign: Campaign{
			StartDate:  "2026-11-01",
			EndDate:    "2026-11-10",
			MaxChances: 10,
		},
		Draw: Draw{
			Time:             "21:00",
			Timezone:         "Europe/Moscow",
			FinalWinners:     5,
			TicketsPerWinner: 2,
			ReserveCount:     3,
		},
		Admin: Admin{Token: "secret"},
	}
}

func TestValidateAcceptsValidConfig(t *testing.T) {
	if err := validConfig().validate(); err != nil {
		t.Fatalf("validate() = %v, want nil", err)
	}
}

func TestValidateRejectsBrokenSettings(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*Config)
	}{
		{"unknown vk mode", func(c *Config) { c.VK.Mode = "sms" }},
		{"longpoll without group", func(c *Config) { c.VK.GroupID = 0 }},
		{"callback without confirmation", func(c *Config) {
			c.VK.Mode = ModeCallback
			c.VK.Confirmation = ""
		}},
		{"bad start date", func(c *Config) { c.Campaign.StartDate = "01.11.2026" }},
		{"end before start", func(c *Config) {
			c.Campaign.StartDate = "2026-11-10"
			c.Campaign.EndDate = "2026-11-01"
		}},
		{"zero max chances", func(c *Config) { c.Campaign.MaxChances = 0 }},
		{"bad draw time", func(c *Config) { c.Draw.Time = "9 вечера" }},
		{"unknown timezone", func(c *Config) { c.Draw.Timezone = "Mars/Olympus" }},
		{"zero final winners", func(c *Config) { c.Draw.FinalWinners = 0 }},
		{"negative reserve count", func(c *Config) { c.Draw.ReserveCount = -1 }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := validConfig()
			tt.mutate(cfg)
			if err := cfg.validate(); err == nil {
				t.Fatal("validate() = nil, want error")
			}
		})
	}
}

func TestRemainingDays(t *testing.T) {
	cfg := validConfig()

	today := time.Date(2026, 11, 1, 23, 30, 0, 0, time.UTC)
	if got := cfg.RemainingDays(today); got != 10 {
		t.Fatalf("RemainingDays(first day) = %d, want 10", got)
	}

	midCampaign := time.Date(2026, 11, 5, 0, 0, 0, 0, time.UTC)
	if got := cfg.RemainingDays(midCampaign); got != 6 {
		t.Fatalf("RemainingDays(mid) = %d, want 6", got)
	}

	after := time.Date(2026, 11, 20, 12, 0, 0, 0, time.UTC)
	if got := cfg.RemainingDays(after); got != 0 {
		t.Fatalf("RemainingDays(after) = %d, want 0", got)
	}
}

func TestTodayUsesDrawTimezone(t *testing.T) {
	cfg := validConfig()

	// 23:30 31 октября в Лондоне — уже 1 ноября в Москве.
	london := time.Date(2026, 10, 31, 23, 30, 0, 0, time.UTC)
	today, err := cfg.Today(london)
	if err != nil {
		t.Fatalf("Today() error: %v", err)
	}
	if got := today.Format(time.DateOnly); got != "2026-11-01" {
		t.Fatalf("Today() = %s, want 2026-11-01", got)
	}
}
