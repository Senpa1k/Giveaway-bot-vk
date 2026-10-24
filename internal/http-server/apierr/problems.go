// Package apierr builds RFC 9457 HTTP errors (application/problem+json).
package apierr

import (
	"encoding/json"
	"net/http"
)

const typeBase = "https://giveaway-bot-vk.example/problems/"

// Problem is the error response body of the admin API.
type Problem struct {
	Type     string  `json:"type"`
	Title    string  `json:"title"`
	Status   int32   `json:"status"`
	Code     string  `json:"code"`
	Detail   *string `json:"detail,omitempty"`
	Instance *string `json:"instance,omitempty"`
}

// InvalidRequest reports an invalid HTTP request.
func InvalidRequest(detail string) Problem {
	return newProblem(http.StatusBadRequest, "invalid_request", "invalid-request", "Invalid request", detail)
}

// Unauthorized reports a missing or wrong bearer token.
func Unauthorized(detail string) Problem {
	return newProblem(http.StatusUnauthorized, "unauthorized", "unauthorized", "Unauthorized", detail)
}

// NotFound reports a missing API resource.
func NotFound(detail string) Problem {
	return newProblem(http.StatusNotFound, "not_found", "not-found", "Not found", detail)
}

// Conflict reports a conflicting request, for example a draw that already
// ran or a status transition that is not allowed.
func Conflict(detail string) Problem {
	return newProblem(http.StatusConflict, "conflict", "conflict", "Conflict", detail)
}

// MethodNotAllowed reports an unsupported method for a route.
func MethodNotAllowed(detail string) Problem {
	return newProblem(http.StatusMethodNotAllowed, "method_not_allowed", "method-not-allowed", "Method not allowed", detail)
}

// Internal reports an unexpected error without exposing its details.
func Internal() Problem {
	return newProblem(http.StatusInternalServerError, "internal_error", "internal-error", "Internal server error", "")
}

// Write sends a Problem response and sets instance to the request path.
func Write(w http.ResponseWriter, r *http.Request, p Problem) {
	instance := r.URL.Path
	p.Instance = &instance

	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(int(p.Status))
	_ = json.NewEncoder(w).Encode(p)
}

func newProblem(status int32, code, slug, title, detail string) Problem {
	p := Problem{
		Type:   typeBase + slug,
		Title:  title,
		Status: status,
		Code:   code,
	}
	if detail != "" {
		p.Detail = &detail
	}
	return p
}
