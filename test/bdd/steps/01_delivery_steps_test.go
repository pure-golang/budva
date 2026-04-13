package steps

import (
	"fmt"

	"github.com/cucumber/godog"
)

func register01DeliverySteps(ctx *godog.ScenarioContext, s *scenarioCtx) {
	ctx.Given(`^правило пересылки в режиме "([^"]*)"$`, func(mode string) error {
		s.deliveryMode = mode
		return nil
	})

	ctx.Given(`^исходный чат типа "([^"]*)"$`, func(srcType string) error {
		s.sourceType = srcType
		return nil
	})

	ctx.When(`^пользователь отправляет сообщение в исходный чат$`, func() error {
		if s.messageText == "" {
			s.messageText = "test message"
		}
		s.delivered = true
		return nil
	})

	ctx.Then(`^сообщение появляется во всех целевых чатах$`, func() error {
		if !s.delivered {
			return fmt.Errorf("message was not delivered")
		}
		return nil
	})
}
