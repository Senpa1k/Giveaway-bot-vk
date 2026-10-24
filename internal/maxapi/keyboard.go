package maxapi

import "encoding/json"

// Button types of inline keyboards.
const (
	ButtonCallback = "callback"
	ButtonLink     = "link"
)

// button is one inline keyboard button. A callback button sends its payload
// to the bot as a message_callback event; a link button opens the URL.
type button struct {
	Type    string `json:"type"`
	Text    string `json:"text"`
	Payload string `json:"payload,omitempty"`
	URL     string `json:"url,omitempty"`
}

// Keyboard is a MAX inline keyboard. It travels as the payload of the
// inline_keyboard attachment and marshals to {"buttons": [[...]]}, where each
// inner array is a row.
type Keyboard struct {
	rows [][]button
}

// NewInlineKeyboard creates an empty inline keyboard.
func NewInlineKeyboard() *Keyboard {
	return &Keyboard{rows: make([][]button, 0)}
}

// CallbackRow appends a row with one button that sends the command back to
// the bot through its payload.
func (k *Keyboard) CallbackRow(text, payload string) *Keyboard {
	k.rows = append(k.rows, []button{{Type: ButtonCallback, Text: text, Payload: payload}})
	return k
}

// LinkRow appends a row with one button that opens the URL.
func (k *Keyboard) LinkRow(text, link string) *Keyboard {
	k.rows = append(k.rows, []button{{Type: ButtonLink, Text: text, URL: link}})
	return k
}

// MarshalJSON encodes the keyboard as the inline_keyboard payload.
func (k *Keyboard) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Buttons [][]button `json:"buttons"`
	}{Buttons: k.rows})
}
