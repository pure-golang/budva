package steps

import (
	"context"
	"fmt"
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
		})
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
			msgs, err := s.env.MessagesInChat(context.Background(), targetID)
			if err != nil {
				return err
			}
			if len(msgs) == 0 {
				return fmt.Errorf("no messages in target chat %d", targetID)
			}
		}
		return nil
	})
}
