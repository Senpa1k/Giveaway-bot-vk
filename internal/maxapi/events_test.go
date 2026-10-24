package maxapi

import "testing"

func TestDecodeUpdateBotStarted(t *testing.T) {
	update := Update{
		UpdateType: UpdateBotStarted,
		User:       &User{UserID: 42},
	}
	message, ok := DecodeUpdate(update)
	if !ok {
		t.Fatal("bot_started must decode")
	}
	if message.UserID != 42 || message.Command != CommandStart || !message.Dialog {
		t.Errorf("message = %+v", message)
	}
	if message.CallbackID != "" {
		t.Errorf("callback id = %q, want empty", message.CallbackID)
	}
}

func TestDecodeUpdateMessageCreated(t *testing.T) {
	update := Update{
		UpdateType: UpdateMessageCreated,
		Message: &Message{
			Sender:    &User{UserID: 7},
			Recipient: &Recipient{ChatType: ChatTypeDialog},
			Body:      &Body{Text: "  получить промокод  "},
		},
	}
	message, ok := DecodeUpdate(update)
	if !ok {
		t.Fatal("a dialog message must decode")
	}
	if message.UserID != 7 || message.TrimmedText() != "получить промокод" || message.Command != "" {
		t.Errorf("message = %+v", message)
	}
	if !message.Dialog {
		t.Error("a dialog message must be a dialog event")
	}
}

func TestDecodeUpdateSkipsNonDialogs(t *testing.T) {
	tests := []struct {
		name   string
		update Update
	}{
		{
			"group chat message",
			Update{
				UpdateType: UpdateMessageCreated,
				Message: &Message{
					Sender:    &User{UserID: 7},
					Recipient: &Recipient{ChatType: ChatTypeChat},
				},
			},
		},
		{
			"channel post has no sender",
			Update{
				UpdateType: UpdateMessageCreated,
				Message: &Message{
					Recipient: &Recipient{ChatType: ChatTypeChannel},
				},
			},
		},
		{
			"message without recipient",
			Update{
				UpdateType: UpdateMessageCreated,
				Message:    &Message{Sender: &User{UserID: 7}},
			},
		},
		{
			"service update",
			Update{UpdateType: "bot_removed"},
		},
		{
			"bot_started without user",
			Update{UpdateType: UpdateBotStarted},
		},
		{
			"callback without user",
			Update{UpdateType: UpdateMessageCallback, Callback: &Callback{CallbackID: "1"}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, ok := DecodeUpdate(tt.update); ok {
				t.Error("the update must be skipped")
			}
		})
	}
}

func TestDecodeUpdateCallback(t *testing.T) {
	update := Update{
		UpdateType: UpdateMessageCallback,
		User:       &User{UserID: 9},
		Callback:   &Callback{CallbackID: "cb-1", CallbackData: "check"},
	}
	message, ok := DecodeUpdate(update)
	if !ok {
		t.Fatal("a callback must decode")
	}
	if message.UserID != 9 || message.Command != "check" || message.CallbackID != "cb-1" {
		t.Errorf("message = %+v", message)
	}
	if !message.Dialog {
		t.Error("a callback must be a dialog event")
	}
}
