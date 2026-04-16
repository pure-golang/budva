package steps

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/cucumber/godog"

	"github.com/pure-golang/budva-claude/internal/domain"
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
		})
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
		})
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
			msgs, err := s.env.MessagesInChat(context.Background(), targetID)
			if err != nil {
				return err
			}
			if len(msgs) == 0 {
				return fmt.Errorf("message was not copied to target chat %d", targetID)
			}
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

		// Запоминаем сообщения до редактирования
		beforeMsgs := make(map[domain.ChatID]map[domain.MessageID]bool)
		for _, targetID := range s.env.TargetIDs {
			beforeMsgs[targetID] = make(map[domain.MessageID]bool)
			msgs, _ := s.env.MessagesInChat(context.Background(), targetID) //nolint:errcheck // Best-effort в вспомогательной логике шага
			for _, m := range msgs {
				beforeMsgs[targetID][m.ID] = true
			}
		}

		editedMsg := &domain.Message{
			ChatID: s.env.SourceID,
			ID:     s.sentMsg.ID,
			Content: domain.MessageContent{
				Type: domain.ContentText,
				Text: &domain.FormattedText{Text: newText},
			},
		}

		s.env.Handler.OnEditedMessage(context.Background(), editedMsg)
		s.env.DrainQueue()

		// Симулируем OnMessageSendSucceeded для новых сообщений (версионирование)
		for _, targetID := range s.env.TargetIDs {
			msgs, _ := s.env.MessagesInChat(context.Background(), targetID) //nolint:errcheck // Best-effort в вспомогательной логике шага
			for _, m := range msgs {
				if !beforeMsgs[targetID][m.ID] {
					s.env.Handler.OnMessageSendSucceeded(m.ChatID, m.ID, m.ID)
				}
			}
		}
		s.env.DrainQueue()

		// Ожидаем завершения runNextLinkWorkflow горутины
		if s.copyOnce {
			deadline := time.After(5 * time.Second)
			for {
				select {
				case <-deadline:
					return fmt.Errorf("runNextLinkWorkflow did not complete within 5s")
				case <-time.After(200 * time.Millisecond):
					updated := false
					for _, targetID := range s.env.TargetIDs {
						msgs, _ := s.env.MessagesInChat(context.Background(), targetID) //nolint:errcheck // Best-effort в вспомогательной логике шага
						for _, m := range msgs {
							if m.Content.Text != nil && strings.Contains(m.Content.Text.Text, "https://t.me") {
								updated = true
								break
							}
						}
						if updated {
							break
						}
					}
					if updated {
						goto done
					}
				}
			}
		done:
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
			msgs, err := s.env.MessagesInChat(context.Background(), targetID)
			if err != nil {
				return err
			}
			found := false
			for _, m := range msgs {
				if m.Content.Text != nil && strings.Contains(m.Content.Text.Text, text) {
					found = true
					break
				}
			}
			if !found {
				return fmt.Errorf("no message containing text %q in target chat %d", text, targetID)
			}
		}
		return nil
	})

	ctx.Then(`^новая копия содержит ссылку на предыдущую версию$`, func() error {
		for _, targetID := range s.env.TargetIDs {
			msgs, err := s.env.MessagesInChat(context.Background(), targetID)
			if err != nil {
				return err
			}
			if len(msgs) == 0 {
				return fmt.Errorf("no messages in target chat %d", targetID)
			}
			found := false
			for _, m := range msgs {
				if m.Content.Text != nil && strings.Contains(m.Content.Text.Text, "https://t.me") {
					found = true
					break
				}
			}
			if !found {
				return fmt.Errorf("no message with link to previous version in target chat %d", targetID)
			}
		}
		return nil
	})

	ctx.Then(`^предыдущая копия обновляется со ссылкой на новую версию$`, func() error {
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

	ctx.Then(`^существующая копия в целевых чатах обновляется на "([^"]*)"$`, func(text string) error {
		for _, targetID := range s.env.TargetIDs {
			found, err := s.env.HasMessageWithText(context.Background(), targetID, text)
			if err != nil {
				return err
			}
			if !found {
				return fmt.Errorf("no message with text %q in target chat %d", text, targetID)
			}
		}
		return nil
	})

	ctx.Then(`^копии остаются в целевых чатах$`, func() error {
		for _, targetID := range s.env.TargetIDs {
			msgs, err := s.env.MessagesInChat(context.Background(), targetID)
			if err != nil {
				return err
			}
			if len(msgs) == 0 {
				return fmt.Errorf("copies should remain in target chat %d", targetID)
			}
		}
		return nil
	})

	ctx.Then(`^копии удаляются из всех целевых чатов$`, func() error {
		for _, targetID := range s.env.TargetIDs {
			msgs, err := s.env.MessagesInChat(context.Background(), targetID)
			if err != nil {
				return err
			}
			if len(msgs) > 0 {
				return fmt.Errorf("copies should be deleted from target chat %d, got %d", targetID, len(msgs))
			}
		}
		return nil
	})

	// --- Retry eventual consistency ---

	ctx.When(`^permanent ID записывается в хранилище$`, func() error {
		for _, targetID := range s.env.TargetIDs {
			msgs, _ := s.env.MessagesInChat(context.Background(), targetID) //nolint:errcheck // Best-effort в вспомогательной логике шага
			for _, m := range msgs {
				if err := s.env.State.Set(fmt.Sprintf("newMsgId:%d:%d", targetID, m.ID), fmt.Sprintf("%d", m.ID)); err != nil {
					return err
				}
				if err := s.env.State.Set(fmt.Sprintf("tmpMsgId:%d:%d", targetID, m.ID), fmt.Sprintf("%d", m.ID)); err != nil {
					return err
				}
			}
		}
		return nil
	})

	ctx.Given(`^permanent ID ещё не записан в хранилище$`, func() error {
		for _, targetID := range s.env.TargetIDs {
			msgs, _ := s.env.MessagesInChat(context.Background(), targetID) //nolint:errcheck // Best-effort в вспомогательной логике шага
			for _, m := range msgs {
				if err := s.env.State.Delete(fmt.Sprintf("newMsgId:%d:%d", m.ChatID, m.ID)); err != nil {
					return err
				}
			}
		}
		s.skipRetryDrain = true
		return nil
	})

	ctx.Then(`^после повторной попытки копии удаляются из целевых чатов$`, func() error {
		s.env.DrainQueue()

		for _, targetID := range s.env.TargetIDs {
			msgs, err := s.env.MessagesInChat(context.Background(), targetID)
			if err != nil {
				return err
			}
			if len(msgs) > 0 {
				return fmt.Errorf("copies should be deleted from target chat %d after retry, got %d", targetID, len(msgs))
			}
		}
		return nil
	})
}
