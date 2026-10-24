package handlers

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"

	"github.com/Senpa1k/Giveaway-bot-vk/internal/http-server/apierr"
	"github.com/Senpa1k/Giveaway-bot-vk/internal/lib/logger"
	"github.com/Senpa1k/Giveaway-bot-vk/internal/maxapi"
)

// maxWebhookBodyBytes bounds one MAX webhook update; updates are small.
const maxWebhookBodyBytes = 1 << 20

// callback receives MAX webhook updates (POST /callback). The subscription is
// registered by the administrators through POST /subscriptions of the Bot API;
// the endpoint answers 200 to every accepted update.
func (s *Server) callback(w http.ResponseWriter, r *http.Request) {
	payload, err := io.ReadAll(io.LimitReader(r.Body, maxWebhookBodyBytes))
	if err != nil {
		apierr.Write(w, r, apierr.InvalidRequest("cannot read the update body"))
		return
	}

	var update maxapi.Update
	if err := json.Unmarshal(payload, &update); err != nil {
		apierr.Write(w, r, apierr.InvalidRequest("invalid update JSON"))
		return
	}

	s.processUpdate(r, update)
	writeOK(w)
}

// processUpdate decodes one platform update, acknowledges button presses and
// queues the bot event. The update is answered immediately; the queue keeps
// user message order while the worker does the slow part, like subscription
// checks.
func (s *Server) processUpdate(r *http.Request, update maxapi.Update) {
	message, ok := maxapi.DecodeUpdate(update)
	if !ok {
		return
	}
	if message.CallbackID != "" && s.answerer != nil {
		if err := s.answerer.AnswerCallback(r.Context(), message.CallbackID); err != nil {
			s.log.Warn("answer callback failed", logger.Err(err), slog.Int64("max_id", message.UserID))
		}
	}
	s.services.Enqueue(message)
}

// writeOK confirms the update to the platform.
func writeOK(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(`{"success":true}`))
}
