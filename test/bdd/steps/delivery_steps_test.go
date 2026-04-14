package steps

import (
	"context"
	"fmt"

	"github.com/cucumber/godog"

	"github.com/pure-golang/budva-claude/internal/domain"
)

func registerDeliverySteps(ctx *godog.ScenarioContext, s *scenarioCtx) {
	ctx.Given(`^правило пересылки в режиме "([^"]*)"$`, func(mode string) error {
		s.deliveryMode = mode
		s.sendCopy = (mode == "копия")
		return nil
	})

	ctx.Given(`^исходный чат типа "([^"]*)"$`, func(srcType string) error {
		s.sourceType = srcType
		return nil
	})

	ctx.When(`^пользователь отправляет сообщение в исходный чат$`, func() error {
		s.applyRuleSet()

		if s.messageText == "" {
			s.messageText = "test message"
		}

		msg := &domain.Message{
			ChatID:     s.env.SourceID,
			ID:         1,
			CanBeSaved: true,
			Content: domain.MessageContent{
				Type: domain.ContentText,
				Text: &domain.FormattedText{Text: s.messageText},
			},
		}
		s.sentMsg = msg
		s.env.Telegram.PutMessage(msg)

		s.env.Handler.OnNewMessage(context.Background(), msg)
		s.env.DrainQueue()

		return nil
	})

	ctx.Then(`^сообщение появляется во всех целевых чатах$`, func() error {
		for _, targetID := range s.env.TargetIDs {
			msgs := s.env.Telegram.MessagesInChat(targetID)
			if len(msgs) == 0 {
				return fmt.Errorf("no messages in target chat %d", targetID)
			}
		}
		return nil
	})
}
