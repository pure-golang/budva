//go:build bdd

package steps

import (
	"context"
	"time"

	"github.com/cucumber/godog"
	"github.com/zelenin/go-tdlib/client"
)

func register06AutoSteps(ctx *godog.ScenarioContext, s *scenarioCtx) {
	ctx.When(`^в исходном чате появляется сообщение с callback-запросом$`, func() error {
		s.sendCopy = true
		s.applyRuleSet()

		msg, err := s.env.PutMessage(s.env.SourceID, textContent("message with button"), s.prefix)
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
		s.sentMsg = msg

		s.env.Handler.OnNewMessage(context.Background(), msg)
		s.env.DrainQueue()
		time.Sleep(500 * time.Millisecond)

		return nil
	})

	ctx.Then(`^бот автоматически отвечает на запрос$`, func() error {
		for _, targetID := range s.env.TargetIDs {
			if _, err := s.env.CheckLastMessage(targetID, s.prefix); err != nil {
				return err
			}
		}
		return nil
	})
}
