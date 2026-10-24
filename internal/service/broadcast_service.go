package service

import (
	"context"
	"log/slog"

	"github.com/Senpa1k/Giveaway-bot-vk/internal/lib/logger"
	"github.com/Senpa1k/Giveaway-bot-vk/internal/vkapi"
)

// BroadcastUsers lists the recipients of broadcasts.
type BroadcastUsers interface {
	AllVKIDs(ctx context.Context) ([]int64, error)
}

// BroadcastService pushes a text message to every bot user. The client paces
// the sends, so a broadcast runs as one long request.
type BroadcastService struct {
	users  BroadcastUsers
	sender Sender
	log    *slog.Logger
}

// NewBroadcastService constructs the broadcast service.
func NewBroadcastService(users BroadcastUsers, sender Sender, log *slog.Logger) *BroadcastService {
	return &BroadcastService{users: users, sender: sender, log: log}
}

// Send delivers the text to all users and returns the delivery counts.
// Undeliverable users are counted as failed and do not stop the run.
func (s *BroadcastService) Send(ctx context.Context, text string) (sent, failed int, err error) {
	vkIDs, err := s.users.AllVKIDs(ctx)
	if err != nil {
		return 0, 0, err
	}

	for _, vkID := range vkIDs {
		if err := s.sender.MessagesSend(ctx, vkID, text, nil, ""); err != nil {
			if vkapi.IsCantSend(err) {
				failed++
				continue
			}
			s.log.Error("broadcast send failed", logger.Err(err), slog.Int64("vk_id", vkID))
			failed++
			continue
		}
		sent++
	}
	return sent, failed, nil
}
