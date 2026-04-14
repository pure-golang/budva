package steps

import (
	"context"
	"fmt"

	"github.com/cucumber/godog"

	"github.com/pure-golang/budva-claude/internal/domain"
)

func registerTransformSteps(ctx *godog.ScenarioContext, s *scenarioCtx) {
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
		msg := &domain.Message{
			ChatID: s.env.SourceID,
			ID:     500,
			Content: domain.MessageContent{
				Type: domain.ContentText,
				Text: &domain.FormattedText{Text: "previous message"},
			},
		}
		s.env.Telegram.PutMessage(msg)
		return nil
	})

	ctx.When(`^пользователь отправляет сообщение со ссылкой на предыдущее сообщение$`, func() error {
		s.applyRuleSet()

		msg := &domain.Message{
			ChatID:     s.env.SourceID,
			ID:         2,
			CanBeSaved: true,
			Content: domain.MessageContent{
				Type: domain.ContentText,
				Text: &domain.FormattedText{
					Text: fmt.Sprintf("see https://t.me/c/%d/500", s.env.SourceID),
				},
			},
		}
		s.sentMsg = msg
		s.env.Telegram.PutMessage(msg)

		s.env.Handler.OnNewMessage(context.Background(), msg)
		s.env.DrainQueue()

		return nil
	})

	ctx.Then(`^в целевом чате ссылка указывает на копию предыдущего сообщения$`, func() error {
		for _, targetID := range s.env.TargetIDs {
			msgs := s.env.Telegram.MessagesInChat(targetID)
			if len(msgs) == 0 {
				return fmt.Errorf("no messages in target chat %d", targetID)
			}
		}
		return nil
	})

	ctx.Given(`^в исходном чате есть сообщение из внешнего чата$`, func() error {
		return nil
	})

	ctx.When(`^пользователь отправляет сообщение со ссылкой на внешнее сообщение$`, func() error {
		s.applyRuleSet()

		msg := &domain.Message{
			ChatID:     s.env.SourceID,
			ID:         3,
			CanBeSaved: true,
			Content: domain.MessageContent{
				Type: domain.ContentText,
				Text: &domain.FormattedText{
					Text: "see https://t.me/c/9999999/42",
				},
			},
		}
		s.sentMsg = msg
		s.env.Telegram.PutMessage(msg)

		s.env.Handler.OnNewMessage(context.Background(), msg)
		s.env.DrainQueue()

		return nil
	})

	ctx.Then(`^в целевом чате сообщение появляется без внешней ссылки$`, func() error {
		for _, targetID := range s.env.TargetIDs {
			msgs := s.env.Telegram.MessagesInChat(targetID)
			if len(msgs) == 0 {
				return fmt.Errorf("no messages in target chat %d", targetID)
			}
		}
		return nil
	})

	ctx.Then(`^в целевом чате сообщение содержит подпись источника$`, func() error {
		for _, targetID := range s.env.TargetIDs {
			msgs := s.env.Telegram.MessagesInChat(targetID)
			if len(msgs) == 0 {
				return fmt.Errorf("no messages in target chat %d", targetID)
			}
		}
		return nil
	})

	ctx.Then(`^в целевом чате сообщение содержит ссылку на оригинал$`, func() error {
		for _, targetID := range s.env.TargetIDs {
			msgs := s.env.Telegram.MessagesInChat(targetID)
			if len(msgs) == 0 {
				return fmt.Errorf("no messages in target chat %d", targetID)
			}
		}
		return nil
	})
}
