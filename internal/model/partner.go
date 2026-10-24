package model

import (
	"time"

	"github.com/google/uuid"
)

// Partner is the partner of one action day.
type Partner struct {
	ID               uuid.UUID
	Name             string
	Day              time.Time
	MaxChannelID     int64
	MaxURL           string
	PromoValidUntil  *time.Time
	PromoWhere       string
	PrizeName        string
	PrizeInstruction string
	WinnersCount     int
	CreatedAt        time.Time
}
