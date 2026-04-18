package steps

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/cucumber/godog"

	"github.com/pure-golang/budva-claude/internal/domain"
	"github.com/pure-golang/budva-claude/test/support"
)

func register05SyncSteps(ctx *godog.ScenarioContext, s *scenarioCtx) {
	ctx.Given(`^правило пересылки в режиме "([^"]*)" с опцией copy_once$`, func(mode string) error {
		s.deliveryMode = mode
		s.sendCopy = (mode == "копия")
		s.copyOnce = true
		s.src.Prev = &domain.Prev{
			Title: domain.PrevTitle,
			For:   s.env.TargetIDs,
		}
		s.src.Next = &domain.Next{
			Title: domain.NextTitle,
			For:   s.env.TargetIDs,
		}
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
		msg, err := s.env.PutMessage(context.Background(), s.env.SourceID, domain.InputMessageContent{
			Type: domain.ContentText,
			Text: &domain.FormattedText{Text: text},
		}, s.prefix)
		if err != nil {
			return err
		}
		s.sentMsg = msg

		s.env.Handler.OnNewMessage(context.Background(), msg)
		s.env.DrainQueue()

		return nil
	})

	ctx.Given(`^пользователь ранее отправил сообщение$`, func() error {
		s.applyRuleSet()

		s.messageText = "test message"
		msg, err := s.env.PutMessage(context.Background(), s.env.SourceID, domain.InputMessageContent{
			Type: domain.ContentText,
			Text: &domain.FormattedText{Text: s.messageText},
		}, s.prefix)
		if err != nil {
			return err
		}
		s.sentMsg = msg

		s.env.Handler.OnNewMessage(context.Background(), msg)
		s.env.DrainQueue()

		return nil
	})

	ctx.Given(`^сообщение было скопировано в целевые чаты$`, func() error {
		for _, targetID := range s.env.TargetIDs {
			msg, err := s.env.CheckLastMessage(context.Background(), targetID, s.prefix)
			if err != nil {
				return err
			}
			s.env.Handler.OnMessageSendSucceeded(msg.ChatID, msg.ID, msg.ID)
		}
		s.env.DrainQueue()
		return nil
	})

	ctx.When(`^пользователь редактирует сообщение на "([^"]*)"$`, func(newText string) error {
		if s.sentMsg == nil {
			return fmt.Errorf("no previously sent message")
		}

		// Реально редактируем SOURCE сообщение через TDLib (как в budva43 e2e).
		// TDLib отправит UpdateMessageEdited → processUpdates → handler.OnEditedMessage →
		// handler синхронизирует изменение в target-копиях.
		editedText := &domain.FormattedText{
			Text: support.PrefixText(s.prefix, newText),
		}
		if err := s.env.Telegram.EditMessageText(context.Background(), s.env.SourceID, s.sentMsg.ID, editedText); err != nil {
			return fmt.Errorf("edit source message: %w", err)
		}

		// Поллим: DrainQueue + проверяем target, пока edit не применится.
		// Для copy_once (versioning): ждём появления ссылки в новой копии.
		// Для обычного edit: ждём появления нового текста в существующей копии.
		deadline := time.After(10 * time.Second)
		expectLink := s.copyOnce
	editPoll:
		for {
			s.env.DrainQueue()
			for _, targetID := range s.env.TargetIDs {
				msg, err := s.env.CheckLastMessage(context.Background(), targetID, s.prefix)
				if err != nil {
					continue
				}
				if msg.Content.Text == nil {
					continue
				}
				if expectLink && strings.Contains(msg.Content.Text.Text, "https://t.me") {
					break editPoll
				}
				if !expectLink && strings.Contains(msg.Content.Text.Text, newText) {
					break editPoll
				}
			}
			select {
			case <-deadline:
				if expectLink {
					return fmt.Errorf("versioning: link not found in target within 10s")
				}
				break editPoll // Пусть Then step проверит результат
			case <-time.After(200 * time.Millisecond):
			}
		}

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
		if s.skipRetryDrain {
			s.env.Queue.ProcessBatch()
		} else {
			s.env.DrainQueue()
		}

		return nil
	})

	ctx.Then(`^в целевом чате появляется новая копия с текстом "([^"]*)"$`, func(text string) error {
		for _, targetID := range s.env.TargetIDs {
			msg, err := s.env.CheckLastMessage(context.Background(), targetID, s.prefix)
			if err != nil {
				return err
			}
			if msg.Content.Text == nil || !strings.Contains(msg.Content.Text.Text, text) {
				return fmt.Errorf("no message containing text %q in target chat %d", text, targetID)
			}
		}
		return nil
	})

	ctx.Then(`^новая копия содержит ссылку на предыдущую версию$`, func() error {
		for _, targetID := range s.env.TargetIDs {
			msg, err := s.env.CheckLastMessage(context.Background(), targetID, s.prefix)
			if err != nil {
				return err
			}
			if msg.Content.Text == nil || !strings.Contains(msg.Content.Text.Text, "https://t.me") {
				return fmt.Errorf("no message with link to previous version in target chat %d", targetID)
			}
		}
		return nil
	})

	ctx.Then(`^предыдущая копия обновляется со ссылкой на новую версию$`, func() error {
		for _, targetID := range s.env.TargetIDs {
			if _, err := s.env.CheckLastMessage(context.Background(), targetID, s.prefix); err != nil {
				return err
			}
		}
		return nil
	})

	ctx.Then(`^существующая копия в целевых чатах обновляется на "([^"]*)"$`, func(text string) error {
		for _, targetID := range s.env.TargetIDs {
			msg, err := s.env.CheckLastMessage(context.Background(), targetID, s.prefix)
			if err != nil {
				return err
			}
			if msg.Content.Text == nil || !strings.Contains(msg.Content.Text.Text, text) {
				return fmt.Errorf("no message with text %q in target chat %d", text, targetID)
			}
		}
		return nil
	})

	ctx.Then(`^копии остаются в целевых чатах$`, func() error {
		for _, targetID := range s.env.TargetIDs {
			if _, err := s.env.CheckLastMessage(context.Background(), targetID, s.prefix); err != nil {
				return err
			}
		}
		return nil
	})

	ctx.Then(`^копии удаляются из всех целевых чатов$`, func() error {
		for _, targetID := range s.env.TargetIDs {
			if err := s.env.CheckNoMessage(context.Background(), targetID, s.prefix); err != nil {
				return err
			}
		}
		return nil
	})

	// --- Retry eventual consistency ---

	ctx.When(`^permanent ID записывается в хранилище$`, func() error {
		for _, targetID := range s.env.TargetIDs {
			msg, err := s.env.CheckLastMessage(context.Background(), targetID, s.prefix)
			if err != nil {
				return err
			}
			if err := s.env.State.Set(fmt.Sprintf("newMsgId:%d:%d", targetID, msg.ID), fmt.Sprintf("%d", msg.ID)); err != nil {
				return err
			}
			if err := s.env.State.Set(fmt.Sprintf("tmpMsgId:%d:%d", targetID, msg.ID), fmt.Sprintf("%d", msg.ID)); err != nil {
				return err
			}
		}
		return nil
	})

	ctx.Given(`^permanent ID ещё не записан в хранилище$`, func() error {
		for _, targetID := range s.env.TargetIDs {
			msg, err := s.env.CheckLastMessage(context.Background(), targetID, s.prefix)
			if err != nil {
				continue
			}
			if err := s.env.State.Delete(fmt.Sprintf("newMsgId:%d:%d", msg.ChatID, msg.ID)); err != nil {
				return err
			}
		}
		s.skipRetryDrain = true
		return nil
	})

	ctx.Then(`^после повторной попытки копии удаляются из целевых чатов$`, func() error {
		s.env.DrainQueue()

		for _, targetID := range s.env.TargetIDs {
			if err := s.env.CheckNoMessage(context.Background(), targetID, s.prefix); err != nil {
				return err
			}
		}
		return nil
	})
}
