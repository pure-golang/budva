//go:build bdd

package steps

import (
	"context"
	"fmt"
	"strings"

	"github.com/cucumber/godog"
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
		msg, err := s.env.PutMessage(context.Background(), s.env.SourceID, textContent(s.messageText), s.prefix)
		if err != nil {
			return err
		}
		s.sentMsg = msg

		s.env.Handler.OnNewMessage(context.Background(), msg)
		s.env.DrainQueue()

		return nil
	})

	ctx.When(`^пользователь отправляет сообщение с текстом "([^"]*)"$`, func(text string) error {
		s.applyRuleSet()

		s.messageText = text
		msg, err := s.env.PutMessage(context.Background(), s.env.SourceID, textContent(text), s.prefix)
		if err != nil {
			return err
		}
		s.sentMsg = msg

		s.env.Handler.OnNewMessage(context.Background(), msg)
		s.env.DrainQueue()

		return nil
	})

	ctx.Then(`^сообщение не появляется в целевых чатах$`, func() error {
		for _, targetID := range s.env.TargetIDs {
			if err := s.env.CheckNoMessage(context.Background(), targetID, s.prefix); err != nil {
				return err
			}
		}
		return nil
	})

	ctx.Then(`^сообщение с текстом "([^"]*)" появляется во всех целевых чатах$`, func(expected string) error {
		for _, targetID := range s.env.TargetIDs {
			msg, err := s.env.CheckLastMessage(context.Background(), targetID, s.prefix)
			if err != nil {
				return err
			}
			caption := messageCaption(msg)
			if caption == nil || !strings.Contains(caption.Text, expected) {
				return fmt.Errorf("no message containing text %q in target chat %d", expected, targetID)
			}
		}
		return nil
	})

	// --- Check/Other dedup ---

	ctx.Given(`^назначен check-чат для отклонённых сообщений$`, func() error {
		if len(s.env.Fixtures.Chats) > 2 {
			s.checkChatID = s.env.Fixtures.Chats[2].ChatID
		} else {
			s.checkChatID = -1004000
		}
		return nil
	})

	ctx.Then(`^сообщение появляется в check-чате ровно один раз$`, func() error {
		if _, err := s.env.CheckLastMessage(context.Background(), s.checkChatID, s.prefix); err != nil {
			return err
		}
		return nil
	})
}
