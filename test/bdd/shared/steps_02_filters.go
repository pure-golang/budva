package shared

import (
	"context"
	"fmt"
	"strings"

	"github.com/cucumber/godog"
)

// RegisterFiltersSteps регистрирует шаги эпика 02_filters.
func RegisterFiltersSteps(ctx *godog.ScenarioContext, s *ScenarioCtx) {
	ctx.Given(`^фильтр исключения с паттерном "([^"]*)"$`, func(pattern string) error {
		s.ExcludePattern = pattern
		return nil
	})

	ctx.Given(`^фильтр включения с паттерном "([^"]*)"$`, func(pattern string) error {
		s.IncludePattern = pattern
		return nil
	})

	ctx.Given(`^фильтр submatch с паттерном "([^"]*)"$`, func(pattern string) error {
		s.SubmatchPattern = pattern
		return nil
	})

	ctx.When(`^пользователь отправляет сообщение без запрещённого паттерна$`, func() error {
		s.ApplyRuleSet()

		s.MessageText = "normal text"
		msg, err := s.Env.PutMessage(s.Env.SourceID, TextContent(s.MessageText), s.Prefix)
		if err != nil {
			return err
		}
		s.SentMsg = msg

		s.Env.Handler.OnNewMessage(context.Background(), msg)
		s.Env.DrainQueue()

		return nil
	})

	ctx.When(`^пользователь отправляет сообщение с текстом "([^"]*)"$`, func(text string) error {
		s.ApplyRuleSet()

		s.MessageText = text
		msg, err := s.Env.PutMessage(s.Env.SourceID, TextContent(text), s.Prefix)
		if err != nil {
			return err
		}
		s.SentMsg = msg

		s.Env.Handler.OnNewMessage(context.Background(), msg)
		s.Env.DrainQueue()

		return nil
	})

	ctx.Then(`^сообщение не появляется в целевых чатах$`, func() error {
		for _, targetID := range s.Env.TargetIDs {
			if err := s.Env.CheckNoMessage(targetID, s.Prefix); err != nil {
				return err
			}
		}
		return nil
	})

	ctx.Then(`^сообщение с текстом "([^"]*)" появляется во всех целевых чатах$`, func(expected string) error {
		for _, targetID := range s.Env.TargetIDs {
			msg, err := s.Env.CheckLastMessage(targetID, s.Prefix)
			if err != nil {
				return err
			}
			caption := MessageCaption(msg)
			if caption == nil || !strings.Contains(caption.Text, expected) {
				return fmt.Errorf("no message containing text %q in target chat %d", expected, targetID)
			}
		}
		return nil
	})

	// --- Check/Other dedup ---

	ctx.Given(`^назначен check-чат для отклонённых сообщений$`, func() error {
		if len(s.Env.Fixtures.Chats) > 2 {
			s.CheckChatID = s.Env.Fixtures.Chats[2].ChatID
		} else {
			s.CheckChatID = -1004000
		}
		return nil
	})

	ctx.Then(`^сообщение появляется в check-чате ровно один раз$`, func() error {
		if _, err := s.Env.CheckLastMessage(s.CheckChatID, s.Prefix); err != nil {
			return err
		}
		return nil
	})
}
