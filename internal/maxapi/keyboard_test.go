package maxapi

import (
	"encoding/json"
	"testing"
)

func TestKeyboardJSON(t *testing.T) {
	keyboard := NewInlineKeyboard().
		CallbackRow("Получить промокод", "promo").
		LinkRow("Подписаться на партнера", "https://max.ru/partner")

	encoded, err := json.Marshal(keyboard)
	if err != nil {
		t.Fatalf("Marshal() error: %v", err)
	}
	var decoded struct {
		Buttons [][]struct {
			Type    string `json:"type"`
			Text    string `json:"text"`
			Payload string `json:"payload"`
			URL     string `json:"url"`
		} `json:"buttons"`
	}
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatalf("Unmarshal() error: %v", err)
	}

	if len(decoded.Buttons) != 2 {
		t.Fatalf("rows = %d, want 2", len(decoded.Buttons))
	}
	callback := decoded.Buttons[0][0]
	if callback.Type != "callback" || callback.Text != "Получить промокод" || callback.Payload != "promo" {
		t.Errorf("callback button = %+v", callback)
	}
	link := decoded.Buttons[1][0]
	if link.Type != "link" || link.URL != "https://max.ru/partner" {
		t.Errorf("link button = %+v", link)
	}
	if link.Payload != "" {
		t.Errorf("link button payload = %q, want empty", link.Payload)
	}
}
