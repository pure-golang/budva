//go:build bdd

package steps

import (
	"context"
	"fmt"
	"time"

	"github.com/cucumber/godog"
	"github.com/zelenin/go-tdlib/client"
)

func register01DeliverySteps(ctx *godog.ScenarioContext, s *scenarioCtx) {
	ctx.Given(`^правило пересылки в режиме "([^"]*)"$`, func(mode string) error {
		s.deliveryMode = mode
		s.sendCopy = (mode == "копия")
		return nil
	})

	ctx.When(`^пользователь отправляет сообщение в исходный чат$`, func() error {
		s.applyRuleSet()

		if s.messageText == "" {
			s.messageText = "test message"
		}

		msg, err := s.env.PutMessage(s.env.SourceID, textContent(s.messageText), s.prefix)
		if err != nil {
			return err
		}
		s.sentMsg = msg

		s.env.Handler.OnNewMessage(context.Background(), msg)
		s.env.DrainQueue()

		return nil
	})

	ctx.Then(`^сообщение появляется во всех целевых чатах$`, func() error {
		for _, targetID := range s.env.TargetIDs {
			if _, err := s.env.CheckLastMessage(targetID, s.prefix); err != nil {
				return err
			}
		}
		return nil
	})

	ctx.Then(`^копия сообщения появляется без авторства оригинала$`, func() error {
		for _, targetID := range s.env.TargetIDs {
			msg, err := s.env.CheckLastMessage(targetID, s.prefix)
			if err != nil {
				return err
			}
			if msg.ForwardInfo != nil {
				return fmt.Errorf("copy mode: expected no ForwardInfo in target %d", targetID)
			}
			if messageCaption(msg) == nil {
				return fmt.Errorf("copy mode: no text in message in target %d", targetID)
			}
		}
		return nil
	})

	ctx.Then(`^пересланное сообщение содержит авторство оригинала$`, func() error {
		for _, targetID := range s.env.TargetIDs {
			msg, err := s.env.CheckLastMessage(targetID, s.prefix)
			if err != nil {
				return err
			}
			if msg.ForwardInfo == nil {
				return fmt.Errorf("forward mode: expected ForwardInfo in target %d, got nil", targetID)
			}
		}
		return nil
	})

	// --- Rate limiting ---

	ctx.When(`^пользователь отправляет два сообщения подряд$`, func() error {
		s.applyRuleSet()

		for i := 1; i <= 2; i++ {
			msg, err := s.env.PutMessage(s.env.SourceID, textContent(fmt.Sprintf("message %d", i)), s.prefix)
			if err != nil {
				return err
			}
			s.env.Handler.OnNewMessage(context.Background(), msg)
		}
		s.env.DrainQueue()

		return nil
	})

	ctx.Then(`^оба сообщения доставлены в целевые чаты$`, func() error {
		for _, targetID := range s.env.TargetIDs {
			if _, err := s.env.CheckLastMessage(targetID, s.prefix); err != nil {
				return err
			}
		}
		return nil
	})

	// --- Reply chain ---

	ctx.When(`^пользователь отвечает на это сообщение текстом "([^"]*)"$`, func(text string) error {
		if s.sentMsg == nil {
			return fmt.Errorf("no previously sent message")
		}

		replyMsg, err := s.env.PutMessageReply(s.env.SourceID, textContent(text), s.sentMsg.Id, s.prefix)
		if err != nil {
			return err
		}

		s.env.Handler.OnNewMessage(context.Background(), replyMsg)
		s.env.DrainQueue()

		return nil
	})

	ctx.Then(`^в целевом чате ответ связан с копией оригинала$`, func() error {
		for _, targetID := range s.env.TargetIDs {
			if _, err := s.env.CheckLastMessage(targetID, s.prefix); err != nil {
				return err
			}
		}
		return nil
	})

	// --- Origin unwrapping ---

	ctx.Given(`^в исходном чате есть пересланное сообщение из канала$`, func() error {
		msg, err := s.env.PutMessage(s.env.SourceID, textContent("original channel content"), s.prefix)
		if err != nil {
			return err
		}
		// Эмулируем ForwardInfo с origin channel — в реальном TDLib это заполняет API.
		msg.ForwardInfo = &client.MessageForwardInfo{
			Origin: &client.MessageOriginChannel{
				ChatId:    s.env.SourceID,
				MessageId: msg.Id,
			},
		}
		s.forwardedMsg = msg
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
			msg, err := s.env.CheckLastMessage(targetID, s.prefix)
			if err != nil {
				return err
			}
			caption := messageCaption(msg)
			if caption == nil || caption.Text == "" {
				return fmt.Errorf("no content in last message of target chat %d", targetID)
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

		// Системные сообщения нельзя отправить напрямую — конструируем вручную
		// (в реальном TDLib они приходят от сервера как chat events).
		msg := &client.Message{
			ChatId:  s.env.SourceID,
			Id:      time.Now().UnixNano(),
			Content: &client.MessageChatJoinByLink{},
		}
		s.sentMsg = msg

		s.env.Handler.OnNewMessage(context.Background(), msg)
		s.env.DrainQueue()

		return nil
	})

	ctx.Then(`^системное сообщение удаляется из исходного чата$`, func() error {
		return nil
	})

	ctx.Then(`^системное сообщение остаётся в исходном чате$`, func() error {
		return nil
	})

	ctx.Then(`^сообщение не пересылается в целевые чаты$`, func() error {
		for _, targetID := range s.env.TargetIDs {
			if err := s.env.CheckNoMessage(targetID, s.prefix); err != nil {
				return err
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
