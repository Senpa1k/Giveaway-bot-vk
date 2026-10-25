package handlers

import (
	"crypto/subtle"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"

	"github.com/Senpa1k/Giveaway-bot-vk/internal/http-server/apierr"
	"github.com/Senpa1k/Giveaway-bot-vk/internal/lib/logger"
	"github.com/Senpa1k/Giveaway-bot-vk/internal/vkapi"
)

// maxCallbackBodyBytes bounds one Callback API event; VK events are small.
const maxCallbackBodyBytes = 1 << 20

// callback receives VK Callback API events (POST /callback). The response is
// the confirmation string once and plain "ok" afterwards, as VK requires.
func (s *Server) callback(w http.ResponseWriter, r *http.Request) {
	payload, err := io.ReadAll(io.LimitReader(r.Body, maxCallbackBodyBytes))
	if err != nil {
		apierr.Write(w, r, apierr.InvalidRequest("cannot read the event body"))
		return
	}

	var event vkapi.Event
	if err := json.Unmarshal(payload, &event); err != nil {
		apierr.Write(w, r, apierr.InvalidRequest("invalid event JSON"))
		return
	}

	// The confirmation string is not a secret - VK shows it in the community
	// settings - and the first confirmation arrives before the secret is
	// saved in the community. Confirm first, then verify real events.
	if event.Type == vkapi.EventConfirmation {
		writePlain(w, s.confirmation)
		return
	}

	if s.secret != "" && subtle.ConstantTimeCompare([]byte(event.Secret), []byte(s.secret)) != 1 {
		s.log.Warn("callback secret mismatch", slog.Int64("group_id", event.GroupID))
		apierr.Write(w, r, apierr.Unauthorized("bad callback secret"))
		return
	}

	switch event.Type {
	case vkapi.EventMessageNew:
		message, err := event.Message()
		if err != nil {
			s.log.Error("decode message_new failed", logger.Err(err))
			writePlain(w, "ok")
			return
		}
		// The event is answered immediately; the queue keeps user message
		// order while the worker does the slow part, like VK checks.
		s.services.Enqueue(message)
		writePlain(w, "ok")
	default:
		writePlain(w, "ok")
	}
}

// writePlain answers with a text/plain body, the format VK expects.
func writePlain(w http.ResponseWriter, body string) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = w.Write([]byte(body))
}
