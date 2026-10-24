package model

import (
	"time"

	"github.com/google/uuid"
)

// Sources of a day chance: a received promocode or a draw registration.
const (
	ChanceSourcePromo = "promo"
	ChanceSourceDraw  = "draw"
)

// Chance is one active day of a user; each chance is a ticket in the final
// draw. One user can have at most one chance per day.
type Chance struct {
	ID        uuid.UUID
	UserID    uuid.UUID
	Day       time.Time
	Source    string
	CreatedAt time.Time
}
