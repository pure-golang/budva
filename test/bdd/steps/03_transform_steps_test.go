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

		msg, err := s.env.PutMessage(context.Background(), s.env.SourceID, domain.InputMessageContent{
			Type: domain.ContentText,
			Text: &domain.FormattedText{
				Text: fmt.Sprintf("see https://t.me/c/%d/500", s.env.SourceID),
			},
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
			if msg.Content.Text == nil || !strings.Contains(msg.Content.Text.Text, "https://t.me") {
				return fmt.Errorf("expected link in message text, got %q", msg.Content.Text)
			}
		}
		return nil
	})

	ctx.Given(`^в исходном чате есть сообщение из внешнего чата$`, func() error {
		return nil
	})

	ctx.When(`^пользователь отправляет сообщение со ссылкой на внешнее сообщение$`, func() error {
		s.applyRuleSet()

		msg, err := s.env.PutMessage(context.Background(), s.env.SourceID, domain.InputMessageContent{
			Type: domain.ContentText,
			Text: &domain.FormattedText{
				Text: "see https://t.me/c/9999999/42",
			},
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
		firstTarget := s.env.TargetIDs[0]
		msg, err := s.env.CheckLastMessage(context.Background(), firstTarget, s.prefix)
		if err != nil {
			return err
		}
		if msg.Content.Text == nil || !strings.Contains(msg.Content.Text.Text, domain.SignTitle) {
			return fmt.Errorf("expected sign title %q in first target chat, not found", domain.SignTitle)
		}
		return nil
	})

	ctx.Then(`^в целевом чате сообщение содержит ссылку на оригинал$`, func() error {
		firstTarget := s.env.TargetIDs[0]
		msg, err := s.env.CheckLastMessage(context.Background(), firstTarget, s.prefix)
		if err != nil {
			return err
		}
		if msg.Content.Text == nil || !strings.Contains(msg.Content.Text.Text, "https://t.me") {
			return fmt.Errorf("expected link to original in first target chat, not found")
		}
		return nil
	})
}
