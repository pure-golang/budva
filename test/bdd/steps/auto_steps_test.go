package steps

import (
	"github.com/cucumber/godog"
)

func registerAutoSteps(ctx *godog.ScenarioContext, s *scenarioCtx) {
	ctx.Given(`^в исходном чате появляется сообщение с callback-запросом$`, func() error {
		return godog.ErrPending
	})

	ctx.Then(`^бот автоматически отвечает на запрос$`, func() error {
		return godog.ErrPending
	})
}
