package vkapi

import (
	"encoding/json"
	"fmt"
	"strings"
)

// Event types delivered by Callback API and Bots Long Poll.
const (
	EventConfirmation = "confirmation"
	EventMessageNew   = "message_new"
)

// Event is a single update sent by VK.
type Event struct {
	Type    string          `json:"type"`
	Object  json.RawMessage `json:"object"`
	GroupID int64           `json:"group_id"`
	Secret  string          `json:"secret"`
}

// Message decodes the message_new object of the event.
func (e Event) Message() (IncomingMessage, error) {
	var object struct {
		Message IncomingMessage `json:"message"`
	}
	if err := json.Unmarshal(e.Object, &object); err != nil {
		return IncomingMessage{}, fmt.Errorf("decode message_new object: %w", err)
	}
	return object.Message, nil
}

// IncomingMessage is a private message sent to the community.
type IncomingMessage struct {
	PeerID  int64  `json:"peer_id"`
	FromID  int64  `json:"from_id"`
	Text    string `json:"text"`
	Payload string `json:"payload"`
}

// Command returns the button command of the message: the cmd or command
// field of the button payload. VK's start button uses "command". Empty
// when the message has no usable payload.
func (m IncomingMessage) Command() string {
	if m.Payload == "" {
		return ""
	}
	var payload struct {
		Cmd     string `json:"cmd"`
		Command string `json:"command"`
	}
	if err := json.Unmarshal([]byte(m.Payload), &payload); err != nil {
		return ""
	}
	if payload.Cmd != "" {
		return payload.Cmd
	}
	return payload.Command
}

// IsPrivate reports whether the message came from a user dialog, not a chat.
func (m IncomingMessage) IsPrivate() bool {
	return m.PeerID == m.FromID && m.PeerID > 0 && m.PeerID < 2_000_000_000
}

// TrimmedText returns the message text without surrounding whitespace.
func (m IncomingMessage) TrimmedText() string {
	return strings.TrimSpace(m.Text)
}
