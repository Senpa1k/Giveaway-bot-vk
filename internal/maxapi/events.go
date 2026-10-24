package maxapi

import "strings"

// Update types the bot reacts to, delivered by webhooks and long polling.
const (
	UpdateBotStarted      = "bot_started"
	UpdateMessageCreated  = "message_created"
	UpdateMessageCallback = "message_callback"
)

// CommandStart is the command MAX sends when the user presses the start
// button of the bot.
const CommandStart = "start"

// Update is a single event sent by MAX.
type Update struct {
	UpdateType string    `json:"update_type"`
	Timestamp  int64     `json:"timestamp"`
	ChatID     int64     `json:"chat_id"`
	User       *User     `json:"user,omitempty"`
	Message    *Message  `json:"message,omitempty"`
	Callback   *Callback `json:"callback,omitempty"`
}

// User is a MAX account.
type User struct {
	UserID   int64  `json:"user_id"`
	Name     string `json:"name"`
	Username string `json:"username"`
	IsBot    bool   `json:"is_bot"`
}

// Message is a message in a dialog, chat or channel.
type Message struct {
	Sender    *User      `json:"sender,omitempty"`
	Recipient *Recipient `json:"recipient,omitempty"`
	Timestamp int64      `json:"timestamp"`
	Body      *Body      `json:"body,omitempty"`
}

// Recipient tells where the message lives; ChatType separates user dialogs
// from group chats and channels.
type Recipient struct {
	ChatType string `json:"chat_type"`
	UserID   int64  `json:"user_id,omitempty"`
	ChatID   int64  `json:"chat_id,omitempty"`
}

// Chat types of a recipient.
const (
	ChatTypeDialog  = "dialog"
	ChatTypeChat    = "chat"
	ChatTypeChannel = "channel"
)

// Body is the content of a message.
type Body struct {
	Mid  string `json:"mid,omitempty"`
	Text string `json:"text,omitempty"`
}

// Callback is the payload of a message_callback update: a button press.
type Callback struct {
	CallbackID   string   `json:"callback_id"`
	CallbackData string   `json:"callback_data"`
	Message      *Message `json:"message,omitempty"`
}

// IncomingMessage is the platform-independent bot event consumed by the
// service layer, built from an Update by DecodeUpdate.
type IncomingMessage struct {
	// UserID is the MAX id of the acting user.
	UserID int64
	// Text is the message text, empty for button presses.
	Text string
	// Command is the button payload or start, empty for plain texts.
	Command string
	// CallbackID identifies a button press for the /answers call; empty for
	// other events.
	CallbackID string
	// Dialog reports whether the event happened in a private user dialog.
	Dialog bool
}

// TrimmedText returns the message text without surrounding whitespace.
func (m IncomingMessage) TrimmedText() string {
	return strings.TrimSpace(m.Text)
}

// DecodeUpdate converts a MAX update into an incoming bot message. The
// second result is false for updates the bot does not react to: non-dialog
// messages, channel posts and service events.
func DecodeUpdate(update Update) (IncomingMessage, bool) {
	switch update.UpdateType {
	case UpdateBotStarted:
		if update.User == nil {
			return IncomingMessage{}, false
		}
		return IncomingMessage{
			UserID:  update.User.UserID,
			Command: CommandStart,
			Dialog:  true,
		}, true

	case UpdateMessageCreated:
		if update.Message == nil || update.Message.Sender == nil {
			return IncomingMessage{}, false
		}
		dialog := update.Message.Recipient != nil && update.Message.Recipient.ChatType == ChatTypeDialog
		if !dialog {
			return IncomingMessage{}, false
		}
		text := ""
		if update.Message.Body != nil {
			text = update.Message.Body.Text
		}
		return IncomingMessage{
			UserID: update.Message.Sender.UserID,
			Text:   text,
			Dialog: true,
		}, true

	case UpdateMessageCallback:
		if update.User == nil || update.Callback == nil {
			return IncomingMessage{}, false
		}
		return IncomingMessage{
			UserID:     update.User.UserID,
			Command:    update.Callback.CallbackData,
			CallbackID: update.Callback.CallbackID,
			Dialog:     true,
		}, true

	default:
		return IncomingMessage{}, false
	}
}
