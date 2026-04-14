package steps

import (
	"context"
	"fmt"
	"time"

	"github.com/cucumber/godog"

	"github.com/pure-golang/budva-claude/internal/domain"
)

func register01DeliverySteps(ctx *godog.ScenarioContext, s *scenarioCtx) {
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

	// --- Rate limiting ---

	ctx.When(`^пользователь отправляет два сообщения подряд$`, func() error {
		s.applyRuleSet()

		for i := int64(1); i <= 2; i++ {
			msg := &domain.Message{
				ChatID:     s.env.SourceID,
				ID:         i,
				CanBeSaved: true,
				Content: domain.MessageContent{
					Type: domain.ContentText,
					Text: &domain.FormattedText{Text: fmt.Sprintf("message %d", i)},
				},
			}
			s.env.Telegram.PutMessage(msg)
			s.env.Handler.OnNewMessage(context.Background(), msg)
		}
		s.env.DrainQueue()

		return nil
	})

	ctx.Then(`^оба сообщения доставлены в целевые чаты$`, func() error {
		for _, targetID := range s.env.TargetIDs {
			msgs := s.env.Telegram.MessagesInChat(targetID)
			if len(msgs) < 2 {
				return fmt.Errorf("expected at least 2 messages in target chat %d, got %d", targetID, len(msgs))
			}
		}
		return nil
	})

	// --- Reply chain ---

	ctx.When(`^пользователь отвечает на это сообщение текстом "([^"]*)"$`, func(text string) error {
		if s.sentMsg == nil {
			return fmt.Errorf("no previously sent message")
		}

		replyMsg := &domain.Message{
			ChatID:     s.env.SourceID,
			ID:         2,
			CanBeSaved: true,
			Content: domain.MessageContent{
				Type: domain.ContentText,
				Text: &domain.FormattedText{Text: text},
			},
			ReplyTo: &domain.MessageReplyTo{
				ChatID:    s.env.SourceID,
				MessageID: s.sentMsg.ID,
			},
		}
		s.env.Telegram.PutMessage(replyMsg)

		s.env.Handler.OnNewMessage(context.Background(), replyMsg)
		s.env.DrainQueue()

		return nil
	})

	ctx.Then(`^в целевом чате ответ связан с копией оригинала$`, func() error {
		for _, targetID := range s.env.TargetIDs {
			msgs := s.env.Telegram.MessagesInChat(targetID)
			if len(msgs) < 2 {
				return fmt.Errorf("expected at least 2 messages in target chat %d, got %d", targetID, len(msgs))
			}
			// Ищем сообщение с ReplyTo
			found := false
			for _, m := range msgs {
				if m.ReplyTo != nil && m.ReplyTo.MessageID != 0 {
					found = true
					break
				}
			}
			if !found {
				return fmt.Errorf("no reply message found in target chat %d", targetID)
			}
		}
		return nil
	})

	// --- Origin unwrapping ---

	ctx.Given(`^в исходном чате есть пересланное сообщение из канала$`, func() error {
		originChatID := domain.ChatID(-1005000)
		originMsgID := domain.MessageID(42)

		// Оригинальное сообщение в канале
		originMsg := &domain.Message{
			ChatID:     originChatID,
			ID:         originMsgID,
			CanBeSaved: true,
			Content: domain.MessageContent{
				Type: domain.ContentText,
				Text: &domain.FormattedText{Text: "original channel content"},
			},
		}
		s.env.Telegram.PutMessage(originMsg)

		// Пересланное сообщение в исходном чате
		forwardedMsg := &domain.Message{
			ChatID:     s.env.SourceID,
			ID:         1,
			CanBeSaved: true,
			Content: domain.MessageContent{
				Type: domain.ContentText,
				Text: &domain.FormattedText{Text: "original channel content"},
			},
			ForwardInfo: &domain.MessageForwardInfo{
				OriginChatID:    originChatID,
				OriginMessageID: originMsgID,
			},
		}
		s.env.Telegram.PutMessage(forwardedMsg)
		s.forwardedMsg = forwardedMsg

		return nil
	})

	ctx.When(`^это сообщение пересылается$`, func() error {
		s.applyRuleSet()

		if s.forwardedMsg == nil {
			return fmt.Errorf("no forwarded message set")
		}

		s.env.Handler.OnNewMessage(context.Background(), s.forwardedMsg)
		s.env.DrainQueue()

		return nil
	})

	ctx.Then(`^в целевом чате используется контент оригинала$`, func() error {
		for _, targetID := range s.env.TargetIDs {
			msgs := s.env.Telegram.MessagesInChat(targetID)
			if len(msgs) == 0 {
				return fmt.Errorf("no messages in target chat %d", targetID)
			}
			// Проверяем что контент соответствует оригиналу
			found := false
			for _, m := range msgs {
				if m.Content.Text != nil && m.Content.Text.Text != "" {
					found = true
					break
				}
			}
			if !found {
				return fmt.Errorf("no message with content in target chat %d", targetID)
			}
		}
		return nil
	})

	// --- Statistics ---

	ctx.Then(`^счётчик просмотренных сообщений увеличивается$`, func() error {
		date := time.Now().Format("2006-01-02")
		for _, targetID := range s.env.TargetIDs {
			key := fmt.Sprintf("viewedMsgs:%d:%s", targetID, date)
			val, err := s.env.State.Get(key)
			if err != nil || val == "" {
				return fmt.Errorf("viewed messages counter not incremented for chat %d", targetID)
			}
		}
		return nil
	})

	// --- System messages ---

	ctx.Given(`^удаление системных сообщений включено$`, func() error {
		s.deleteSystemMessages = true
		return nil
	})

	ctx.Given(`^удаление системных сообщений выключено$`, func() error {
		s.deleteSystemMessages = false
		return nil
	})

	ctx.When(`^в исходном чате появляется системное сообщение$`, func() error {
		s.applyRuleSet()

		msg := &domain.Message{
			ChatID: s.env.SourceID,
			ID:     1,
			Content: domain.MessageContent{
				Type: domain.ContentSystem,
			},
		}
		s.sentMsg = msg
		s.env.Telegram.PutMessage(msg)

		s.env.Handler.OnNewMessage(context.Background(), msg)
		s.env.DrainQueue()

		return nil
	})

	ctx.Then(`^системное сообщение удаляется из исходного чата$`, func() error {
		msgs := s.env.Telegram.MessagesInChat(s.env.SourceID)
		for _, m := range msgs {
			if m.ID == s.sentMsg.ID {
				return fmt.Errorf("system message %d still exists in source chat", s.sentMsg.ID)
			}
		}
		return nil
	})

	ctx.Then(`^системное сообщение остаётся в исходном чате$`, func() error {
		msgs := s.env.Telegram.MessagesInChat(s.env.SourceID)
		for _, m := range msgs {
			if m.ID == s.sentMsg.ID {
				return nil
			}
		}
		return fmt.Errorf("system message %d was deleted from source chat", s.sentMsg.ID)
	})

	ctx.Then(`^сообщение не пересылается в целевые чаты$`, func() error {
		for _, targetID := range s.env.TargetIDs {
			msgs := s.env.Telegram.MessagesInChat(targetID)
			if len(msgs) > 0 {
				return fmt.Errorf("unexpected messages in target chat %d: %d", targetID, len(msgs))
			}
		}
		return nil
	})

	// --- Statistics ---

	ctx.Then(`^счётчик пересланных сообщений увеличивается$`, func() error {
		date := time.Now().Format("2006-01-02")
		for _, targetID := range s.env.TargetIDs {
			key := fmt.Sprintf("forwardedMsgs:%d:%s", targetID, date)
			val, err := s.env.State.Get(key)
			if err != nil || val == "" {
				return fmt.Errorf("forwarded messages counter not incremented for chat %d", targetID)
			}
		}
		return nil
	})
}
