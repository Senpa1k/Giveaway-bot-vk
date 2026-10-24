package handlers

import (
	"net/http"

	"github.com/Senpa1k/Giveaway-bot-vk/internal/http-server/apierr"
	"github.com/Senpa1k/Giveaway-bot-vk/internal/lib/logger"
)

// health reports that the process answers requests (GET /health).
func (s *Server) health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, r, http.StatusOK, map[string]string{"status": "ok"})
}

// readiness reports whether PostgreSQL answers (GET /ready).
func (s *Server) readiness(w http.ResponseWriter, r *http.Request) {
	if err := s.services.Ping(r.Context()); err != nil {
		s.log.ErrorContext(r.Context(), "readiness check failed", logger.Err(err))
		apierr.Write(w, r, apierr.Internal())
		return
	}
	writeJSON(w, r, http.StatusOK, map[string]string{"status": "ready"})
}

// notFound answers unknown routes with a problem.
func (s *Server) notFound(w http.ResponseWriter, r *http.Request) {
	apierr.Write(w, r, apierr.NotFound("route not found"))
}

// methodNotAllowed answers a wrong method with a problem.
func (s *Server) methodNotAllowed(w http.ResponseWriter, r *http.Request) {
	apierr.Write(w, r, apierr.MethodNotAllowed("method not allowed for this route"))
}
