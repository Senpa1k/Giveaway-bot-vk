package model

import (
	"time"

	"github.com/google/uuid"
)

// DrawKind separates daily partner draws from the final draw.
type DrawKind string

const (
	DrawKindDaily DrawKind = "daily"
	DrawKindFinal DrawKind = "final"
)

// DrawStatus is the lifecycle of a draw: pending before the raffle, drawn
// after winners are picked, approved after an admin confirms the results and
// notified once winners received their messages.
type DrawStatus string

const (
	DrawStatusPending  DrawStatus = "pending"
	DrawStatusDrawn    DrawStatus = "drawn"
	DrawStatusApproved DrawStatus = "approved"
	DrawStatusNotified DrawStatus = "notified"
)

// Draw is one raffle: a partner day or the final.
type Draw struct {
	ID           uuid.UUID
	Kind         DrawKind
	PartnerID    *uuid.UUID
	DrawDate     time.Time
	Status       DrawStatus
	WinnersCount int
	ReserveCount int
	DrawnAt      *time.Time
	ApprovedAt   *time.Time
	ApprovedBy   string
	NotifiedAt   *time.Time
	CreatedAt    time.Time
}

// DrawEntry is a registered participant of a draw.
type DrawEntry struct {
	ID        uuid.UUID
	DrawID    uuid.UUID
	UserID    uuid.UUID
	CreatedAt time.Time
}

// WinnerKind separates prize winners from reserve winners.
type WinnerKind string

const (
	WinnerKindWinner  WinnerKind = "winner"
	WinnerKindReserve WinnerKind = "reserve"
)

// WinnerStatus tracks prize delivery: awaiting_response until the winner
// answers, contacted after the claim dialog starts, confirmed once the
// contact data is saved. Missed and reassigned close a prize without issue.
type WinnerStatus string

const (
	WinnerStatusAwaitingResponse WinnerStatus = "awaiting_response"
	WinnerStatusContacted        WinnerStatus = "contacted"
	WinnerStatusConfirmed        WinnerStatus = "confirmed"
	WinnerStatusMissed           WinnerStatus = "missed"
	WinnerStatusReassigned       WinnerStatus = "reassigned"
)

// Winner is a picked winner of a draw.
type Winner struct {
	ID           uuid.UUID
	DrawID       uuid.UUID
	UserID       uuid.UUID
	Position     int
	Kind         WinnerKind
	Prize        string
	Tickets      int
	Status       WinnerStatus
	ContactName  string
	ConsentGiven bool
	ConsentAt    *time.Time
	NotifiedAt   *time.Time
	CreatedAt    time.Time
}

// DrawResult is the outcome of a raffle for reports and admin review.
type DrawResult struct {
	Draw         Draw       `json:"draw"`
	Participants int        `json:"participants"`
	Winners      []Winner   `json:"winners"`
}
