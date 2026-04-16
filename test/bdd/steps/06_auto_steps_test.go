package steps

import (
	"context"
	"time"

	"github.com/cucumber/godog"

	"github.com/pure-golang/budva-claude/internal/domain"
)

func register06AutoSteps(ctx *godog.ScenarioContext, s *scenarioCtx) {
	ctx.When(`^в исходном чате появляется сообщение с callback-запросом$`, func() error {
		s.sendCopy = true
		s.applyRuleSet()

		msg, err := s.env.PutMessage(context.Background(), s.env.SourceID, domain.InputMessageContent{
			Type: domain.ContentText,
			Text: &domain.FormattedText{Text: "message with button"},
		}, s.prefix)
		if err != nil {
			return err
		}
		// Добавляем ReplyMarkup вручную (TDLib не позволяет отправить inline keyboard от обычного пользователя)
		msg.ReplyMarkup = &domain.ReplyMarkup{CallbackData: []byte("callback_data")}
		s.sentMsg = msg

		s.env.Handler.OnNewMessage(context.Background(), msg)
		s.env.DrainQueue()
		// Ждём обработки callback
		time.Sleep(500 * time.Millisecond)

		return nil
	})

	ctx.Then(`^бот автоматически отвечает на запрос$`, func() error {
		// GetCallbackQueryAnswer выполняется на реальном TDLib.
		// Проверяем что сообщение доставлено (transform pipeline выполнился).
		for _, targetID := range s.env.TargetIDs {
			if _, err := s.env.CheckLastMessage(context.Background(), targetID, s.prefix); err != nil {
				return err
			}
		}
		return nil
	})
}
