package model

import "github.com/google/uuid"

// PartnerPromoStats is the promocode balance of one partner.
type PartnerPromoStats struct {
	PartnerID uuid.UUID `json:"partner_id"`
	Name      string    `json:"name"`
	Day       string    `json:"day"`
	Requests  int64     `json:"requests"`
	Total     int64     `json:"total"`
	Issued    int64     `json:"issued"`
	Remaining int64     `json:"remaining"`
}

// DrawStats is the participation summary of one draw.
type DrawStats struct {
	DrawID       uuid.UUID  `json:"draw_id"`
	Kind         DrawKind   `json:"kind"`
	Date         string     `json:"date"`
	PartnerName  string     `json:"partner_name"`
	Status       DrawStatus `json:"status"`
	Participants int64      `json:"participants"`
}

// Stats aggregates the analytics of the technical spec: user growth,
// promocode demand and supply, draw participation.
type Stats struct {
	TotalUsers         int64              `json:"total_users"`
	NewUsersByDay      []DayCount         `json:"new_users_by_day"`
	PromoRequestsTotal int64              `json:"promo_requests_total"`
	PromoIssuedTotal   int64              `json:"promo_issued_total"`
	PromoByPartner     []PartnerPromoStats `json:"promo_by_partner"`
	Draws              []DrawStats        `json:"draws"`
}
