package vkapi

import "testing"

func TestIncomingMessageCommand(t *testing.T) {
	tests := []struct {
		name    string
		payload string
		want    string
	}{
		{"cmd payload", `{"cmd":"promo"}`, "promo"},
		{"command payload", `{"command":"start"}`, "start"},
		{"cmd wins over command", `{"cmd":"join","command":"start"}`, "join"},
		{"empty payload", "", ""},
		{"broken payload", "{cmd", ""},
		{"no command fields", `{"foo":1}`, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			message := IncomingMessage{Payload: tt.payload}
			if got := message.Command(); got != tt.want {
				t.Fatalf("Command() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestIncomingMessageIsPrivate(t *testing.T) {
	private := IncomingMessage{PeerID: 100, FromID: 100}
	if !private.IsPrivate() {
		t.Error("a user dialog must be private")
	}

	chat := IncomingMessage{PeerID: 2_000_000_100, FromID: 100}
	if chat.IsPrivate() {
		t.Error("a group chat must not be private")
	}

	group := IncomingMessage{PeerID: -100, FromID: -100}
	if group.IsPrivate() {
		t.Error("a community message must not be private")
	}
}
