package maxapi

import (
	"context"
	"log/slog"
	"time"

	"github.com/Senpa1k/Giveaway-bot-vk/internal/lib/logger"
)

// longPollFetchOverhead is added to the long poll wait to bound the HTTP
// request timeout.
const longPollFetchOverhead = 15 * time.Second

// pollErrorPause separates long poll retries so a broken connection does not
// spin the loop.
const pollErrorPause = 3 * time.Second

// longPollTypes is the update filter of the long poll subscription.
const longPollTypes = UpdateBotStarted + "," + UpdateMessageCreated + "," + UpdateMessageCallback

// LongPoll consumes updates through GET /updates. The platform positions
// long polling as a development tool; webhooks serve production traffic, so
// the bot can run locally without a public HTTPS address.
type LongPoll struct {
	client *Client
	wait   time.Duration
	log    *slog.Logger
}

// NewLongPoll creates a long poll consumer of the bot updates.
func NewLongPoll(client *Client, wait time.Duration, log *slog.Logger) *LongPoll {
	return &LongPoll{client: client, wait: wait, log: log}
}

// Run blocks fetching updates and passing them to handle until the context
// ends. It retries fetch errors with a pause.
func (lp *LongPoll) Run(ctx context.Context, handle func(context.Context, Update)) error {
	lp.log.Info("long poll started", slog.Duration("wait", lp.wait))

	var marker *int64
	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		updates, next, err := lp.client.GetUpdates(ctx, marker, longPollTypes, lp.wait)
		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			lp.log.Error("long poll fetch failed, retrying", logger.Err(err))
			if sleepErr := sleep(ctx, pollErrorPause); sleepErr != nil {
				return sleepErr
			}
			continue
		}

		marker = next
		for _, update := range updates {
			handle(ctx, update)
		}
	}
}
