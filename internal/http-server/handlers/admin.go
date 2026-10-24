package handlers

import (
	"crypto/subtle"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/Senpa1k/Giveaway-bot-vk/internal/http-server/apierr"
	"github.com/Senpa1k/Giveaway-bot-vk/internal/lib/logger"
	"github.com/Senpa1k/Giveaway-bot-vk/internal/model"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// Limits of the admin API payloads.
const (
	maxPartnerNameLen   = 100
	maxPrizeTextLen     = 300
	maxInstructionLen   = 1000
	maxPromoBatchSize   = 10000
	maxBroadcastTextLen = 4000
)

// adminAuth guards the admin API with the configured bearer token.
func (s *Server) adminAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token, ok := bearerToken(r)
		if !ok || subtle.ConstantTimeCompare([]byte(token), []byte(s.adminToken)) != 1 {
			apierr.Write(w, r, apierr.Unauthorized("missing or wrong admin token"))
			return
		}
		next.ServeHTTP(w, r)
	})
}

// bearerToken extracts the token from "Authorization: Bearer <token>".
func bearerToken(r *http.Request) (string, bool) {
	scheme, token, found := strings.Cut(r.Header.Get("Authorization"), " ")
	if !found || !strings.EqualFold(scheme, "Bearer") {
		return "", false
	}
	token = strings.TrimSpace(token)
	return token, token != ""
}

type createPartnerRequest struct {
	Name             string `json:"name"`
	Day              string `json:"day"`
	MaxChannelID     int64  `json:"max_channel_id"`
	MaxURL           string `json:"max_url"`
	PromoValidUntil  string `json:"promo_valid_until"`
	PromoWhere       string `json:"promo_where"`
	PrizeName        string `json:"prize_name"`
	PrizeInstruction string `json:"prize_instruction"`
	WinnersCount     int    `json:"winners_count"`
}

type partnerResponse struct {
	ID               uuid.UUID `json:"id"`
	Name             string    `json:"name"`
	Day              string    `json:"day"`
	MaxChannelID     int64     `json:"max_channel_id"`
	MaxURL           string    `json:"max_url"`
	PromoValidUntil  *string   `json:"promo_valid_until"`
	PromoWhere       string    `json:"promo_where"`
	PrizeName        string    `json:"prize_name"`
	PrizeInstruction string    `json:"prize_instruction"`
	WinnersCount     int       `json:"winners_count"`
	CreatedAt        time.Time `json:"created_at"`
}

// createPartner registers the partner of one action day (POST /api/admin/partners).
func (s *Server) createPartner(w http.ResponseWriter, r *http.Request) {
	var req createPartnerRequest
	if !decodeJSON(w, r, &req) {
		return
	}

	partner, detail := newPartner(req)
	if detail != "" {
		apierr.Write(w, r, apierr.InvalidRequest(detail))
		return
	}

	if err := s.services.Create(r.Context(), partner); err != nil {
		s.log.ErrorContext(r.Context(), "create partner failed", logger.Err(err))
		apierr.Write(w, r, apierr.Internal())
		return
	}

	writeJSON(w, r, http.StatusCreated, toPartnerResponse(partner))
}

// listPartners returns all partners ordered by day (GET /api/admin/partners).
func (s *Server) listPartners(w http.ResponseWriter, r *http.Request) {
	partners, err := s.services.ListPartners(r.Context())
	if err != nil {
		s.log.ErrorContext(r.Context(), "list partners failed", logger.Err(err))
		apierr.Write(w, r, apierr.Internal())
		return
	}

	response := make([]partnerResponse, 0, len(partners))
	for i := range partners {
		response = append(response, toPartnerResponse(&partners[i]))
	}
	writeJSON(w, r, http.StatusOK, response)
}

// newPartner validates the request and builds the partner; a non-empty
// detail means the request is invalid.
func newPartner(req createPartnerRequest) (*model.Partner, string) {
	switch {
	case strings.TrimSpace(req.Name) == "":
		return nil, "name is required"
	case utf8.RuneCountInString(req.Name) > maxPartnerNameLen:
		return nil, "name is too long"
	case req.Day == "":
		return nil, "day is required (YYYY-MM-DD)"
	case req.MaxChannelID <= 0:
		return nil, "max_channel_id must be a positive community id"
	case strings.TrimSpace(req.MaxURL) == "":
		return nil, "max_url is required"
	case strings.TrimSpace(req.PrizeName) == "":
		return nil, "prize_name is required"
	case utf8.RuneCountInString(req.PrizeName) > maxPrizeTextLen:
		return nil, "prize_name is too long"
	case utf8.RuneCountInString(req.PrizeInstruction) > maxInstructionLen:
		return nil, "prize_instruction is too long"
	}

	day, err := time.Parse(time.DateOnly, req.Day)
	if err != nil {
		return nil, "day must be YYYY-MM-DD"
	}

	partner := &model.Partner{
		Name:             strings.TrimSpace(req.Name),
		Day:              day,
		MaxChannelID:     req.MaxChannelID,
		MaxURL:           strings.TrimSpace(req.MaxURL),
		PromoWhere:       strings.TrimSpace(req.PromoWhere),
		PrizeName:        strings.TrimSpace(req.PrizeName),
		PrizeInstruction: strings.TrimSpace(req.PrizeInstruction),
		WinnersCount:     req.WinnersCount,
	}
	if partner.WinnersCount == 0 {
		partner.WinnersCount = 1
	}
	if partner.WinnersCount < 0 {
		return nil, "winners_count must be positive"
	}

	if req.PromoValidUntil != "" {
		validUntil, err := time.Parse(time.DateOnly, req.PromoValidUntil)
		if err != nil {
			return nil, "promo_valid_until must be YYYY-MM-DD or empty"
		}
		partner.PromoValidUntil = &validUntil
	}
	return partner, ""
}

func toPartnerResponse(partner *model.Partner) partnerResponse {
	response := partnerResponse{
		ID:               partner.ID,
		Name:             partner.Name,
		Day:              partner.Day.Format(time.DateOnly),
		MaxChannelID:     partner.MaxChannelID,
		MaxURL:           partner.MaxURL,
		PromoWhere:       partner.PromoWhere,
		PrizeName:        partner.PrizeName,
		PrizeInstruction: partner.PrizeInstruction,
		WinnersCount:     partner.WinnersCount,
		CreatedAt:        partner.CreatedAt,
	}
	if partner.PromoValidUntil != nil {
		validUntil := partner.PromoValidUntil.Format(time.DateOnly)
		response.PromoValidUntil = &validUntil
	}
	return response
}

type importPromocodesRequest struct {
	Codes []string `json:"codes"`
}

type importPromocodesResponse struct {
	Imported int64 `json:"imported"`
}

// importPromocodes uploads a batch of free codes for the partner
// (POST /api/admin/partners/{partnerID}/promocodes).
func (s *Server) importPromocodes(w http.ResponseWriter, r *http.Request) {
	partnerID, ok := routeUUID(w, r, "partnerID")
	if !ok {
		return
	}

	var req importPromocodesRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if len(req.Codes) == 0 {
		apierr.Write(w, r, apierr.InvalidRequest("codes must not be empty"))
		return
	}
	if len(req.Codes) > maxPromoBatchSize {
		apierr.Write(w, r, apierr.InvalidRequest("too many codes in one batch"))
		return
	}

	codes := make([]string, 0, len(req.Codes))
	for _, code := range req.Codes {
		code = strings.TrimSpace(code)
		if code == "" {
			apierr.Write(w, r, apierr.InvalidRequest("codes must not contain empty values"))
			return
		}
		codes = append(codes, code)
	}

	if _, err := s.services.PartnerByID(r.Context(), partnerID); err != nil {
		s.writeStorageError(w, r, err, "find partner")
		return
	}

	imported, err := s.services.ImportCodes(r.Context(), partnerID, codes)
	if err != nil {
		s.log.ErrorContext(r.Context(), "import promocodes failed", logger.Err(err))
		apierr.Write(w, r, apierr.Internal())
		return
	}

	writeJSON(w, r, http.StatusOK, importPromocodesResponse{Imported: imported})
}

// getStats returns the analytics snapshot (GET /api/admin/stats).
func (s *Server) getStats(w http.ResponseWriter, r *http.Request) {
	stats, err := s.services.Get(r.Context())
	if err != nil {
		s.log.ErrorContext(r.Context(), "collect stats failed", logger.Err(err))
		apierr.Write(w, r, apierr.Internal())
		return
	}
	writeJSON(w, r, http.StatusOK, stats)
}

type broadcastRequest struct {
	Text string `json:"text"`
}

type broadcastResponse struct {
	Sent   int `json:"sent"`
	Failed int `json:"failed"`
}

// broadcast pushes a message to every bot user (POST /api/admin/broadcast).
func (s *Server) broadcast(w http.ResponseWriter, r *http.Request) {
	var req broadcastRequest
	if !decodeJSON(w, r, &req) {
		return
	}

	text := strings.TrimSpace(req.Text)
	if text == "" {
		apierr.Write(w, r, apierr.InvalidRequest("text is required"))
		return
	}
	if utf8.RuneCountInString(text) > maxBroadcastTextLen {
		apierr.Write(w, r, apierr.InvalidRequest("text is too long"))
		return
	}

	sent, failed, err := s.services.Send(r.Context(), text)
	if err != nil {
		s.log.ErrorContext(r.Context(), "broadcast failed", logger.Err(err))
		apierr.Write(w, r, apierr.Internal())
		return
	}
	writeJSON(w, r, http.StatusOK, broadcastResponse{Sent: sent, Failed: failed})
}

// routeUUID parses a UUID URL parameter, answering 400 on a bad value.
func routeUUID(w http.ResponseWriter, r *http.Request, param string) (uuid.UUID, bool) {
	id, err := uuid.Parse(chi.URLParam(r, param))
	if err != nil {
		apierr.Write(w, r, apierr.InvalidRequest(param+" must be a UUID"))
		return uuid.Nil, false
	}
	return id, true
}
