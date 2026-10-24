// Package maxapi is a small client for the MAX Bot API methods used by the
// bot. Requests go to platform-api2.max.ru with the access token in the
// Authorization header.
package maxapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"sync"
	"time"
)

const (
	// DefaultAPIURL is the production MAX Bot API endpoint.
	DefaultAPIURL = "https://platform-api2.max.ru"

	maxAttempts        = 3
	rateLimitRetry     = 500 * time.Millisecond
	maxResponseBytes   = 4 << 20
	defaultHTTPTimeout = 30 * time.Second
)

// APIError is a non-2xx response of the MAX Bot API. Code carries the HTTP
// status, so callers can react to rate limits and forbidden sends.
type APIError struct {
	Code        int
	Description string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("max api error %d: %s", e.Code, e.Description)
}

// IsCantSend reports whether the error means MAX refused to deliver the
// message, for example when the user blocked the bot.
func IsCantSend(err error) bool {
	var apiErr *APIError
	return errors.As(err, &apiErr) && apiErr.Code == http.StatusForbidden
}

// Client calls the MAX Bot API and paces outgoing messages. The platform
// allows at most two messages per second per dialog.
type Client struct {
	baseURL    string
	token      string
	http       *http.Client
	log        *slog.Logger
	sendMu     sync.Mutex
	nextSendAt time.Time
	sendGap    time.Duration
}

// New creates a MAX Bot API client. sendInterval paces POST /messages calls
// to stay under the platform message rate limit; zero disables pacing.
func New(baseURL, token string, sendInterval time.Duration, log *slog.Logger) *Client {
	return &Client{
		baseURL: baseURL,
		token:   token,
		http:    &http.Client{Timeout: defaultHTTPTimeout},
		log:     log,
		sendGap: sendInterval,
	}
}

// newMessageBody is the request body of POST /messages.
type newMessageBody struct {
	Text        string       `json:"text,omitempty"`
	Attachments []attachment `json:"attachments,omitempty"`
	Notify      bool         `json:"notify"`
}

// attachment is one attachment of a message. Keyboards travel as the
// inline_keyboard attachment.
type attachment struct {
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload,omitempty"`
}

// SendMessage sends a private message to the user with an optional inline
// keyboard.
func (c *Client) SendMessage(ctx context.Context, userID int64, text string, keyboard *Keyboard) error {
	body := newMessageBody{Text: text, Notify: true}
	if keyboard != nil {
		payload, err := json.Marshal(keyboard)
		if err != nil {
			return fmt.Errorf("encode keyboard: %w", err)
		}
		body.Attachments = []attachment{{Type: "inline_keyboard", Payload: payload}}
	}

	query := url.Values{}
	query.Set("user_id", strconv.FormatInt(userID, 10))

	if err := c.pace(ctx); err != nil {
		return err
	}
	return c.call(ctx, http.MethodPost, "/messages", query, body, nil, defaultHTTPTimeout)
}

// AnswerCallback acknowledges a button press, stopping the spinner on the
// button in the user's client.
func (c *Client) AnswerCallback(ctx context.Context, callbackID string) error {
	body := map[string]string{"callback_id": callbackID}
	return c.call(ctx, http.MethodPost, "/answers", nil, body, nil, defaultHTTPTimeout)
}

// IsMember reports whether the user is a member of the chat or a subscriber
// of the channel. The bot must be an administrator there.
func (c *Client) IsMember(ctx context.Context, chatID, userID int64) (bool, error) {
	path := fmt.Sprintf("/chats/%d/members", chatID)
	query := url.Values{}
	query.Set("user_ids", strconv.FormatInt(userID, 10))

	var response struct {
		Members []struct {
			UserID int64 `json:"user_id"`
		} `json:"members"`
	}
	if err := c.call(ctx, http.MethodGet, path, query, nil, &response, defaultHTTPTimeout); err != nil {
		return false, err
	}
	for _, member := range response.Members {
		if member.UserID == userID {
			return true, nil
		}
	}
	return false, nil
}

// GetUpdates performs one long poll request and returns the updates with the
// marker of the next batch. A nil marker starts from the oldest kept events.
func (c *Client) GetUpdates(ctx context.Context, marker *int64, types string, wait time.Duration) ([]Update, *int64, error) {
	query := url.Values{}
	query.Set("timeout", strconv.Itoa(int(wait.Seconds())))
	query.Set("limit", "100")
	if types != "" {
		query.Set("types", types)
	}
	if marker != nil {
		query.Set("marker", strconv.FormatInt(*marker, 10))
	}

	var response struct {
		Updates []Update `json:"updates"`
		Marker  *int64   `json:"marker"`
	}
	timeout := wait + longPollFetchOverhead
	if err := c.call(ctx, http.MethodGet, "/updates", query, nil, &response, timeout); err != nil {
		return nil, nil, err
	}
	return response.Updates, response.Marker, nil
}

// call performs one HTTP request to the Bot API, retrying the rate limit
// status. The token travels in the Authorization header, as the platform
// requires.
func (c *Client) call(
	ctx context.Context,
	method, path string,
	query url.Values,
	body, dst any,
	timeout time.Duration,
) error {
	var payload []byte
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("encode %s body: %w", path, err)
		}
		payload = encoded
	}

	var lastErr error
	for attempt := range maxAttempts {
		if attempt > 0 {
			if err := sleep(ctx, rateLimitRetry); err != nil {
				return err
			}
		}

		err := c.request(ctx, method, path, query, payload, dst, timeout)
		if err == nil {
			return nil
		}
		var apiErr *APIError
		if errors.As(err, &apiErr) && apiErr.Code == http.StatusTooManyRequests && attempt < maxAttempts-1 {
			lastErr = err
			continue
		}
		return err
	}
	return lastErr
}

// request builds and runs one HTTP exchange and decodes the response.
func (c *Client) request(
	ctx context.Context,
	method, path string,
	query url.Values,
	payload []byte,
	dst any,
	timeout time.Duration,
) error {
	requestCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	target := c.baseURL + path
	if len(query) > 0 {
		target += "?" + query.Encode()
	}

	var reader io.Reader
	if payload != nil {
		reader = bytes.NewReader(payload)
	}
	req, err := http.NewRequestWithContext(requestCtx, method, target, reader)
	if err != nil {
		return fmt.Errorf("build %s request: %w", path, err)
	}
	req.Header.Set("Authorization", c.token)
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("call %s: %w", path, err)
	}
	responseBody, readErr := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
	closeErr := resp.Body.Close()
	if readErr != nil {
		return fmt.Errorf("read %s response: %w", path, readErr)
	}
	if closeErr != nil {
		return fmt.Errorf("close %s response: %w", path, closeErr)
	}

	if resp.StatusCode >= http.StatusBadRequest {
		var failure struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		}
		_ = json.Unmarshal(responseBody, &failure)
		description := failure.Message
		if description == "" {
			description = string(responseBody)
		}
		return &APIError{Code: resp.StatusCode, Description: description}
	}
	if dst != nil && len(responseBody) > 0 {
		if err := json.Unmarshal(responseBody, dst); err != nil {
			return fmt.Errorf("decode %s payload: %w", path, err)
		}
	}
	return nil
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
