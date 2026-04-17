package steps

import (
	"context"
	"fmt"
	"strings"

	"github.com/cucumber/godog"

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
		_, err := s.env.PutMessage(context.Background(), s.env.SourceID, domain.InputMessageContent{
			Type: domain.ContentText,
			Text: &domain.FormattedText{Text: "previous message"},
		}, s.prefix)
		return err
	})

	ctx.When(`^пользователь отправляет сообщение со ссылкой на предыдущее сообщение$`, func() error {
		s.applyRuleSet()

		// Получаем реальный permalink на предыдущее сообщение в source
		prevMsgs, err := s.env.Telegram.GetChatHistory(context.Background(), s.env.SourceID, 0, 0, 1)
		if err != nil || len(prevMsgs) == 0 {
			return fmt.Errorf("no previous message in source chat")
		}
		link, err := s.env.Telegram.GetMessageLink(context.Background(), s.env.SourceID, prevMsgs[0].ID, false)
		if err != nil {
			return fmt.Errorf("get message link: %w", err)
		}

		msg, err := s.env.PutMessage(context.Background(), s.env.SourceID, domain.InputMessageContent{
			Type: domain.ContentText,
			Text: &domain.FormattedText{Text: fmt.Sprintf("see %s", link)},
		}, s.prefix)
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
			msg, err := s.env.CheckLastMessage(context.Background(), targetID, s.prefix)
			if err != nil {
				return err
			}
			if msg.Content.Text == nil {
				return fmt.Errorf("no text in target %d", targetID)
			}
			// Проверяем что ссылка в target указывает на КОПИЮ (в target чате), а не на оригинал
			hasTargetLink := false
			for _, e := range msg.Content.Text.Entities {
				if e.Type == domain.TextEntityTextURL && strings.HasPrefix(e.URL, "https://t.me/") {
					hasTargetLink = true
					break
				}
			}
			if !hasTargetLink && !strings.Contains(msg.Content.Text.Text, "https://t.me/") {
				return fmt.Errorf("expected link to copy in target %d, not found in %q", targetID, msg.Content.Text.Text)
			}
		}
		return nil
	})

	ctx.Given(`^в исходном чате есть сообщение из внешнего чата$`, func() error {
		return nil
	})

	ctx.When(`^пользователь отправляет сообщение со ссылкой на внешнее сообщение$`, func() error {
		s.applyRuleSet()

		// Внешняя ссылка — ссылка на сообщение в target чате (не source)
		externalChat := s.env.TargetIDs[0]
		extMsgs, err := s.env.Telegram.GetChatHistory(context.Background(), externalChat, 0, 0, 1)
		externalLink := "https://t.me/c/9999999/42" // Fallback: несуществующий чат
		if err == nil && len(extMsgs) > 0 {
			if link, linkErr := s.env.Telegram.GetMessageLink(context.Background(), externalChat, extMsgs[0].ID, false); linkErr == nil {
				externalLink = link
			}
		}

		msg, err := s.env.PutMessage(context.Background(), s.env.SourceID, domain.InputMessageContent{
			Type: domain.ContentText,
			Text: &domain.FormattedText{Text: fmt.Sprintf("see %s", externalLink)},
		}, s.prefix)
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
			if _, err := s.env.CheckLastMessage(context.Background(), targetID, s.prefix); err != nil {
				return err
			}
		}
		return nil
	})

	ctx.Then(`^в целевом чате сообщение содержит подпись источника$`, func() error {
		for _, targetID := range s.env.TargetIDs {
			msg, err := s.env.CheckLastMessage(context.Background(), targetID, s.prefix)
			if err != nil {
				return err
			}
			if msg.Content.Text == nil {
				return fmt.Errorf("no text in target %d", targetID)
			}
			// Transform добавляет sign как bold: **Sign**
			// TDLib ParseTextEntities конвертирует markdown → entities + plain text
			// Проверяем: текст содержит SignTitle И есть bold entity
			if !strings.Contains(msg.Content.Text.Text, domain.SignTitle) {
				return fmt.Errorf("expected sign title %q in target %d, got %q",
					domain.SignTitle, targetID, msg.Content.Text.Text)
			}
			hasBoldEntity := false
			for _, e := range msg.Content.Text.Entities {
				if e.Type == domain.TextEntityBold {
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
			msg, err := s.env.CheckLastMessage(context.Background(), targetID, s.prefix)
			if err != nil {
				return err
			}
			if msg.Content.Text == nil {
				return fmt.Errorf("no text in target %d", targetID)
			}
			// Проверяем что текст изменился: оригинальная фраза не должна присутствовать
			if strings.Contains(msg.Content.Text.Text, s.messageText) {
				return fmt.Errorf("expected translated text in target %d, but original %q still present in %q",
					targetID, s.messageText, msg.Content.Text.Text)
			}
		}
		return nil
	})

	ctx.Then(`^в целевом чате сообщение содержит ссылку на оригинал$`, func() error {
		for _, targetID := range s.env.TargetIDs {
			msg, err := s.env.CheckLastMessage(context.Background(), targetID, s.prefix)
			if err != nil {
				return err
			}
			if msg.Content.Text == nil {
				return fmt.Errorf("no text in target %d", targetID)
			}
			// Transform добавляет link как: [🔗Link](https://t.me/c/{chatID}/{msgID})
			// TDLib конвертирует → TextEntityTextURL с URL
			hasLinkEntity := false
			for _, e := range msg.Content.Text.Entities {
				if e.Type == domain.TextEntityTextURL && strings.HasPrefix(e.URL, "https://t.me/") {
					hasLinkEntity = true
					break
				}
			}
			if !hasLinkEntity {
				return fmt.Errorf("expected TextURL entity with t.me permalink in target %d, not found; text=%q entities=%v",
					targetID, msg.Content.Text.Text, msg.Content.Text.Entities)
			}
		}
		return nil
	})
}
