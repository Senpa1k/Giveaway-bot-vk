package service

import (
	"context"

	"github.com/Senpa1k/Giveaway-bot-vk/internal/vkapi"
)

// Sender delivers bot messages to VK users.
type Sender interface {
	MessagesSend(ctx context.Context, peerID int64, text string, keyboard *vkapi.Keyboard, attachment string) error
}

// Subscriber checks VK community subscriptions.
type Subscriber interface {
	IsMember(ctx context.Context, groupID, userID int64) (bool, error)
}

// Reply is a bot message waiting to be sent. A nil Keyboard sends a plain
// message; an empty Attachment sends no banner.
type Reply struct {
	Text       string
	Keyboard   *vkapi.Keyboard
	Attachment string
}
