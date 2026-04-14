package steps

import (
	"context"

	"github.com/cucumber/godog"

	"github.com/pure-golang/budva-claude/internal/domain"
	"github.com/pure-golang/budva-claude/test/support"
)

// scenarioCtx хранит состояние одного сценария.
type scenarioCtx struct {
	env *support.TestEnv

	deliveryMode string
	sourceType   string
	messageText  string
	sentMsg      *domain.Message

	// Опции правила
	sendCopy  bool
	copyOnce  bool
	indelible bool

	// Опции источника
	src *domain.Source

	// Фильтры
	excludePattern  string
	includePattern  string
	submatchPattern string

	// Замены фрагментов
	replaceFrom []string
	replaceTo   []string
}

func (s *scenarioCtx) reset() error {
	if s.env != nil {
		s.env.Close()
	}
	env, err := support.NewTestEnv()
	if err != nil {
		return err
	}
	s.env = env
	s.deliveryMode = ""
	s.sourceType = ""
	s.messageText = ""
	s.sentMsg = nil
	s.sendCopy = false
	s.copyOnce = false
	s.indelible = false
	s.src = &domain.Source{ChatID: env.SourceID}
	s.excludePattern = ""
	s.includePattern = ""
	s.submatchPattern = ""
	s.replaceFrom = nil
	s.replaceTo = nil
	return nil
}

// applyRuleSet собирает RuleSet из накопленного состояния и устанавливает в handler.
func (s *scenarioCtx) applyRuleSet() {
	rs := s.env.MakeRuleSet(s.sendCopy, s.src)

	rule := rs.ForwardRules["test_rule"]
	rule.CopyOnce = s.copyOnce
	rule.Indelible = s.indelible
	rule.Exclude = s.excludePattern
	rule.Include = s.includePattern

	if len(s.replaceFrom) > 0 {
		for _, dstID := range s.env.TargetIDs {
			dst := rs.Destinations[dstID]
			for i := range s.replaceFrom {
				dst.ReplaceFragments = append(dst.ReplaceFragments, &domain.ReplaceFragment{
					From: s.replaceFrom[i],
					To:   s.replaceTo[i],
				})
			}
		}
	}

	s.env.Handler.SetRuleSet(rs)
}

func initScenario(ctx *godog.ScenarioContext) {
	s := &scenarioCtx{}

	ctx.Before(func(gctx context.Context, sc *godog.Scenario) (context.Context, error) {
		if err := s.reset(); err != nil {
			return gctx, err
		}
		return gctx, nil
	})

	ctx.After(func(gctx context.Context, sc *godog.Scenario, err error) (context.Context, error) {
		if s.env != nil {
			s.env.Close()
		}
		return gctx, nil
	})

	register01DeliverySteps(ctx, s)
	register02FiltersSteps(ctx, s)
	register03TransformSteps(ctx, s)
	register04MediaSteps(ctx, s)
	register05SyncSteps(ctx, s)
	register06AutoSteps(ctx, s)
}
