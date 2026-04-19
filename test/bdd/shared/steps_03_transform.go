package shared

import (
	"context"
	"fmt"
	"strings"

	"github.com/cucumber/godog"
	"github.com/zelenin/go-tdlib/client"

	"github.com/pure-golang/budva-claude/internal/domain"
)

// RegisterTransformSteps регистрирует шаги эпика 03_transform.
func RegisterTransformSteps(ctx *godog.ScenarioContext, s *ScenarioCtx) {
	ctx.Given(`^правило замены фрагментов "([^"]*)" → "([^"]*)" и "([^"]*)" → "([^"]*)"$`, func(from1, to1, from2, to2 string) error {
		s.ReplaceFrom = []string{from1, from2}
		s.ReplaceTo = []string{to1, to2}
		return nil
	})

	ctx.Given(`^для источника включена опция "([^"]*)"$`, func(option string) error {
		switch option {
		case "подпись":
			s.Src.Sign = &domain.Sign{
				Title: domain.SignTitle,
				For:   s.Env.TargetIDs,
			}
		case "ссылка":
			s.Src.Link = &domain.Link{
				Title: domain.LinkTitle,
				For:   s.Env.TargetIDs,
			}
		case "перевод":
			s.Src.Translate = &domain.Translate{
				Lang: "ru",
				For:  s.Env.TargetIDs,
			}
		case "автоответы":
			s.Src.AutoAnswer = true
		}
		return nil
	})

	ctx.Given(`^в исходном чате есть ранее скопированное сообщение$`, func() error {
		_, err := s.Env.PutMessage(s.Env.SourceID, TextContent("previous message"), s.Prefix)
		return err
	})

	ctx.When(`^пользователь отправляет сообщение со ссылкой на предыдущее сообщение$`, func() error {
		s.ApplyRuleSet()

		// Получаем реальный permalink на предыдущее сообщение в source.
		prevMsgs, err := s.Env.Telegram.GetChatHistory(&client.GetChatHistoryRequest{
			ChatId: s.Env.SourceID,
			Limit:  1,
		})
		if err != nil || prevMsgs == nil || len(prevMsgs.Messages) == 0 {
			return fmt.Errorf("no previous message in source chat")
		}
		link, err := s.Env.Telegram.GetMessageLink(&client.GetMessageLinkRequest{
			ChatId:    s.Env.SourceID,
			MessageId: prevMsgs.Messages[0].Id,
		})
		if err != nil {
			return fmt.Errorf("get message link: %w", err)
		}

		msg, err := s.Env.PutMessage(s.Env.SourceID, TextContent(fmt.Sprintf("see %s", link.Link)), s.Prefix)
		if err != nil {
			return err
		}
		s.SentMsg = msg

		s.Env.Handler.OnNewMessage(context.Background(), msg)
		s.Env.DrainQueue()

		return nil
	})

	ctx.Then(`^в целевом чате ссылка указывает на копию предыдущего сообщения$`, func() error {
		for _, targetID := range s.Env.TargetIDs {
			msg, err := s.Env.CheckLastMessage(targetID, s.Prefix)
			if err != nil {
				return err
			}
			text := MessageCaption(msg)
			if text == nil {
				return fmt.Errorf("no text in target %d", targetID)
			}
			if !HasTMeEntity(text) && !strings.Contains(text.Text, "https://t.me/") {
				return fmt.Errorf("expected link to copy in target %d, not found in %q", targetID, text.Text)
			}
		}
		return nil
	})

	ctx.Given(`^в исходном чате есть сообщение из внешнего чата$`, func() error {
		return nil
	})

	ctx.When(`^пользователь отправляет сообщение со ссылкой на внешнее сообщение$`, func() error {
		s.ApplyRuleSet()

		// Внешняя ссылка — ссылка на сообщение в target чате (не source).
		externalChat := s.Env.TargetIDs[0]
		extMsgs, err := s.Env.Telegram.GetChatHistory(&client.GetChatHistoryRequest{
			ChatId: externalChat,
			Limit:  1,
		})
		externalLink := "https://t.me/c/9999999/42" // Fallback.
		if err == nil && extMsgs != nil && len(extMsgs.Messages) > 0 {
			link, linkErr := s.Env.Telegram.GetMessageLink(&client.GetMessageLinkRequest{
				ChatId:    externalChat,
				MessageId: extMsgs.Messages[0].Id,
			})
			if linkErr == nil {
				externalLink = link.Link
			}
		}

		msg, err := s.Env.PutMessage(s.Env.SourceID, TextContent(fmt.Sprintf("see %s", externalLink)), s.Prefix)
		if err != nil {
			return err
		}
		s.SentMsg = msg

		s.Env.Handler.OnNewMessage(context.Background(), msg)
		s.Env.DrainQueue()

		return nil
	})

	ctx.Then(`^в целевом чате сообщение появляется без внешней ссылки$`, func() error {
		for _, targetID := range s.Env.TargetIDs {
			if _, err := s.Env.CheckLastMessage(targetID, s.Prefix); err != nil {
				return err
			}
		}
		return nil
	})

	ctx.Then(`^в целевом чате сообщение содержит подпись источника$`, func() error {
		for _, targetID := range s.Env.TargetIDs {
			msg, err := s.Env.CheckLastMessage(targetID, s.Prefix)
			if err != nil {
				return err
			}
			text := MessageCaption(msg)
			if text == nil {
				return fmt.Errorf("no text in target %d", targetID)
			}
			if !strings.Contains(text.Text, domain.SignTitle) {
				return fmt.Errorf("expected sign title %q in target %d, got %q",
					domain.SignTitle, targetID, text.Text)
			}
			hasBoldEntity := false
			for _, e := range text.Entities {
				if _, ok := e.Type.(*client.TextEntityTypeBold); ok {
					hasBoldEntity = true
					break
				}
			}
			if !hasBoldEntity {
				return fmt.Errorf("expected bold entity for sign in target %d, no bold entities found", targetID)
			}
		}
		return nil
	})

	ctx.Then(`^в целевом чате сообщение содержит переведённый текст$`, func() error {
		for _, targetID := range s.Env.TargetIDs {
			msg, err := s.Env.CheckLastMessage(targetID, s.Prefix)
			if err != nil {
				return err
			}
			text := MessageCaption(msg)
			if text == nil {
				return fmt.Errorf("no text in target %d", targetID)
			}
			if strings.Contains(text.Text, s.MessageText) {
				return fmt.Errorf("expected translated text in target %d, but original %q still present in %q",
					targetID, s.MessageText, text.Text)
			}
		}
		return nil
	})

	ctx.Then(`^в целевом чате сообщение содержит ссылку на оригинал$`, func() error {
		for _, targetID := range s.Env.TargetIDs {
			msg, err := s.Env.CheckLastMessage(targetID, s.Prefix)
			if err != nil {
				return err
			}
			text := MessageCaption(msg)
			if text == nil {
				return fmt.Errorf("no text in target %d", targetID)
			}
			if !HasTMeEntity(text) {
				return fmt.Errorf("expected TextURL entity with t.me permalink in target %d, not found; text=%q",
					targetID, text.Text)
			}
		}
		return nil
	})
}
