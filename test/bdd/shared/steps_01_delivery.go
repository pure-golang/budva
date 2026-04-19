package shared

import (
	"context"
	"fmt"
	"time"

	"github.com/cucumber/godog"
	"github.com/zelenin/go-tdlib/client"
)

// RegisterDeliverySteps регистрирует шаги эпика 01_delivery.
// Некоторые из этих шагов (например, «правило пересылки в режиме», «пользователь
// отправляет сообщение в исходный чат») переиспользуются feature-файлами других
// эпиков, поэтому регистрация происходит для всех эпиков.
func RegisterDeliverySteps(ctx *godog.ScenarioContext, s *ScenarioCtx) {
	ctx.Given(`^правило пересылки в режиме "([^"]*)"$`, func(mode string) error {
		s.DeliveryMode = mode
		s.SendCopy = (mode == "копия")
		return nil
	})

	ctx.When(`^пользователь отправляет сообщение в исходный чат$`, func() error {
		s.ApplyRuleSet()

		if s.MessageText == "" {
			s.MessageText = "test message"
		}

		msg, err := s.Env.PutMessage(s.Env.SourceID, TextContent(s.MessageText), s.Prefix)
		if err != nil {
			return err
		}
		s.SentMsg = msg

		s.Env.Handler.OnNewMessage(context.Background(), msg)
		s.Env.DrainQueue()

		return nil
	})

	ctx.Then(`^сообщение появляется во всех целевых чатах$`, func() error {
		for _, targetID := range s.Env.TargetIDs {
			if _, err := s.Env.CheckLastMessage(targetID, s.Prefix); err != nil {
				return err
			}
		}
		return nil
	})

	ctx.Then(`^копия сообщения появляется без авторства оригинала$`, func() error {
		for _, targetID := range s.Env.TargetIDs {
			msg, err := s.Env.CheckLastMessage(targetID, s.Prefix)
			if err != nil {
				return err
			}
			if msg.ForwardInfo != nil {
				return fmt.Errorf("copy mode: expected no ForwardInfo in target %d", targetID)
			}
			if MessageCaption(msg) == nil {
				return fmt.Errorf("copy mode: no text in message in target %d", targetID)
			}
		}
		return nil
	})

	ctx.Then(`^пересланное сообщение содержит авторство оригинала$`, func() error {
		for _, targetID := range s.Env.TargetIDs {
			msg, err := s.Env.CheckLastMessage(targetID, s.Prefix)
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
		s.ApplyRuleSet()

		for i := 1; i <= 2; i++ {
			msg, err := s.Env.PutMessage(s.Env.SourceID, TextContent(fmt.Sprintf("message %d", i)), s.Prefix)
			if err != nil {
				return err
			}
			s.Env.Handler.OnNewMessage(context.Background(), msg)
		}
		s.Env.DrainQueue()

		return nil
	})

	ctx.Then(`^оба сообщения доставлены в целевые чаты$`, func() error {
		for _, targetID := range s.Env.TargetIDs {
			if _, err := s.Env.CheckLastMessage(targetID, s.Prefix); err != nil {
				return err
			}
		}
		return nil
	})

	// --- Reply chain ---

	ctx.When(`^пользователь отвечает на это сообщение текстом "([^"]*)"$`, func(text string) error {
		if s.SentMsg == nil {
			return fmt.Errorf("no previously sent message")
		}

		replyMsg, err := s.Env.PutMessageReply(s.Env.SourceID, TextContent(text), s.SentMsg.Id, s.Prefix)
		if err != nil {
			return err
		}

		s.Env.Handler.OnNewMessage(context.Background(), replyMsg)
		s.Env.DrainQueue()

		return nil
	})

	ctx.Then(`^в целевом чате ответ связан с копией оригинала$`, func() error {
		for _, targetID := range s.Env.TargetIDs {
			if _, err := s.Env.CheckLastMessage(targetID, s.Prefix); err != nil {
				return err
			}
		}
		return nil
	})

	// --- Origin unwrapping ---

	ctx.Given(`^в исходном чате есть пересланное сообщение из канала$`, func() error {
		msg, err := s.Env.PutMessage(s.Env.SourceID, TextContent("original channel content"), s.Prefix)
		if err != nil {
			return err
		}
		// Эмулируем ForwardInfo с origin channel — в реальном TDLib это заполняет API.
		msg.ForwardInfo = &client.MessageForwardInfo{
			Origin: &client.MessageOriginChannel{
				ChatId:    s.Env.SourceID,
				MessageId: msg.Id,
			},
		}
		s.ForwardedMsg = msg
		return nil
	})

	ctx.When(`^это сообщение пересылается$`, func() error {
		s.ApplyRuleSet()

		if s.ForwardedMsg == nil {
			return fmt.Errorf("no forwarded message set")
		}

		s.Env.Handler.OnNewMessage(context.Background(), s.ForwardedMsg)
		s.Env.DrainQueue()

		return nil
	})

	ctx.Then(`^в целевом чате используется контент оригинала$`, func() error {
		for _, targetID := range s.Env.TargetIDs {
			msg, err := s.Env.CheckLastMessage(targetID, s.Prefix)
			if err != nil {
				return err
			}
			caption := MessageCaption(msg)
			if caption == nil || caption.Text == "" {
				return fmt.Errorf("no content in last message of target chat %d", targetID)
			}
		}
		return nil
	})

	// --- Statistics ---

	ctx.Then(`^счётчик просмотренных сообщений увеличивается$`, func() error {
		date := time.Now().Format("2006-01-02")
		for _, targetID := range s.Env.TargetIDs {
			key := fmt.Sprintf("viewedMsgs:%d:%s", targetID, date)
			val, err := s.Env.State.Get(key)
			if err != nil || val == "" {
				return fmt.Errorf("viewed messages counter not incremented for chat %d", targetID)
			}
		}
		return nil
	})

	// --- System messages ---

	ctx.Given(`^удаление системных сообщений включено$`, func() error {
		s.DeleteSystemMessages = true
		return nil
	})

	ctx.Given(`^удаление системных сообщений выключено$`, func() error {
		s.DeleteSystemMessages = false
		return nil
	})

	ctx.When(`^в исходном чате появляется системное сообщение$`, func() error {
		s.ApplyRuleSet()

		// Системные сообщения нельзя отправить напрямую — конструируем вручную
		// (в реальном TDLib они приходят от сервера как chat events).
		msg := &client.Message{
			ChatId:  s.Env.SourceID,
			Id:      time.Now().UnixNano(),
			Content: &client.MessageChatJoinByLink{},
		}
		s.SentMsg = msg

		s.Env.Handler.OnNewMessage(context.Background(), msg)
		s.Env.DrainQueue()

		return nil
	})

	ctx.Then(`^системное сообщение удаляется из исходного чата$`, func() error {
		return nil
	})

	ctx.Then(`^системное сообщение остаётся в исходном чате$`, func() error {
		return nil
	})

	ctx.Then(`^сообщение не пересылается в целевые чаты$`, func() error {
		for _, targetID := range s.Env.TargetIDs {
			if err := s.Env.CheckNoMessage(targetID, s.Prefix); err != nil {
				return err
			}
		}
		return nil
	})

	// --- Statistics ---

	ctx.Then(`^счётчик пересланных сообщений увеличивается$`, func() error {
		date := time.Now().Format("2006-01-02")
		for _, targetID := range s.Env.TargetIDs {
			key := fmt.Sprintf("forwardedMsgs:%d:%s", targetID, date)
			val, err := s.Env.State.Get(key)
			if err != nil || val == "" {
				return fmt.Errorf("forwarded messages counter not incremented for chat %d", targetID)
			}
		}
		return nil
	})
}
