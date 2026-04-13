package steps

import (
	"context"

	"github.com/cucumber/godog"
)

// scenarioCtx хранит состояние одного сценария.
type scenarioCtx struct {
	deliveryMode string
	sourceType   string
	messageText  string
	expectedText string
	delivered    bool
	deleted      bool

	// Опции правила
	copyOnce  bool
	indelible bool

	// Опции источника
	signEnabled      bool
	linkEnabled      bool
	translateEnabled bool
	autoAnswer       bool

	// Фильтры
	excludePattern  string
	includePattern  string
	submatchPattern string
}

func (s *scenarioCtx) reset() {
	*s = scenarioCtx{}
}

func initScenario(ctx *godog.ScenarioContext) {
	s := &scenarioCtx{}

	ctx.Before(func(ctx context.Context, sc *godog.Scenario) (context.Context, error) {
		s.reset()
		return ctx, nil
	})

	register01DeliverySteps(ctx, s)
	register02FilterSteps(ctx, s)
	register03TransformSteps(ctx, s)
	register04MediaSteps(ctx, s)
	register05SyncSteps(ctx, s)
	register06AutoSteps(ctx, s)
}
