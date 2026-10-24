package vkapi

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestKeyboardJSON(t *testing.T) {
	keyboard := NewInlineKeyboard().
		TextRow("Получить промокод", "promo", ColorPrimary).
		LinkRow("Подписаться на партнера", "https://vk.com/partner")

	encoded, err := json.Marshal(keyboard)
	if err != nil {
		t.Fatalf("Marshal() error: %v", err)
	}
	var decoded struct {
		Inline  bool `json:"inline"`
		Buttons [][]struct {
			Action struct {
				Type    string `json:"type"`
				Label   string `json:"label"`
				Payload string `json:"payload"`
				Link    string `json:"link"`
			} `json:"action"`
			Color string `json:"color"`
		} `json:"buttons"`
	}
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatalf("Unmarshal() error: %v", err)
	}

	if !decoded.Inline {
		t.Error("keyboard must be inline")
	}
	if len(decoded.Buttons) != 2 {
		t.Fatalf("rows = %d, want 2", len(decoded.Buttons))
	}
	text := decoded.Buttons[0][0]
	if text.Action.Type != "text" || text.Action.Label != "Получить промокод" || text.Color != ColorPrimary {
		t.Errorf("text button = %+v", text)
	}
	if !strings.Contains(text.Action.Payload, `"cmd":"promo"`) {
		t.Errorf("payload = %q, want the promo command", text.Action.Payload)
	}
	link := decoded.Buttons[1][0]
	if link.Action.Type != "open_link" || link.Action.Link != "https://vk.com/partner" {
		t.Errorf("link button = %+v", link)
	}
}
