package model

import (
	"time"

	"github.com/google/uuid"
)

// Dialog states of the winner contact flow. StateNone means the user is not
// in a dialog and button commands work as usual.
// States of the winner contact dialog.
const (
	StateNone            = ""
	StateAwaitingName    = "awaiting_name"
	StateAwaitingConsent = "awaiting_consent"
)

// User is a bot user identified by their MAX id.
type User struct {
	ID        uuid.UUID
	MaxID     int64
	FirstName string
	LastName  string
	State     UserState
	CreatedAt time.Time
	UpdatedAt time.Time
}

// UserState is the persisted dialog state of a user. ID points to the prize
// being claimed and Name holds the answer of the name step.
type UserState struct {
	State string
	ID    uuid.UUID
	Name  string
}

// DayCount is the number of events on one calendar day, YYYY-MM-DD.
type DayCount struct {
	Day   string `json:"day"`
	Count int64  `json:"count"`
}
