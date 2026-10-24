package vkapi

import "encoding/json"

// Button colors of VK text buttons.
const (
	ColorPrimary   = "primary"
	ColorSecondary = "secondary"
	ColorNegative  = "negative"
	ColorPositive  = "positive"
)

type buttonAction struct {
	Type    string `json:"type"`
	Label   string `json:"label,omitempty"`
	Payload string `json:"payload,omitempty"`
	Link    string `json:"link,omitempty"`
}

type button struct {
	Action buttonAction `json:"action"`
	Color  string       `json:"color,omitempty"`
}

// Keyboard is a VK message keyboard. Inline keyboards travel with the
// message and disappear with it.
type Keyboard struct {
	Inline  bool       `json:"inline"`
	Buttons [][]button `json:"buttons"`
}

// NewInlineKeyboard creates an empty inline keyboard.
func NewInlineKeyboard() *Keyboard {
	return &Keyboard{Inline: true, Buttons: make([][]button, 0)}
}

// TextRow appends a row with one text button that sends the command back to
// the bot through its payload.
func (k *Keyboard) TextRow(label, cmd, color string) *Keyboard {
	k.Buttons = append(k.Buttons, []button{{
		Action: buttonAction{Type: "text", Label: label, Payload: commandPayload(cmd)},
		Color:  color,
	}})
	return k
}

// LinkRow appends a row with one open-link button; link buttons have no color.
func (k *Keyboard) LinkRow(label, link string) *Keyboard {
	k.Buttons = append(k.Buttons, []button{{
		Action: buttonAction{Type: "open_link", Label: label, Link: link},
	}})
	return k
}

// commandPayload encodes a button command as the VK payload JSON.
func commandPayload(cmd string) string {
	payload, err := json.Marshal(map[string]string{"cmd": cmd})
	if err != nil {
		// map[string]string always marshals; keep a safe fallback.
		return `{"cmd":"start"}`
	}
	return string(payload)
}
