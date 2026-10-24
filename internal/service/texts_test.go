package service

import "testing"

func TestCommandByText(t *testing.T) {
	tests := []struct {
		text string
		want string
	}{
		{"Начать", CmdStart},
		{"  начать  ", CmdStart},
		{"Получить промокод", CmdPromo},
		{"Участвовать в розыгрыше", CmdJoin},
		{"Проверить подписку", CmdCheck},
		{"Проверить еще раз", CmdCheck},
		{"Как участвовать", CmdHowTo},
		{"Мои шансы", CmdChances},
		{"Правила акции", CmdRules},
		{"Получить приз", CmdClaim},
		{"Согласен, передать данные", CmdConsentYes},
		{"Отказаться", CmdConsentNo},
		{"Согласен отдать все данные", ""},
		{"", ""},
	}

	for _, tt := range tests {
		if got := CommandByText(tt.text); got != tt.want {
			t.Errorf("CommandByText(%q) = %q, want %q", tt.text, got, tt.want)
		}
	}
}
