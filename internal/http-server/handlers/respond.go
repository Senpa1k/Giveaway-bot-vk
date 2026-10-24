package handlers

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/Senpa1k/Giveaway-bot-vk/internal/http-server/apierr"
)

// maxAdminBodyBytes bounds one admin API request.
const maxAdminBodyBytes = 1 << 20

// writeJSON sends one JSON response.
func writeJSON(w http.ResponseWriter, _ *http.Request, status int, body any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(body); err != nil {
		// The status code is already sent; the error is only logged by the
		// enclosing handler through the server error channel.
		_ = err
	}
}

// decodeJSON reads a single JSON object into dst and writes 400 when it
// cannot. Unknown fields are rejected to catch client typos early.
func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) bool {
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxAdminBodyBytes))
	dec.DisallowUnknownFields()
	err := dec.Decode(dst)
	if err == nil && !errors.Is(dec.Decode(&struct{}{}), io.EOF) {
		err = errors.New("request body must contain a single JSON object")
	}
	if err != nil {
		apierr.Write(w, r, apierr.InvalidRequest("invalid JSON body"))
		return false
	}
	return true
}
