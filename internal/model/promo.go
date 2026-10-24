package model

import (
	"time"

	"github.com/google/uuid"
)

// Promocode is a partner code; UserID is nil until the code is issued.
type Promocode struct {
	ID         uuid.UUID
	PartnerID  uuid.UUID
	Code       string
	UserID     *uuid.UUID
	AssignedAt *time.Time
	CreatedAt  time.Time
}
