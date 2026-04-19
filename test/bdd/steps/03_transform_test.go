//go:build bdd

package steps

import (
	"context"
	"fmt"
	"strings"

	"github.com/cucumber/godog"
	"github.com/zelenin/go-tdlib/client"

	"github.com/pure-golang/budva-claude/internal/domain"
)

func register03TransformSteps(ctx *godog.ScenarioContext, s *scenarioCtx) {
	ctx.Given(`^правило замены фрагментов "([^"]*)" → "([^"]*)" и "([^"]*)" → "([^"]*)"$`, func(from1, to1, from2, to2 string) error {
		s.replaceFrom = []string{from1, from2}
		s.replaceTo = []string{to1, to2}
		return nil
	})

	ctx.Given(`^для источника включена опция "([^"]*)"$`, func(option string) error {
		switch option {
		case "подпись":
			s.src.Sign = &domain.Sign{
				Title: domain.SignTitle,
				For:   s.env.TargetIDs,
			}
		case "ссылка":
			s.src.Link = &domain.Link{
				Title: domain.LinkTitle,
				For:   s.env.TargetIDs,
			}
		case "перевод":
			s.src.Translate = &domain.Translate{
				Lang: "ru",
				For:  s.env.TargetIDs,
			}
		case "автоответы":
			s.src.AutoAnswer = true
		}
		return nil
	})

	ctx.Given(`^в исходном чате есть ранее скопированное сообщение$`, func() error {
		_, err := s.env.PutMessage(s.env.SourceID, textContent("previous message"), s.prefix)
		return err
	})

	ctx.When(`^пользователь отправляет сообщение со ссылкой на предыдущее сообщение$`, func() error {
		s.applyRuleSet()

		// Получаем реальный permalink на предыдущее сообщение в source.
		prevMsgs, err := s.env.Telegram.GetChatHistory(&client.GetChatHistoryRequest{
			ChatId: s.env.SourceID,
			Limit:  1,
		})
		if err != nil || prevMsgs == nil || len(prevMsgs.Messages) == 0 {
			return fmt.Errorf("no previous message in source chat")
		}
		link, err := s.env.Telegram.GetMessageLink(&client.GetMessageLinkRequest{
			ChatId:    s.env.SourceID,
			MessageId: prevMsgs.Messages[0].Id,
		})
		if err != nil {
			return fmt.Errorf("get message link: %w", err)
		}

		msg, err := s.env.PutMessage(s.env.SourceID, textContent(fmt.Sprintf("see %s", link.Link)), s.prefix)
		if err != nil {
			return err
		}
		s.sentMsg = msg

		s.env.Handler.OnNewMessage(context.Background(), msg)
		s.env.DrainQueue()

		return nil
	})

	ctx.Then(`^в целевом чате ссылка указывает на копию предыдущего сообщения$`, func() error {
		for _, targetID := range s.env.TargetIDs {
			msg, err := s.env.CheckLastMessage(targetID, s.prefix)
			if err != nil {
				return err
			}
			text := messageCaption(msg)
			if text == nil {
				return fmt.Errorf("no text in target %d", targetID)
			}
			if !hasTMeEntity(text) && !strings.Contains(text.Text, "https://t.me/") {
				return fmt.Errorf("expected link to copy in target %d, not found in %q", targetID, text.Text)
			}
		}
		return nil
	})

	ctx.Given(`^в исходном чате есть сообщение из внешнего чата$`, func() error {
		return nil
	})

	ctx.When(`^пользователь отправляет сообщение со ссылкой на внешнее сообщение$`, func() error {
		s.applyRuleSet()

		// Внешняя ссылка — ссылка на сообщение в target чате (не source).
		externalChat := s.env.TargetIDs[0]
		extMsgs, err := s.env.Telegram.GetChatHistory(&client.GetChatHistoryRequest{
			ChatId: externalChat,
			Limit:  1,
		})
		externalLink := "https://t.me/c/9999999/42" // Fallback.
		if err == nil && extMsgs != nil && len(extMsgs.Messages) > 0 {
			link, linkErr := s.env.Telegram.GetMessageLink(&client.GetMessageLinkRequest{
				ChatId:    externalChat,
				MessageId: extMsgs.Messages[0].Id,
			})
			if linkErr == nil {
				externalLink = link.Link
			}
		}

		msg, err := s.env.PutMessage(s.env.SourceID, textContent(fmt.Sprintf("see %s", externalLink)), s.prefix)
		if err != nil {
			return err
		}
		s.sentMsg = msg

		s.env.Handler.OnNewMessage(context.Background(), msg)
		s.env.DrainQueue()

		return nil
	})

	ctx.Then(`^в целевом чате сообщение появляется без внешней ссылки$`, func() error {
		for _, targetID := range s.env.TargetIDs {
			if _, err := s.env.CheckLastMessage(targetID, s.prefix); err != nil {
				return err
			}
		}
		return nil
	})

	ctx.Then(`^в целевом чате сообщение содержит подпись источника$`, func() error {
		for _, targetID := range s.env.TargetIDs {
			msg, err := s.env.CheckLastMessage(targetID, s.prefix)
			if err != nil {
				return err
			}
			text := messageCaption(msg)
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
		for _, targetID := range s.env.TargetIDs {
			msg, err := s.env.CheckLastMessage(targetID, s.prefix)
			if err != nil {
				return err
			}
			text := messageCaption(msg)
			if text == nil {
				return fmt.Errorf("no text in target %d", targetID)
			}
			if strings.Contains(text.Text, s.messageText) {
				return fmt.Errorf("expected translated text in target %d, but original %q still present in %q",
					targetID, s.messageText, text.Text)
			}
		}
		return nil
	})

	ctx.Then(`^в целевом чате сообщение содержит ссылку на оригинал$`, func() error {
		for _, targetID := range s.env.TargetIDs {
			msg, err := s.env.CheckLastMessage(targetID, s.prefix)
			if err != nil {
				return err
			}
			text := messageCaption(msg)
			if text == nil {
				return fmt.Errorf("no text in target %d", targetID)
			}
			if !hasTMeEntity(text) {
				return fmt.Errorf("expected TextURL entity with t.me permalink in target %d, not found; text=%q",
					targetID, text.Text)
			}
		}
		return nil
	})
}
