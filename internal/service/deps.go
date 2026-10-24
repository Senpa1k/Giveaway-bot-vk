package service

import (
	"context"

	"github.com/Senpa1k/Giveaway-bot-vk/internal/maxapi"
)

// Sender delivers bot messages to MAX users.
type Sender interface {
	SendMessage(ctx context.Context, userID int64, text string, keyboard *maxapi.Keyboard) error
}

// Subscriber checks MAX channel subscriptions.
type Subscriber interface {
	IsMember(ctx context.Context, chatID, userID int64) (bool, error)
}

// Reply is a bot message waiting to be sent. A nil Keyboard sends a plain
// message.
type Reply struct {
	Text     string
	Keyboard *maxapi.Keyboard
}
