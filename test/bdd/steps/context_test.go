//go:build bdd

package steps

import (
	"context"
	"fmt"
	"regexp"
	"sync/atomic"

	"github.com/cucumber/godog"
	"github.com/zelenin/go-tdlib/client"

	"github.com/pure-golang/budva-claude/internal/domain"
	"github.com/pure-golang/budva-claude/test/support"
)

const fixturesPath = ".config/stand.json"

// sharedStack создаётся один раз для всех сценариев (TDLib не пересоздаётся).
var (
	sharedStack    *support.LiveStack
	sharedStackErr error
)

func getOrCreateStack() (*support.LiveStack, error) {
	if sharedStack != nil {
		return sharedStack, nil
	}
	if sharedStackErr != nil {
		return nil, sharedStackErr
	}
	stack := support.NewLiveStack(fixturesPath)
	if err := stack.Start(); err != nil {
		sharedStackErr = err
		return nil, err
	}
	sharedStack = stack
	return stack, nil
}

// scenarioCtx хранит состояние одного сценария.
type scenarioCtx struct {
	env *support.LiveStack

	// prefix — уникальный маркер сценария (nanoid). Добавляется к тексту сообщений,
	// чтобы отличить сообщения текущего сценария от мусора прошлых запусков.
	prefix string

	deliveryMode string
	sourceType   string
	messageText  string
	sentMsg      *client.Message

	// Опции правила
	sendCopy             bool
	copyOnce             bool
	indelible            bool
	deleteSystemMessages bool

	// Опции источника
	src *domain.Source

	// Фильтры
	excludePattern  string
	includePattern  string
	submatchPattern string

	// Замены фрагментов
	replaceFrom []string
	replaceTo   []string

	// Retry testing
	skipRetryDrain bool

	// Check/Other чаты
	checkChatID int64

	// Пересланное сообщение из канала
	forwardedMsg *client.Message
}

// scenarioSeq — глобальный счётчик сценариев для маркера.
// Цифровой маркер не переводится TDLib (в отличие от hex, где `da...` → `Да...`),
// а последовательность даёт стабильный порядок сценариев в логах.
var scenarioSeq atomic.Uint64

// generatePrefix возвращает маркер сценария из трёх цифр с ведущими нулями.
func generatePrefix() string {
	return fmt.Sprintf("%03d", scenarioSeq.Add(1))
}

func (s *scenarioCtx) reset() error {
	env, err := getOrCreateStack()
	if err != nil {
		return err
	}

	if err := env.ResetState(); err != nil {
		return err
	}

	s.env = env
	s.prefix = generatePrefix()
	s.deliveryMode = ""
	s.sourceType = ""
	s.messageText = ""
	s.sentMsg = nil
	s.sendCopy = false
	s.copyOnce = false
	s.indelible = false
	s.deleteSystemMessages = false
	s.src = &domain.Source{ChatID: env.SourceID}
	s.excludePattern = ""
	s.includePattern = ""
	s.submatchPattern = ""
	s.replaceFrom = nil
	s.replaceTo = nil
	s.skipRetryDrain = false
	s.checkChatID = 0
	s.forwardedMsg = nil
	return nil
}

// applyRuleSet собирает RuleSet из накопленного состояния и устанавливает в handler.
func (s *scenarioCtx) applyRuleSet() {
	s.src.DeleteSystemMessages = s.deleteSystemMessages

	// TargetIDs могут быть переопределены шагом «целевой чат типа» после того,
	// как опции источника (Link/Sign/Translate/Prev/Next) были привязаны к
	// прежнему slice. Пересобираем `For` здесь, чтобы transform-проверки
	// containsChatID(For, DstChatID) видели актуальный набор целевых чатов.
	if s.src.Link != nil {
		s.src.Link.For = s.env.TargetIDs
	}
	if s.src.Sign != nil {
		s.src.Sign.For = s.env.TargetIDs
	}
	if s.src.Translate != nil {
		s.src.Translate.For = s.env.TargetIDs
	}
	if s.src.Prev != nil {
		s.src.Prev.For = s.env.TargetIDs
	}
	if s.src.Next != nil {
		s.src.Next.For = s.env.TargetIDs
	}

	rs := s.env.MakeRuleSet(s.sendCopy, s.src)

	rule := rs.ForwardRules["test_rule"]
	rule.CopyOnce = s.copyOnce
	rule.Indelible = s.indelible
	rule.Exclude = s.excludePattern
	rule.Include = s.includePattern
	if s.submatchPattern != "" {
		rule.IncludeSubmatch = []*domain.SubmatchRule{{
			Regexp:         regexp.QuoteMeta(s.submatchPattern),
			CompiledRegexp: regexp.MustCompile(regexp.QuoteMeta(s.submatchPattern)),
			Group:          0,
			Match:          []string{s.submatchPattern},
		}}
	}
	rule.Check = s.checkChatID

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
		return gctx, nil
	})

	// Общие Given-шаги для выбора чатов по имени фикстуры
	ctx.Given(`^исходный чат типа "([^"]*)"$`, func(srcType string) error {
		s.sourceType = srcType
		fix, err := s.env.ChatByName(srcType)
		if err != nil {
			return err
		}
		s.env.SourceID = fix.ChatID
		s.src.ChatID = fix.ChatID
		return nil
	})

	ctx.Given(`^целевой чат типа "([^"]*)"$`, func(dstType string) error {
		fix, err := s.env.ChatByName(dstType)
		if err != nil {
			return err
		}
		s.env.TargetIDs = []int64{fix.ChatID}
		return nil
	})

	register01DeliverySteps(ctx, s)
	register02FiltersSteps(ctx, s)
	register03TransformSteps(ctx, s)
	register04MediaSteps(ctx, s)
	register05SyncSteps(ctx, s)
	register06AutoSteps(ctx, s)
}
