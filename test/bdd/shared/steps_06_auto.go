package shared

import (
	"context"
	"time"

	"github.com/cucumber/godog"
	"github.com/zelenin/go-tdlib/client"
)

// RegisterAutoSteps регистрирует шаги эпика 06_auto.
func RegisterAutoSteps(ctx *godog.ScenarioContext, s *ScenarioCtx) {
	ctx.When(`^в исходном чате появляется сообщение с callback-запросом$`, func() error {
		s.SendCopy = true
		s.ApplyRuleSet()

		msg, err := s.Env.PutMessage(s.Env.SourceID, TextContent("message with button"), s.Prefix)
		if err != nil {
			return err
		}
		// Добавляем inline keyboard с callback-кнопкой вручную.
		// TDLib не позволяет отправить inline keyboard от обычного пользователя,
		// поэтому симулируем для unit-уровня: ReplyMarkup модифицируется в памяти
		// перед вызовом handler.
		msg.ReplyMarkup = &client.ReplyMarkupInlineKeyboard{
			Rows: [][]*client.InlineKeyboardButton{{
				{
					Text: "action",
					Type: &client.InlineKeyboardButtonTypeCallback{Data: []byte("callback_data")},
				},
			}},
		}
		s.SentMsg = msg

		s.Env.Handler.OnNewMessage(context.Background(), msg)
		s.Env.DrainQueue()
		time.Sleep(500 * time.Millisecond)

		return nil
	})

	ctx.Then(`^бот автоматически отвечает на запрос$`, func() error {
		for _, targetID := range s.Env.TargetIDs {
			if _, err := s.Env.CheckLastMessage(targetID, s.Prefix); err != nil {
				return err
			}
		}
		return nil
	})
}
