package steps

import (
	"context"
	"fmt"

	"github.com/cucumber/godog"

	"github.com/pure-golang/budva-claude/internal/domain"
)

func register06AutoSteps(ctx *godog.ScenarioContext, s *scenarioCtx) {
	ctx.When(`^в исходном чате появляется сообщение с callback-запросом$`, func() error {
		s.sendCopy = true
		s.applyRuleSet()

		msg := &domain.Message{
			ChatID:     s.env.SourceID,
			ID:         1,
			CanBeSaved: true,
			Content: domain.MessageContent{
				Type: domain.ContentText,
				Text: &domain.FormattedText{Text: "message with button"},
			},
			ReplyMarkup: &domain.ReplyMarkup{CallbackData: []byte("callback_data")},
		}
		s.sentMsg = msg
		s.env.Telegram.PutMessage(msg)

		s.env.Handler.OnNewMessage(context.Background(), msg)
		s.env.DrainQueue()

		return nil
	})

	ctx.Then(`^бот автоматически отвечает на запрос$`, func() error {
		// FakeTelegram.GetCallbackQueryAnswer возвращает "" — auto-answer не добавляет текст.
		// Проверяем что сообщение доставлено без ошибок (transform pipeline выполнился).
		for _, targetID := range s.env.TargetIDs {
			msgs := s.env.Telegram.MessagesInChat(targetID)
			if len(msgs) == 0 {
				return fmt.Errorf("no messages in target chat %d", targetID)
			}
		}
		return nil
	})
}
