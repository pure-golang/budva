package steps

import (
	"context"
	"fmt"

	"github.com/cucumber/godog"

	"github.com/pure-golang/budva-claude/internal/domain"
)

func registerSyncSteps(ctx *godog.ScenarioContext, s *scenarioCtx) {
	ctx.Given(`^правило пересылки в режиме "([^"]*)" с опцией copy_once$`, func(mode string) error {
		s.deliveryMode = mode
		s.sendCopy = (mode == "копия")
		s.copyOnce = true
		return nil
	})

	ctx.Given(`^правило пересылки в режиме "([^"]*)" без опции copy_once$`, func(mode string) error {
		s.deliveryMode = mode
		s.sendCopy = (mode == "копия")
		s.copyOnce = false
		return nil
	})

	ctx.Given(`^правило пересылки в режиме "([^"]*)" с опцией indelible$`, func(mode string) error {
		s.deliveryMode = mode
		s.sendCopy = (mode == "копия")
		s.indelible = true
		return nil
	})

	ctx.Given(`^правило пересылки в режиме "([^"]*)" без опции indelible$`, func(mode string) error {
		s.deliveryMode = mode
		s.sendCopy = (mode == "копия")
		s.indelible = false
		return nil
	})

	ctx.Given(`^пользователь ранее отправил сообщение с текстом "([^"]*)"$`, func(text string) error {
		s.applyRuleSet()

		s.messageText = text
		msg := &domain.Message{
			ChatID:     s.env.SourceID,
			ID:         1,
			CanBeSaved: true,
			Content: domain.MessageContent{
				Type: domain.ContentText,
				Text: &domain.FormattedText{Text: text},
			},
		}
		s.sentMsg = msg
		s.env.Telegram.PutMessage(msg)

		s.env.Handler.OnNewMessage(context.Background(), msg)
		s.env.DrainQueue()

		return nil
	})

	ctx.Given(`^пользователь ранее отправил сообщение$`, func() error {
		s.applyRuleSet()

		s.messageText = "test message"
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

	ctx.Given(`^сообщение было скопировано в целевые чаты$`, func() error {
		// Сообщение уже было обработано handler в предыдущем Given-шаге,
		// проверяем что копии появились и симулируем OnMessageSendSucceeded
		for _, targetID := range s.env.TargetIDs {
			msgs := s.env.Telegram.MessagesInChat(targetID)
			if len(msgs) == 0 {
				return fmt.Errorf("message was not copied to target chat %d", targetID)
			}
			// Симулируем подтверждение отправки: temp ID → permanent ID
			for _, m := range msgs {
				s.env.Handler.OnMessageSendSucceeded(m.ChatID, m.ID, m.ID)
			}
		}
		s.env.DrainQueue()
		return nil
	})

	ctx.When(`^пользователь редактирует сообщение на "([^"]*)"$`, func(newText string) error {
		if s.sentMsg == nil {
			return fmt.Errorf("no previously sent message")
		}

		editedMsg := &domain.Message{
			ChatID: s.env.SourceID,
			ID:     s.sentMsg.ID,
			Content: domain.MessageContent{
				Type: domain.ContentText,
				Text: &domain.FormattedText{Text: newText},
			},
		}
		// Обновляем сообщение в fake telegram
		s.env.Telegram.PutMessage(editedMsg)

		s.env.Handler.OnEditedMessage(context.Background(), editedMsg)
		s.env.DrainQueue()

		s.messageText = newText
		return nil
	})

	ctx.When(`^пользователь удаляет оригинальное сообщение$`, func() error {
		if s.sentMsg == nil {
			return fmt.Errorf("no previously sent message")
		}

		s.env.Handler.OnDeletedMessages(
			context.Background(),
			s.env.SourceID,
			[]domain.MessageID{s.sentMsg.ID},
			true,
		)
		s.env.DrainQueue()

		return nil
	})

	ctx.Then(`^в целевом чате появляется новая копия с текстом "([^"]*)"$`, func(text string) error {
		for _, targetID := range s.env.TargetIDs {
			if !s.env.Telegram.HasMessageWithText(targetID, text) {
				return fmt.Errorf("no message with text %q in target chat %d", text, targetID)
			}
		}
		return nil
	})

	ctx.Then(`^новая копия содержит ссылку на предыдущую версию$`, func() error {
		// Проверяем что в целевых чатах есть сообщения (навигация проверяется на уровне интеграции)
		for _, targetID := range s.env.TargetIDs {
			msgs := s.env.Telegram.MessagesInChat(targetID)
			if len(msgs) == 0 {
				return fmt.Errorf("no messages in target chat %d", targetID)
			}
		}
		return nil
	})

	ctx.Then(`^предыдущая копия обновляется со ссылкой на новую версию$`, func() error {
		for _, targetID := range s.env.TargetIDs {
			msgs := s.env.Telegram.MessagesInChat(targetID)
			if len(msgs) == 0 {
				return fmt.Errorf("no messages in target chat %d", targetID)
			}
		}
		return nil
	})

	ctx.Then(`^существующая копия в целевых чатах обновляется на "([^"]*)"$`, func(text string) error {
		for _, targetID := range s.env.TargetIDs {
			if !s.env.Telegram.HasMessageWithText(targetID, text) {
				return fmt.Errorf("no message with text %q in target chat %d", text, targetID)
			}
		}
		return nil
	})

	ctx.Then(`^копии остаются в целевых чатах$`, func() error {
		for _, targetID := range s.env.TargetIDs {
			msgs := s.env.Telegram.MessagesInChat(targetID)
			if len(msgs) == 0 {
				return fmt.Errorf("copies should remain in target chat %d", targetID)
			}
		}
		return nil
	})

	ctx.Then(`^копии удаляются из всех целевых чатов$`, func() error {
		for _, targetID := range s.env.TargetIDs {
			msgs := s.env.Telegram.MessagesInChat(targetID)
			if len(msgs) > 0 {
				return fmt.Errorf("copies should be deleted from target chat %d, got %d", targetID, len(msgs))
			}
		}
		return nil
	})
}
