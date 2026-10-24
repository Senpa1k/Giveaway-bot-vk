// Package config loads bot settings from YAML and environment variables.
package config

import (
	"errors"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/ilyakaznacheev/cleanenv"
)

// Режимы получения событий MAX: long polling для локальной разработки и
// вебхуки для продакшена с публичным HTTPS-адресом.
const (
	ModeLongPoll = "longpoll"
	ModeWebhook  = "webhook"
)

// Config holds bot settings.
type Config struct {
	Env             string        `yaml:"env" env:"ENV" env-default:"local"`
	LogLevel        string        `yaml:"log_level" env:"LOG_LEVEL" env-default:"info"`
	ShutdownTimeout time.Duration `yaml:"shutdown_timeout" env:"SHUTDOWN_TIMEOUT" env-default:"10s"`
	HTTPServer      HTTPServer    `yaml:"http-server"`
	Database        Database      `yaml:"database"`
	MAX             MAX           `yaml:"max"`
	Campaign        Campaign      `yaml:"campaign"`
	Draw            Draw          `yaml:"draw"`
	Admin           Admin         `yaml:"admin"`
}

// HTTPServer holds HTTP listener settings.
type HTTPServer struct {
	Address           string        `yaml:"address" env:"HTTP_ADDR" env-default:":8080"`
	ReadHeaderTimeout time.Duration `yaml:"read_header_timeout" env:"HTTP_READ_HEADER_TIMEOUT" env-default:"5s"`
	ReadTimeout       time.Duration `yaml:"read_timeout" env:"HTTP_READ_TIMEOUT" env-default:"10s"`
	WriteTimeout      time.Duration `yaml:"write_timeout" env:"HTTP_WRITE_TIMEOUT" env-default:"10s"`
	IdleTimeout       time.Duration `yaml:"idle_timeout" env:"HTTP_IDLE_TIMEOUT" env-default:"60s"`
}

// Database holds PostgreSQL pool settings.
type Database struct {
	URL             string        `yaml:"url" env:"DATABASE_URL" env-required:"true"`
	MaxConns        int32         `yaml:"max_conns" env:"DATABASE_MAX_CONNS" env-default:"10"`
	MinConns        int32         `yaml:"min_conns" env:"DATABASE_MIN_CONNS" env-default:"2"`
	MaxConnLifetime time.Duration `yaml:"max_conn_lifetime" env:"DATABASE_MAX_CONN_LIFETIME" env-default:"30m"`
	ConnectTimeout  time.Duration `yaml:"connect_timeout" env:"DATABASE_CONNECT_TIMEOUT" env-default:"5s"`
	QueryTimeout    time.Duration `yaml:"query_timeout" env:"DATABASE_QUERY_TIMEOUT" env-default:"3s"`
}

// MAX holds MAX Bot API and event settings.
type MAX struct {
	Mode         string        `yaml:"mode" env:"MAX_MODE" env-default:"longpoll"`
	APIURL       string        `yaml:"api_url" env:"MAX_API_URL" env-default:"https://platform-api2.max.ru"`
	AccessToken  string        `yaml:"access_token" env:"MAX_ACCESS_TOKEN" env-required:"true"`
	LongPollWait time.Duration `yaml:"longpoll_wait" env:"MAX_LONGPOLL_WAIT" env-default:"25s"`
	SendInterval time.Duration `yaml:"send_interval" env:"MAX_SEND_INTERVAL" env-default:"600ms"`
}

// Campaign holds the action timeline and prize settings.
type Campaign struct {
	StartDate  string `yaml:"start_date" env:"CAMPAIGN_START_DATE" env-default:""`
	EndDate    string `yaml:"end_date" env:"CAMPAIGN_END_DATE" env-default:""`
	MainPrize  string `yaml:"main_prize" env:"CAMPAIGN_MAIN_PRIZE" env-default:"главный приз акции"`
	MaxChances int    `yaml:"max_chances" env:"CAMPAIGN_MAX_CHANCES" env-default:"10"`
}

// Draw holds daily and final draw settings.
type Draw struct {
	Time             string `yaml:"time" env:"DRAW_TIME" env-default:"21:00"`
	Timezone         string `yaml:"timezone" env:"DRAW_TIMEZONE" env-default:"Europe/Moscow"`
	FinalWinners     int    `yaml:"final_winners" env:"DRAW_FINAL_WINNERS" env-default:"5"`
	TicketsPerWinner int    `yaml:"tickets_per_winner" env:"DRAW_TICKETS_PER_WINNER" env-default:"2"`
	ReserveCount     int    `yaml:"reserve_count" env:"DRAW_RESERVE_COUNT" env-default:"3"`
	AutoFinal        bool   `yaml:"auto_final" env:"DRAW_AUTO_FINAL" env-default:"false"`
}

// Admin holds admin API settings.
type Admin struct {
	Token string `yaml:"token" env:"ADMIN_TOKEN" env-required:"true"`
}

// Load reads CONFIG_PATH when set, then applies environment variables.
func Load() (*Config, error) {
	var cfg Config
	if path := os.Getenv("CONFIG_PATH"); path != "" {
		if err := cleanenv.ReadConfig(path, &cfg); err != nil {
			return nil, fmt.Errorf("read config %q: %w", path, err)
		}
	} else if err := cleanenv.ReadEnv(&cfg); err != nil {
		return nil, fmt.Errorf("read config from environment: %w", err)
	}

	if err := cfg.validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	return &cfg, nil
}

// MustLoad loads config or terminates the process, as in AvitoLab.
func MustLoad() *Config {
	cfg, err := Load()
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}
	return cfg
}

// validate rejects settings that can never produce a working bot.
func (c *Config) validate() error {
	var errs []error

	switch c.MAX.Mode {
	case ModeLongPoll, ModeWebhook:
	default:
		errs = append(errs, fmt.Errorf("max.mode must be %q or %q", ModeLongPoll, ModeWebhook))
	}
	if strings.TrimSpace(c.MAX.APIURL) == "" {
		errs = append(errs, errors.New("max.api_url must not be empty"))
	}

	if _, _, err := c.Campaign.Span(); err != nil {
		errs = append(errs, err)
	}
	if c.Campaign.MaxChances < 1 {
		errs = append(errs, fmt.Errorf("campaign.max_chances must be at least 1, got %d", c.Campaign.MaxChances))
	}

	if _, _, err := c.Draw.Clock(); err != nil {
		errs = append(errs, err)
	}
	if _, err := c.Draw.Location(); err != nil {
		errs = append(errs, err)
	}
	if c.Draw.FinalWinners < 1 {
		errs = append(errs, fmt.Errorf("draw.final_winners must be at least 1, got %d", c.Draw.FinalWinners))
	}
	if c.Draw.TicketsPerWinner < 1 {
		errs = append(errs, fmt.Errorf("draw.tickets_per_winner must be at least 1, got %d", c.Draw.TicketsPerWinner))
	}
	if c.Draw.ReserveCount < 0 {
		errs = append(errs, fmt.Errorf("draw.reserve_count cannot be negative, got %d", c.Draw.ReserveCount))
	}

	return errors.Join(errs...)
}

// Span returns the parsed campaign dates. A zero date means the bound is not
// configured; a configured end must not precede the start.
func (c Campaign) Span() (start, end time.Time, err error) {
	if c.StartDate != "" {
		if start, err = time.Parse(time.DateOnly, c.StartDate); err != nil {
			return time.Time{}, time.Time{}, fmt.Errorf("campaign.start_date must be YYYY-MM-DD: %w", err)
		}
	}
	if c.EndDate != "" {
		if end, err = time.Parse(time.DateOnly, c.EndDate); err != nil {
			return time.Time{}, time.Time{}, fmt.Errorf("campaign.end_date must be YYYY-MM-DD: %w", err)
		}
	}
	if !start.IsZero() && !end.IsZero() && end.Before(start) {
		return time.Time{}, time.Time{}, fmt.Errorf("campaign.end_date %s is before start_date %s", c.EndDate, c.StartDate)
	}
	return start, end, nil
}

// Clock returns the parsed daily draw time of day.
func (d Draw) Clock() (hour, minute int, err error) {
	clock, err := time.Parse("15:04", d.Time)
	if err != nil {
		return 0, 0, fmt.Errorf("draw.time must be HH:MM: %w", err)
	}
	return clock.Hour(), clock.Minute(), nil
}

// Location returns the draw time zone.
func (d Draw) Location() (*time.Location, error) {
	loc, err := time.LoadLocation(d.Timezone)
	if err != nil {
		return nil, fmt.Errorf("draw.timezone must be an IANA time zone: %w", err)
	}
	return loc, nil
}

// Today returns the current calendar day in the draw time zone. All day
// comparisons — partner days, chances, draw dates — use this value.
func (c *Config) Today(now time.Time) (time.Time, error) {
	loc, err := c.Draw.Location()
	if err != nil {
		return time.Time{}, err
	}
	local := now.In(loc)
	return time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, loc), nil
}

// RemainingDays returns how many campaign days are left for the given day,
// including it. An unconfigured end date counts as zero days left.
func (c *Config) RemainingDays(today time.Time) int {
	_, end, err := c.Campaign.Span()
	if err != nil || end.IsZero() {
		return 0
	}
	left := daysBetween(today, end) + 1
	if left < 0 {
		return 0
	}
	return left
}

// daysBetween counts whole calendar days from a to b using noon UTC so that
// daylight saving shifts cannot break the count.
func daysBetween(a, b time.Time) int {
	au := time.Date(a.Year(), a.Month(), a.Day(), 12, 0, 0, 0, time.UTC)
	bu := time.Date(b.Year(), b.Month(), b.Day(), 12, 0, 0, 0, time.UTC)
	return int(bu.Sub(au).Hours() / 24)
}
