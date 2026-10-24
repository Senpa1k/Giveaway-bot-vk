// Package model contains domain entities and errors shared between layers.
package model

import "errors"

var (
	// ErrUserNotFound is returned by storage when no user matches the query.
	ErrUserNotFound = errors.New("user not found")
	// ErrPartnerNotFound is returned when no partner matches the day or ID.
	ErrPartnerNotFound = errors.New("partner not found")
	// ErrPromocodeNotFound is returned when the user has no code from the partner.
	ErrPromocodeNotFound = errors.New("promocode not found")
	// ErrPromocodesDepleted is returned when the partner has no free codes left.
	ErrPromocodesDepleted = errors.New("promocodes depleted")
	// ErrAlreadyRegistered is returned when the user is already in the day draw.
	ErrAlreadyRegistered = errors.New("already registered in the draw")
	// ErrDrawNotFound is returned when no draw matches the query.
	ErrDrawNotFound = errors.New("draw not found")
	// ErrInvalidDrawStatus marks a draw status transition that is not allowed.
	ErrInvalidDrawStatus = errors.New("invalid draw status transition")
	// ErrWinnerNotFound is returned when no winner matches the query.
	ErrWinnerNotFound = errors.New("winner not found")
	// ErrInvalidWinnerStatus marks a winner status transition that is not allowed.
	ErrInvalidWinnerStatus = errors.New("invalid winner status transition")
	// ErrNoEntries is returned when a draw cannot run without participants.
	ErrNoEntries = errors.New("draw has no entries")
)
