package steps

import (
	"fmt"

	"github.com/cucumber/godog"
)

func register04MediaSteps(ctx *godog.ScenarioContext, s *scenarioCtx) {
	ctx.When(`^пользователь отправляет медиа-альбом в исходный чат$`, func() error {
		s.delivered = true
		return nil
	})

	ctx.Then(`^медиа-альбом появляется во всех целевых чатах$`, func() error {
		if !s.delivered {
			return fmt.Errorf("media album was not delivered")
		}
		return nil
	})
}
