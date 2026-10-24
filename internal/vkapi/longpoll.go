package vkapi

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/Senpa1k/Giveaway-bot-vk/internal/lib/logger"
)

// longPollFetchOverhead is added to the long poll wait to bound the HTTP
// request timeout.
const longPollFetchOverhead = 15 * time.Second

// LongPoll consumes events through VK Bots Long Poll. It exists so the bot
// can run without the public HTTPS address required by the Callback API,
// which is handy for local development.
type LongPoll struct {
	client  *Client
	groupID int64
	wait    time.Duration
	log     *slog.Logger
}

// NewLongPoll creates a long poll consumer of the community events.
func NewLongPoll(client *Client, groupID int64, wait time.Duration, log *slog.Logger) *LongPoll {
	return &LongPoll{client: client, groupID: groupID, wait: wait, log: log}
}

// Run blocks fetching updates and passing them to handle until the context
// ends. It reconnects on VK long poll failures.
func (lp *LongPoll) Run(ctx context.Context, handle func(context.Context, Event)) error {
	server, key, ts, err := lp.client.getServer(ctx, lp.groupID)
	if err != nil {
		return err
	}
	lp.log.Info("long poll started", slog.String("server", server), slog.String("ts", ts))

	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		response, err := lp.fetch(ctx, server, key, ts)
		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			lp.log.Error("long poll fetch failed, retrying", logger.Err(err))
			if sleepErr := sleep(ctx, 3*time.Second); sleepErr != nil {
				return sleepErr
			}
			continue
		}

		// failed=1: the ts is stale, the response carries the next one.
		// failed=2 or 3: the connection key is lost, request a new server.
		if response.Failed == 2 || response.Failed == 3 {
			server, key, ts, err = lp.client.getServer(ctx, lp.groupID)
			if err != nil {
				if ctx.Err() != nil {
					return ctx.Err()
				}
				lp.log.Error("long poll reconnect failed", logger.Err(err))
				if sleepErr := sleep(ctx, 3*time.Second); sleepErr != nil {
					return sleepErr
				}
			}
			continue
		}
		if response.TS != "" {
			ts = response.TS
		}
		for _, update := range response.Updates {
			handle(ctx, update)
		}
	}
}

// fetch performs one long poll request.
func (lp *LongPoll) fetch(ctx context.Context, server, key, ts string) (longPollResponse, error) {
	query := url.Values{}
	query.Set("act", "a_check")
	query.Set("key", key)
	query.Set("ts", ts)
	query.Set("wait", strconv.Itoa(int(lp.wait.Seconds())))
	query.Set("mode", "2")

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, server+"?"+query.Encode(), nil)
	if err != nil {
		return longPollResponse{}, fmt.Errorf("build long poll request: %w", err)
	}

	httpClient := &http.Client{Timeout: lp.wait + longPollFetchOverhead}
	resp, err := httpClient.Do(req)
	if err != nil {
		return longPollResponse{}, fmt.Errorf("long poll request: %w", err)
	}
	payload, readErr := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
	closeErr := resp.Body.Close()
	if readErr != nil {
		return longPollResponse{}, fmt.Errorf("read long poll response: %w", readErr)
	}
	if closeErr != nil {
		return longPollResponse{}, fmt.Errorf("close long poll response: %w", closeErr)
	}

	var response longPollResponse
	if err := json.Unmarshal(payload, &response); err != nil {
		return longPollResponse{}, fmt.Errorf("decode long poll response: %w", err)
	}
	return response, nil
}

type longPollResponse struct {
	TS       string  `json:"ts"`
	Failed   int     `json:"failed"`
	Updates  []Event `json:"updates"`
}
