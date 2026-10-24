package handlers

import (
	"bytes"
	"encoding/csv"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/Senpa1k/Giveaway-bot-vk/internal/http-server/apierr"
	"github.com/Senpa1k/Giveaway-bot-vk/internal/lib/logger"
	"github.com/Senpa1k/Giveaway-bot-vk/internal/model"

	"github.com/google/uuid"
)

type drawResponse struct {
	ID           uuid.UUID  `json:"id"`
	Kind         string     `json:"kind"`
	Day          string     `json:"day"`
	PartnerID    *uuid.UUID `json:"partner_id"`
	PartnerName  string     `json:"partner_name"`
	Status       string     `json:"status"`
	WinnersCount int        `json:"winners_count"`
	ReserveCount int        `json:"reserve_count"`
	Participants int        `json:"participants"`
	DrawnAt      *time.Time `json:"drawn_at"`
	ApprovedAt   *time.Time `json:"approved_at"`
	ApprovedBy   string     `json:"approved_by"`
	NotifiedAt   *time.Time `json:"notified_at"`
}

type winnerResponse struct {
	ID           uuid.UUID  `json:"id"`
	Position     int        `json:"position"`
	Kind         string     `json:"kind"`
	Status       string     `json:"status"`
	Prize        string     `json:"prize"`
	Tickets      int        `json:"tickets"`
	UserID       uuid.UUID  `json:"user_id"`
	MaxID        int64      `json:"max_id"`
	ContactName  string     `json:"contact_name"`
	ConsentGiven bool       `json:"consent_given"`
	NotifiedAt   *time.Time `json:"notified_at"`
}

type participantResponse struct {
	UserID       uuid.UUID `json:"user_id"`
	MaxID        int64     `json:"max_id"`
	RegisteredAt time.Time `json:"registered_at"`
}

type drawDetailResponse struct {
	Draw         drawResponse          `json:"draw"`
	Winners      []winnerResponse      `json:"winners"`
	Participants []participantResponse `json:"participants"`
}

type runDrawRequest struct {
	Day string `json:"day"`
}

// listDraws returns all draws with participant counts
// (GET /api/admin/draws).
func (s *Server) listDraws(w http.ResponseWriter, r *http.Request) {
	draws, err := s.services.ListDraws(r.Context())
	if err != nil {
		s.log.ErrorContext(r.Context(), "list draws failed", logger.Err(err))
		apierr.Write(w, r, apierr.Internal())
		return
	}

	stats, err := s.services.Get(r.Context())
	if err != nil {
		s.log.ErrorContext(r.Context(), "collect draw stats failed", logger.Err(err))
		apierr.Write(w, r, apierr.Internal())
		return
	}
	participants := make(map[uuid.UUID]int64, len(stats.Draws))
	for _, draw := range stats.Draws {
		participants[draw.DrawID] = draw.Participants
	}

	response := make([]drawResponse, 0, len(draws))
	for i := range draws {
		response = append(response, drawResponse{
			ID:           draws[i].ID,
			Kind:         string(draws[i].Kind),
			Day:          draws[i].DrawDate.Format(time.DateOnly),
			PartnerID:    draws[i].PartnerID,
			Status:       string(draws[i].Status),
			WinnersCount: draws[i].WinnersCount,
			ReserveCount: draws[i].ReserveCount,
			Participants: int(participants[draws[i].ID]),
			DrawnAt:      draws[i].DrawnAt,
			ApprovedAt:   draws[i].ApprovedAt,
			ApprovedBy:   draws[i].ApprovedBy,
			NotifiedAt:   draws[i].NotifiedAt,
		})
	}
	writeJSON(w, r, http.StatusOK, response)
}

// runDailyDraw picks the winners of the partner day
// (POST /api/admin/draws/daily). The day defaults to the current campaign
// day. Results stay silent until an admin approves them.
func (s *Server) runDailyDraw(w http.ResponseWriter, r *http.Request) {
	day, ok := s.requestDay(w, r)
	if !ok {
		return
	}

	result, err := s.services.RunDaily(r.Context(), day)
	if err != nil {
		s.writeDrawError(w, r, err, "run daily draw")
		return
	}
	s.writeDrawResult(w, r, result)
}

// runFinalDraw picks the winners of the final draw
// (POST /api/admin/draws/final). The day defaults to the current campaign
// day; every accumulated chance is one ticket.
func (s *Server) runFinalDraw(w http.ResponseWriter, r *http.Request) {
	day, ok := s.requestDay(w, r)
	if !ok {
		return
	}

	result, err := s.services.RunFinal(r.Context(), day)
	if err != nil {
		s.writeDrawError(w, r, err, "run final draw")
		return
	}
	s.writeDrawResult(w, r, result)
}

// getDraw returns the draw with its winners and participants
// (GET /api/admin/draws/{drawID}).
func (s *Server) getDraw(w http.ResponseWriter, r *http.Request) {
	drawID, ok := routeUUID(w, r, "drawID")
	if !ok {
		return
	}

	draw, err := s.services.DrawByID(r.Context(), drawID)
	if err != nil {
		s.writeStorageError(w, r, err, "find draw")
		return
	}

	detail, err := s.drawDetail(w, r, draw)
	if err != nil {
		return
	}
	writeJSON(w, r, http.StatusOK, detail)
}

type approveDrawRequest struct {
	ApprovedBy string `json:"approved_by"`
}

// approveDraw confirms the drawn results and sends the winner messages
// (POST /api/admin/draws/{drawID}/approve).
func (s *Server) approveDraw(w http.ResponseWriter, r *http.Request) {
	drawID, ok := routeUUID(w, r, "drawID")
	if !ok {
		return
	}

	var req approveDrawRequest
	if !decodeJSON(w, r, &req) {
		return
	}

	if _, err := s.services.Approve(r.Context(), drawID, req.ApprovedBy); err != nil {
		s.writeStorageError(w, r, err, "approve draw")
		return
	}
	if err := s.services.NotifyDraw(r.Context(), drawID); err != nil {
		s.writeStorageError(w, r, err, "notify winners")
		return
	}

	draw, err := s.services.DrawByID(r.Context(), drawID)
	if err != nil {
		s.writeStorageError(w, r, err, "find draw")
		return
	}
	detail, err := s.drawDetail(w, r, draw)
	if err != nil {
		return
	}
	writeJSON(w, r, http.StatusOK, detail)
}

type missWinnerResponse struct {
	MissedWinnerID   uuid.UUID  `json:"missed_winner_id"`
	PromotedWinnerID *uuid.UUID `json:"promoted_winner_id"`
	PromotedMaxID    *int64     `json:"promoted_max_id"`
}

// missWinner closes a missed prize and hands it to the first reserve winner
// (POST /api/admin/winners/{winnerID}/miss).
func (s *Server) missWinner(w http.ResponseWriter, r *http.Request) {
	winnerID, ok := routeUUID(w, r, "winnerID")
	if !ok {
		return
	}

	promoted, err := s.services.Miss(r.Context(), winnerID)
	if err != nil {
		s.writeStorageError(w, r, err, "transfer prize to reserve")
		return
	}

	response := missWinnerResponse{MissedWinnerID: winnerID}
	if promoted != nil {
		users, err := s.services.ByIDs(r.Context(), []uuid.UUID{promoted.UserID})
		if err != nil {
			s.log.ErrorContext(r.Context(), "load promoted winner failed", logger.Err(err))
		} else if user, found := users[promoted.UserID]; found {
			response.PromotedWinnerID = &promoted.ID
			response.PromotedMaxID = &user.MaxID
		}
	}
	writeJSON(w, r, http.StatusOK, response)
}

// exportDraw streams the winners or the participants of the draw as CSV for
// the MAX team and the partner (GET /api/admin/draws/{drawID}/export?type=winners|participants).
func (s *Server) exportDraw(w http.ResponseWriter, r *http.Request) {
	drawID, ok := routeUUID(w, r, "drawID")
	if !ok {
		return
	}

	draw, err := s.services.DrawByID(r.Context(), drawID)
	if err != nil {
		s.writeStorageError(w, r, err, "find draw")
		return
	}
	detail, err := s.drawDetail(w, r, draw)
	if err != nil {
		return
	}

	var buffer bytes.Buffer
	writer := csv.NewWriter(&buffer)
	if r.URL.Query().Get("type") == "participants" {
		_ = writer.Write([]string{"max_id", "user_id", "registered_at"})
		for _, participant := range detail.Participants {
			_ = writer.Write([]string{
				int64String(participant.MaxID),
				participant.UserID.String(),
				participant.RegisteredAt.Format(time.RFC3339),
			})
		}
	} else {
		_ = writer.Write([]string{"position", "kind", "status", "prize", "tickets", "max_id", "user_id", "contact_name", "consent_given", "notified_at"})
		for _, winner := range detail.Winners {
			notifiedAt := ""
			if winner.NotifiedAt != nil {
				notifiedAt = winner.NotifiedAt.Format(time.RFC3339)
			}
			_ = writer.Write([]string{
				intString(winner.Position),
				winner.Kind,
				winner.Status,
				winner.Prize,
				intString(winner.Tickets),
				int64String(winner.MaxID),
				winner.UserID.String(),
				winner.ContactName,
				boolString(winner.ConsentGiven),
				notifiedAt,
			})
		}
	}
	writer.Flush()
	if err := writer.Error(); err != nil {
		s.log.ErrorContext(r.Context(), "build draw export failed", logger.Err(err))
		apierr.Write(w, r, apierr.Internal())
		return
	}

	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="draw_`+draw.ID.String()+`.csv"`)
	_, _ = w.Write(buffer.Bytes())
}

// requestDay resolves the optional day of a draw request; empty means today.
func (s *Server) requestDay(w http.ResponseWriter, r *http.Request) (time.Time, bool) {
	var req runDrawRequest
	if !decodeJSON(w, r, &req) {
		return time.Time{}, false
	}
	if req.Day == "" {
		return s.today(), true
	}

	day, err := time.Parse(time.DateOnly, req.Day)
	if err != nil {
		apierr.Write(w, r, apierr.InvalidRequest("day must be YYYY-MM-DD"))
		return time.Time{}, false
	}
	return day, true
}

// writeDrawResult answers a run-draw request with the fresh results.
func (s *Server) writeDrawResult(w http.ResponseWriter, r *http.Request, result *model.DrawResult) {
	draw := result.Draw
	detail, err := s.drawDetail(w, r, &draw)
	if err != nil {
		return
	}
	writeJSON(w, r, http.StatusOK, detail)
}

// drawDetail loads the winners and participants of a draw for a response.
func (s *Server) drawDetail(w http.ResponseWriter, r *http.Request, draw *model.Draw) (*drawDetailResponse, error) {
	ctx := r.Context()

	entries, err := s.services.Entries(ctx, draw.ID)
	if err != nil {
		s.log.ErrorContext(ctx, "list draw entries failed", logger.Err(err))
		apierr.Write(w, r, apierr.Internal())
		return nil, err
	}
	winners, err := s.services.ListByDraw(ctx, draw.ID)
	if err != nil {
		s.log.ErrorContext(ctx, "list draw winners failed", logger.Err(err))
		apierr.Write(w, r, apierr.Internal())
		return nil, err
	}

	userIDs := make([]uuid.UUID, 0, len(entries)+len(winners))
	for _, entry := range entries {
		userIDs = append(userIDs, entry.UserID)
	}
	for _, winner := range winners {
		userIDs = append(userIDs, winner.UserID)
	}
	users, err := s.services.ByIDs(ctx, userIDs)
	if err != nil {
		s.log.ErrorContext(ctx, "load draw users failed", logger.Err(err))
		apierr.Write(w, r, apierr.Internal())
		return nil, err
	}

	response := &drawDetailResponse{
		Draw: drawResponse{
			ID:           draw.ID,
			Kind:         string(draw.Kind),
			Day:          draw.DrawDate.Format(time.DateOnly),
			PartnerID:    draw.PartnerID,
			Status:       string(draw.Status),
			WinnersCount: draw.WinnersCount,
			ReserveCount: draw.ReserveCount,
			Participants: len(entries),
			DrawnAt:      draw.DrawnAt,
			ApprovedAt:   draw.ApprovedAt,
			ApprovedBy:   draw.ApprovedBy,
			NotifiedAt:   draw.NotifiedAt,
		},
		Winners:      make([]winnerResponse, 0, len(winners)),
		Participants: make([]participantResponse, 0, len(entries)),
	}

	if draw.PartnerID != nil {
		partner, err := s.services.PartnerByID(ctx, *draw.PartnerID)
		if err == nil {
			response.Draw.PartnerName = partner.Name
		} else {
			s.log.ErrorContext(ctx, "find draw partner failed", logger.Err(err))
		}
	}

	for _, winner := range winners {
		response.Winners = append(response.Winners, winnerResponse{
			ID:           winner.ID,
			Position:     winner.Position,
			Kind:         string(winner.Kind),
			Status:       string(winner.Status),
			Prize:        winner.Prize,
			Tickets:      winner.Tickets,
			UserID:       winner.UserID,
			MaxID:        users[winner.UserID].MaxID,
			ContactName:  winner.ContactName,
			ConsentGiven: winner.ConsentGiven,
			NotifiedAt:   winner.NotifiedAt,
		})
	}
	for _, entry := range entries {
		response.Participants = append(response.Participants, participantResponse{
			UserID:       entry.UserID,
			MaxID:        users[entry.UserID].MaxID,
			RegisteredAt: entry.CreatedAt,
		})
	}
	return response, nil
}

// writeDrawError maps raffle errors to problems: missing partner or empty
// draws are not server failures but conflict-shaped answers.
func (s *Server) writeDrawError(w http.ResponseWriter, r *http.Request, err error, op string) {
	switch {
	case errors.Is(err, model.ErrPartnerNotFound):
		apierr.Write(w, r, apierr.NotFound(op+": no partner on this day"))
	case errors.Is(err, model.ErrDrawNotFound):
		apierr.Write(w, r, apierr.Conflict(op+": nobody registered that day"))
	case errors.Is(err, model.ErrNoEntries):
		apierr.Write(w, r, apierr.Conflict(op+": the draw has no entries"))
	case errors.Is(err, model.ErrInvalidDrawStatus):
		apierr.Write(w, r, apierr.Conflict(op+": the draw is not pending"))
	default:
		s.log.ErrorContext(r.Context(), op+" failed", logger.Err(err))
		apierr.Write(w, r, apierr.Internal())
	}
}

// writeStorageError maps storage sentinel errors to problems.
func (s *Server) writeStorageError(w http.ResponseWriter, r *http.Request, err error, op string) {
	switch {
	case errors.Is(err, model.ErrPartnerNotFound),
		errors.Is(err, model.ErrDrawNotFound),
		errors.Is(err, model.ErrWinnerNotFound),
		errors.Is(err, model.ErrPromocodeNotFound):
		apierr.Write(w, r, apierr.NotFound(op+": resource not found"))
	case errors.Is(err, model.ErrInvalidDrawStatus),
		errors.Is(err, model.ErrInvalidWinnerStatus),
		errors.Is(err, model.ErrAlreadyRegistered):
		apierr.Write(w, r, apierr.Conflict(op+": invalid state"))
	default:
		s.log.ErrorContext(r.Context(), op+" failed", logger.Err(err))
		apierr.Write(w, r, apierr.Internal())
	}
}

func intString(value int) string {
	return strconv.Itoa(value)
}

func int64String(value int64) string {
	return strconv.FormatInt(value, 10)
}

func boolString(value bool) string {
	if value {
		return "yes"
	}
	return "no"
}
