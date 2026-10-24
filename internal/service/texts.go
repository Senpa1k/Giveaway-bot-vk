package service

import (
	"fmt"
	"strings"
	"time"

	"github.com/Senpa1k/Giveaway-bot-vk/internal/model"
	"github.com/Senpa1k/Giveaway-bot-vk/internal/vkapi"
)

// Button commands carried by keyboard payloads and text aliases.
const (
	CmdStart      = "start"
	CmdPromo      = "promo"
	CmdJoin       = "join"
	CmdCheck      = "check"
	CmdHowTo      = "howto"
	CmdChances    = "chances"
	CmdRules      = "rules"
	CmdClaim      = "claim"
	CmdConsentYes = "consent_yes"
	CmdConsentNo  = "consent_no"
)

// commandAliases maps button labels and common texts to commands, so the bot
// answers typed text the same way as button presses.
var commandAliases = map[string]string{
	"начать":            CmdStart,
	"start":             CmdStart,
	"получить промокод": CmdPromo,
	"участвовать в розыгрыше":   CmdJoin,
	"подписаться":               CmdJoin,
	"проверить подписку":        CmdCheck,
	"проверить еще раз":         CmdCheck,
	"как участвовать":           CmdHowTo,
	"мои шансы":                 CmdChances,
	"правила акции":             CmdRules,
	"получить приз":             CmdClaim,
	"согласен, передать данные": CmdConsentYes,
	"отказаться":                CmdConsentNo,
}

// CommandByText resolves a typed message to a command; empty when unknown.
func CommandByText(text string) string {
	return commandAliases[strings.ToLower(strings.TrimSpace(text))]
}

// mainMenu is the keyboard of the welcome screen and help replies.
func mainMenu() *vkapi.Keyboard {
	return vkapi.NewInlineKeyboard().
		TextRow("Получить промокод", CmdPromo, vkapi.ColorPrimary).
		TextRow("Участвовать в розыгрыше", CmdJoin, vkapi.ColorPrimary).
		TextRow("Как участвовать", CmdHowTo, vkapi.ColorSecondary).
		TextRow("Мои шансы", CmdChances, vkapi.ColorSecondary).
		TextRow("Правила акции", CmdRules, vkapi.ColorSecondary)
}

func welcomeReply(banner string) *Reply {
	return &Reply{
		Text:       "Добро пожаловать в Бот розыгрышей",
		Keyboard:   mainMenu(),
		Attachment: banner,
	}
}

func helpReply() *Reply {
	return &Reply{
		Text:     "Я здесь, чтобы раздавать призы 🎁\n\nВыбери действие в меню ниже.",
		Keyboard: mainMenu(),
	}
}

func noPartnerReply() *Reply {
	return &Reply{
		Text:     "Сегодня партнеров на сцене нет 🎧\n\nВозвращайся завтра — розыгрыши уже готовятся.",
		Keyboard: mainMenu(),
	}
}

// promoSuccessReply announces an issued code; a repeated press of the day
// shows the same message with the same code.
func promoSuccessReply(partner *model.Partner, code string) *Reply {
	text := fmt.Sprintf("Твой фестивальный бонус от %s готов 🎁\n\nПромокод: %s\n\n", partner.Name, code)
	if partner.PromoValidUntil != nil {
		text += fmt.Sprintf("Используй его до %s", partner.PromoValidUntil.Format("02.01.2006"))
		if partner.PromoWhere != "" {
			text += fmt.Sprintf(" на %s", partner.PromoWhere)
		}
		text += ".\n\n"
	} else if partner.PromoWhere != "" {
		text += fmt.Sprintf("Используй его на %s.\n\n", partner.PromoWhere)
	}
	text += "Теперь можешь принять участие в розыгрыше призов дня."

	return &Reply{
		Text: text,
		Keyboard: vkapi.NewInlineKeyboard().
			TextRow("Участвовать в розыгрыше", CmdJoin, vkapi.ColorPrimary),
	}
}

func promoDepletedReply(partner *model.Partner) *Reply {
	return &Reply{
		Text: fmt.Sprintf(
			"Промокоды от %s уже закончились — их разобрали быстрее всех 🔥\n\n"+
				"Но ты все еще можешь подписаться на партнера и принять участие в розыгрыше призов дня.",
			partner.Name,
		),
		Keyboard: vkapi.NewInlineKeyboard().
			TextRow("Участвовать в розыгрыше", CmdJoin, vkapi.ColorPrimary),
	}
}

func joinScreenReply(partner *model.Partner) *Reply {
	return &Reply{
		Text: fmt.Sprintf(
			"Чтобы попасть в розыгрыш призов от %s, подпишись на аккаунт партнера.",
			partner.Name,
		),
		Keyboard: vkapi.NewInlineKeyboard().
			LinkRow("Подписаться на "+partner.Name, partner.VKURL).
			TextRow("Проверить подписку", CmdCheck, vkapi.ColorPrimary),
	}
}

// subscriptionFailedReply asks to retry when the VK check itself failed.
func subscriptionFailedReply() *Reply {
	return &Reply{
		Text: "Не получилось проверить подписку — попробуй еще раз через минуту.",
		Keyboard: vkapi.NewInlineKeyboard().
			TextRow("Проверить подписку", CmdCheck, vkapi.ColorPrimary),
	}
}

func notSubscribedReply(partner *model.Partner) *Reply {
	return &Reply{
		Text: fmt.Sprintf(
			"Пока не вижу подписку 👀\n\nПодпишись на аккаунт %s, вернись в бот и попробуй еще раз.",
			partner.Name,
		),
		Keyboard: vkapi.NewInlineKeyboard().
			LinkRow("Подписаться", partner.VKURL).
			TextRow("Проверить еще раз", CmdCheck, vkapi.ColorPrimary),
	}
}

func joinedReply(partner *model.Partner, totalChances int) *Reply {
	return &Reply{
		Text: fmt.Sprintf(
			"Ты в лайнапе ✅\n\n"+
				"Заявка на розыгрыш призов от %s зарегистрирована.\n\n"+
				"Возвращайся завтра — на сцену выйдет новый партнер.\n\n"+
				"Накоплено шансов в финальном розыгрыше: %d.",
			partner.Name, totalChances,
		),
		Keyboard: mainMenu(),
	}
}

func alreadyJoinedReply() *Reply {
	return &Reply{
		Text:     "Ты уже участвуешь в сегодняшнем розыгрыше ✅\n\nРезультаты сообщим после подведения итогов.",
		Keyboard: mainMenu(),
	}
}

// chancesView is the data of the «Мои шансы» screen.
type chancesView struct {
	Count      int
	MaxChances int
	Days       int
	Partners   []string
	Remaining  int
	MainPrize  string
}

func chancesReply(view chancesView) *Reply {
	partners := "пока никто"
	if len(view.Partners) > 0 {
		partners = strings.Join(view.Partners, ", ")
	}

	return &Reply{
		Text: fmt.Sprintf(
			"Твой фестивальный пропуск 🎟\n\n"+
				"Накоплено шансов: %d из %d\n"+
				"Пройдено дней: %d\n"+
				"Партнеры, с которыми ты уже взаимодействовал: %s\n"+
				"Осталось дней акции: %d\n\n"+
				"Главный приз: %s\n\n"+
				"Чем больше дней розыгрыша ты посетишь, тем выше вероятность выиграть главный приз.",
			view.Count, view.MaxChances, view.Days, partners, view.Remaining, view.MainPrize,
		),
		Keyboard: mainMenu(),
	}
}

func howToReply(maxChances int) *Reply {
	return &Reply{
		Text: fmt.Sprintf(
			"Как участвовать 🎪\n\n"+
				"1. Заходи в бот каждый день — на сцене новый партнер.\n"+
				"2. Получай промокод дня и подпишись на аккаунт партнера.\n"+
				"3. Нажми «Проверить подписку», чтобы попасть в розыгрыш призов дня.\n"+
				"4. Каждый активный день дает один шанс в финальном розыгрыше — максимум %d.\n\n"+
				"Главный приз разыгрывается среди всех накопленных шансов.",
			maxChances,
		),
		Keyboard: mainMenu(),
	}
}

func rulesReply(maxChances int) *Reply {
	return &Reply{
		Text: fmt.Sprintf(
			"Правила акции 📜\n\n"+
				"• Розыгрыши проходят каждый день акции среди подписчиков партнера дня.\n"+
				"• Победители определяются автоматически и случайным образом.\n"+
				"• Один участник — одна заявка в ежедневном розыгрыше.\n"+
				"• Один активный день — один шанс в финальном розыгрыше, максимум %d.\n"+
				"• Для получения приза нужно оставить контактные данные и согласие на их передачу.\n"+
				"• Если победитель не отвечает в установленный срок, приз переходит резервному победителю.\n\n"+
				"Полные правила акции публикуются на странице организатора.",
			maxChances,
		),
		Keyboard: mainMenu(),
	}
}

// claimPrompt replies re-ask the pending question of the contact flow.
func claimPrompt(name string) *Reply {
	return &Reply{
		Text: fmt.Sprintf(
			"Передадим твои данные (%s) партнеру или билетному оператору, чтобы ты получил приз.\n\nСогласен на передачу данных?",
			name,
		),
		Keyboard: vkapi.NewInlineKeyboard().
			TextRow("Согласен, передать данные", CmdConsentYes, vkapi.ColorPositive).
			TextRow("Отказаться", CmdConsentNo, vkapi.ColorNegative),
	}
}

// russianDate formats a date for winner messages.
func russianDate(day time.Time) string {
	return day.Format("02.01.2006")
}
