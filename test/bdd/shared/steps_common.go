package shared

import (
	"github.com/cucumber/godog"
)

// RegisterAllSteps регистрирует шаги всех эпиков и общие Given-шаги.
// Используется каждым per-epic initScenario, поскольку feature-файлы свободно
// переиспользуют шаги между эпиками (например, «правило пересылки в режиме»
// из 01_delivery встречается в 02..05).
func RegisterAllSteps(ctx *godog.ScenarioContext, s *ScenarioCtx) {
	RegisterCommonSteps(ctx, s)
	RegisterDeliverySteps(ctx, s)
	RegisterFiltersSteps(ctx, s)
	RegisterTransformSteps(ctx, s)
	RegisterMediaSteps(ctx, s)
	RegisterSyncSteps(ctx, s)
	RegisterAutoSteps(ctx, s)
}

// RegisterCommonSteps регистрирует Given-шаги выбора чатов по имени фикстуры,
// используемые во всех эпиках.
func RegisterCommonSteps(ctx *godog.ScenarioContext, s *ScenarioCtx) {
	ctx.Given(`^исходный чат типа "([^"]*)"$`, func(srcType string) error {
		s.SourceType = srcType
		fix, err := s.Env.ChatByName(srcType)
		if err != nil {
			return err
		}
		s.Env.SourceID = fix.ChatID
		s.Src.ChatID = fix.ChatID
		return nil
	})

	ctx.Given(`^целевой чат типа "([^"]*)"$`, func(dstType string) error {
		fix, err := s.Env.ChatByName(dstType)
		if err != nil {
			return err
		}
		s.Env.TargetIDs = []int64{fix.ChatID}
		return nil
	})
}
