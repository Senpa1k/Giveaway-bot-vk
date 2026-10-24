// Package vkapi is a small client for the VK API methods used by the bot.
package vkapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const (
	vkAPIBase = "https://api.vk.com/method/"

	// errCodeRateLimit is VK error 6: too many requests per second.
	errCodeRateLimit = 6
	// errCodeCantSend is VK error 901: the bot cannot message the user.
	errCodeCantSend = 901

	maxAttempts        = 3
	rateLimitRetry     = 400 * time.Millisecond
	maxResponseBytes   = 1 << 20
	defaultHTTPTimeout = 30 * time.Second
)

// APIError is an error response of the VK API.
type APIError struct {
	Code        int
	Description string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("vk api error %d: %s", e.Code, e.Description)
}

// IsCantSend reports whether the error means VK refused to deliver the
// message, for example when the user blocked the community.
func IsCantSend(err error) bool {
	var apiErr *APIError
	return errors.As(err, &apiErr) && apiErr.Code == errCodeCantSend
}

// randomCounter makes random_id unique across process restarts and calls.
var randomCounter atomic.Int64

// Client calls the VK API and paces outgoing messages.
type Client struct {
	token      string
	apiVersion string
	http       *http.Client
	sendMu     sync.Mutex
	nextSendAt time.Time
	sendGap    time.Duration
}

// New creates a VK API client. sendInterval paces messages.send calls to stay
// under the community message rate limit; zero disables pacing.
func New(token, apiVersion string, sendInterval time.Duration) *Client {
	return &Client{
		token:      token,
		apiVersion: apiVersion,
		http:       &http.Client{Timeout: defaultHTTPTimeout},
		sendGap:    sendInterval,
	}
}

// MessagesSend sends a private message to the user with an optional keyboard
// and attachment.
func (c *Client) MessagesSend(ctx context.Context, peerID int64, text string, keyboard *Keyboard, attachment string) error {
	params := url.Values{}
	params.Set("peer_id", strconv.FormatInt(peerID, 10))
	params.Set("random_id", nextRandomID())
	params.Set("message", text)
	if keyboard != nil {
		encoded, err := json.Marshal(keyboard)
		if err != nil {
			return fmt.Errorf("encode keyboard: %w", err)
		}
		params.Set("keyboard", string(encoded))
	}
	if attachment != "" {
		params.Set("attachment", attachment)
	}

	if err := c.pace(ctx); err != nil {
		return err
	}
	return c.call(ctx, "messages.send", params, nil)
}

// IsMember reports whether the user is subscribed to the community.
func (c *Client) IsMember(ctx context.Context, groupID, userID int64) (bool, error) {
	params := url.Values{}
	params.Set("group_id", strconv.FormatInt(groupID, 10))
	params.Set("user_id", strconv.FormatInt(userID, 10))

	var member json.Number
	if err := c.call(ctx, "groups.isMember", params, &member); err != nil {
		return false, err
	}
	return member.String() == "1", nil
}

// getServer returns the long poll server of the community.
func (c *Client) getServer(ctx context.Context, groupID int64) (server, key, ts string, err error) {
	params := url.Values{}
	params.Set("group_id", strconv.FormatInt(groupID, 10))

	var resp struct {
		Server string `json:"server"`
		Key    string `json:"key"`
		TS     string `json:"ts"`
	}
	if err := c.call(ctx, "groups.getLongPollServer", params, &resp); err != nil {
		return "", "", "", fmt.Errorf("get long poll server: %w", err)
	}
	return resp.Server, resp.Key, resp.TS, nil
}

// call performs one VK API method call, retrying the rate limit error.
func (c *Client) call(ctx context.Context, method string, params url.Values, dst any) error {
	params.Set("access_token", c.token)
	params.Set("v", c.apiVersion)
	body := params.Encode()

	var lastErr error
	for attempt := range maxAttempts {
		if attempt > 0 {
			if err := sleep(ctx, rateLimitRetry); err != nil {
				return err
			}
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodPost, vkAPIBase+method, strings.NewReader(body))
		if err != nil {
			return fmt.Errorf("build %s request: %w", method, err)
		}
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

		resp, err := c.http.Do(req)
		if err != nil {
			return fmt.Errorf("call %s: %w", method, err)
		}
		payload, readErr := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
		closeErr := resp.Body.Close()
		if readErr != nil {
			return fmt.Errorf("read %s response: %w", method, readErr)
		}
		if closeErr != nil {
			return fmt.Errorf("close %s response: %w", method, closeErr)
		}

		var envelope struct {
			Response json.RawMessage `json:"response"`
			Error    *struct {
				Code        int    `json:"error_code"`
				Description string `json:"error_msg"`
			} `json:"error"`
		}
		if err := json.Unmarshal(payload, &envelope); err != nil {
			return fmt.Errorf("decode %s response: %w", method, err)
		}
		if envelope.Error != nil {
			apiErr := &APIError{Code: envelope.Error.Code, Description: envelope.Error.Description}
			if apiErr.Code == errCodeRateLimit && attempt < maxAttempts-1 {
				lastErr = apiErr
				continue
			}
			return apiErr
		}
		if dst != nil && len(envelope.Response) > 0 {
			if err := json.Unmarshal(envelope.Response, dst); err != nil {
				return fmt.Errorf("decode %s payload: %w", method, err)
			}
		}
		return nil
	}
	return lastErr
}

// pace holds the message rate: at most one send per sendInterval.
func (c *Client) pace(ctx context.Context) error {
	if c.sendGap <= 0 {
		return nil
	}

	c.sendMu.Lock()
	now := time.Now()
	if wait := c.nextSendAt.Sub(now); wait > 0 {
		if err := sleep(ctx, wait); err != nil {
			c.sendMu.Unlock()
			return err
		}
		c.nextSendAt = time.Now().Add(c.sendGap)
	} else {
		c.nextSendAt = now.Add(c.sendGap)
	}
	c.sendMu.Unlock()
	return nil
}

// sleep waits for d or until the context ends.
func sleep(ctx context.Context, d time.Duration) error {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

// nextRandomID builds a unique deduplication id for messages.send.
func nextRandomID() string {
	return strconv.FormatInt(time.Now().UnixNano()+randomCounter.Add(1), 10)
}
