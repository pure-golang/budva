package steps

import (
	"context"
	"fmt"
	"strings"

	"github.com/cucumber/godog"

	"github.com/pure-golang/budva-claude/internal/domain"
)

func register02FiltersSteps(ctx *godog.ScenarioContext, s *scenarioCtx) {
	ctx.Given(`^фильтр исключения с паттерном "([^"]*)"$`, func(pattern string) error {
		s.excludePattern = pattern
		return nil
	})

	ctx.Given(`^фильтр включения с паттерном "([^"]*)"$`, func(pattern string) error {
		s.includePattern = pattern
		return nil
	})

	ctx.Given(`^фильтр submatch с паттерном "([^"]*)"$`, func(pattern string) error {
		s.submatchPattern = pattern
		return nil
	})

	ctx.When(`^пользователь отправляет сообщение без запрещённого паттерна$`, func() error {
		s.applyRuleSet()

		s.messageText = "normal text"
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
		s.env.TelegramFake.PutMessage(msg)

		s.env.Handler.OnNewMessage(context.Background(), msg)
		s.env.DrainQueue()

		return nil
	})

	ctx.When(`^пользователь отправляет сообщение с текстом "([^"]*)"$`, func(text string) error {
		s.applyRuleSet()

		s.messageText = text
		msg := &domain.Message{
			ChatID:     s.env.SourceID,
			ID:         1,
			CanBeSaved: true,
			Content: domain.MessageContent{
				Type: domain.ContentText,
				Text: &domain.FormattedText{Text: text},
			},
		}
		s.sentMsg = msg
		s.env.TelegramFake.PutMessage(msg)

		s.env.Handler.OnNewMessage(context.Background(), msg)
		s.env.DrainQueue()

		return nil
	})

	ctx.Then(`^сообщение не появляется в целевых чатах$`, func() error {
		for _, targetID := range s.env.TargetIDs {
			msgs := s.env.TelegramFake.MessagesInChat(targetID)
			if len(msgs) > 0 {
				return fmt.Errorf("expected no messages in target chat %d, got %d", targetID, len(msgs))
			}
		}
		return nil
	})

	ctx.Then(`^сообщение с текстом "([^"]*)" появляется во всех целевых чатах$`, func(expected string) error {
		for _, targetID := range s.env.TargetIDs {
			msgs := s.env.TelegramFake.MessagesInChat(targetID)
			if len(msgs) == 0 {
				return fmt.Errorf("no messages in target chat %d for expected text %q", targetID, expected)
			}
			found := false
			for _, m := range msgs {
				if m.Content.Text != nil && strings.Contains(m.Content.Text.Text, expected) {
					found = true
					break
				}
			}
			if !found {
				return fmt.Errorf("no message containing text %q in target chat %d", expected, targetID)
			}
		}
		return nil
	})

	// --- Check/Other dedup ---

	ctx.Given(`^назначен check-чат для отклонённых сообщений$`, func() error {
		s.checkChatID = -1004000
		return nil
	})

	ctx.Then(`^сообщение появляется в check-чате ровно один раз$`, func() error {
		msgs := s.env.TelegramFake.MessagesInChat(s.checkChatID)
		if len(msgs) != 1 {
			return fmt.Errorf("expected exactly 1 message in check chat %d, got %d", s.checkChatID, len(msgs))
		}
		return nil
	})
}
