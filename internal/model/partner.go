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
	VKGroupID        int64
	VKURL            string
	PromoValidUntil  *time.Time
	PromoWhere       string
	PrizeName        string
	PrizeInstruction string
	WinnersCount     int
	CreatedAt        time.Time
}
