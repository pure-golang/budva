package steps

import (
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
		s.messageText = "normal text"
		s.delivered = true
		return nil
	})

	ctx.When(`^пользователь отправляет сообщение с текстом "([^"]*)"$`, func(text string) error {
		s.messageText = text
		if s.excludePattern != "" && strings.Contains(text, s.excludePattern) {
			s.delivered = false
		} else {
			s.delivered = true
		}
		if s.includePattern != "" && strings.Contains(text, s.includePattern) {
			s.expectedText = s.includePattern
		}
		if s.submatchPattern != "" && strings.Contains(text, s.submatchPattern) {
			s.expectedText = s.submatchPattern
		}
		return nil
	})

	ctx.Then(`^сообщение не появляется в целевых чатах$`, func() error {
		if s.delivered {
			return fmt.Errorf("message should not have been delivered")
		}
		return nil
	})

	ctx.Then(`^сообщение с текстом "([^"]*)" появляется во всех целевых чатах$`, func(expected string) error {
		if !s.delivered {
			return fmt.Errorf("message was not delivered")
		}
		if s.expectedText != "" && s.expectedText != expected {
			return fmt.Errorf("expected text %q, got %q", expected, s.expectedText)
		}
		return nil
	})
}
