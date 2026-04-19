// Package shared содержит общий контекст BDD-сценариев и общие шаги,
// переиспользуемые между feature-группами test/bdd/NN_*.
package shared

import (
	"regexp"

	"github.com/zelenin/go-tdlib/client"

	"github.com/pure-golang/budva-claude/internal/domain"
	testsupport "github.com/pure-golang/budva-claude/internal/test/support"
)

// ScenarioCtx хранит состояние одного BDD-сценария.
type ScenarioCtx struct {
	Env *testsupport.LiveStack

	// Prefix — уникальный маркер сценария (см. GeneratePrefix). Добавляется к тексту
	// сообщений, чтобы отличить сообщения текущего сценария от сообщений соседних
	// сценариев внутри одного прогона. От мусора прошлых прогонов защищает отдельный
	// фильтр по msg.Date в LiveStack.isFresh.
	Prefix string

	DeliveryMode string
	SourceType   string
	MessageText  string
	SentMsg      *client.Message

	// Опции правила
	SendCopy             bool
	CopyOnce             bool
	Indelible            bool
	DeleteSystemMessages bool

	// Опции источника
	Src *domain.Source

	// Фильтры
	ExcludePattern  string
	IncludePattern  string
	SubmatchPattern string

	// Замены фрагментов
	ReplaceFrom []string
	ReplaceTo   []string

	// Retry testing
	SkipRetryDrain bool

	// Связь perm↔tmp per target для сценария eventual-consistency (05_sync/05):
	// step «permanent ID ещё не записан» удаляет mapping по корректному ключу,
	// step «permanent ID записывается» — восстанавливает его по сохранённому tmp.
	TmpIDByTarget  map[int64]int64
	PermIDByTarget map[int64]int64

	// Check/Other чаты
	CheckChatID int64

	// Пересланное сообщение из канала
	ForwardedMsg *client.Message
}

// Reset сбрасывает состояние сценария и переиспользует общий LiveStack.
func (s *ScenarioCtx) Reset() error {
	env, err := GetOrCreateStack()
	if err != nil {
		return err
	}

	if err := env.ResetState(); err != nil {
		return err
	}

	s.Env = env
	s.Prefix = GeneratePrefix()
	s.DeliveryMode = ""
	s.SourceType = ""
	s.MessageText = ""
	s.SentMsg = nil
	s.SendCopy = false
	s.CopyOnce = false
	s.Indelible = false
	s.DeleteSystemMessages = false
	s.Src = &domain.Source{ChatID: env.SourceID}
	s.ExcludePattern = ""
	s.IncludePattern = ""
	s.SubmatchPattern = ""
	s.ReplaceFrom = nil
	s.ReplaceTo = nil
	s.SkipRetryDrain = false
	s.TmpIDByTarget = make(map[int64]int64)
	s.PermIDByTarget = make(map[int64]int64)
	s.CheckChatID = 0
	s.ForwardedMsg = nil
	return nil
}

// ApplyRuleSet собирает RuleSet из накопленного состояния и устанавливает в handler.
func (s *ScenarioCtx) ApplyRuleSet() {
	s.Src.DeleteSystemMessages = s.DeleteSystemMessages

	// TargetIDs могут быть переопределены шагом «целевой чат типа» после того,
	// как опции источника (Link/Sign/Translate/Prev/Next) были привязаны к
	// прежнему slice. Пересобираем `For` здесь, чтобы transform-проверки
	// containsChatID(For, DstChatID) видели актуальный набор целевых чатов.
	if s.Src.Link != nil {
		s.Src.Link.For = s.Env.TargetIDs
	}
	if s.Src.Sign != nil {
		s.Src.Sign.For = s.Env.TargetIDs
	}
	if s.Src.Translate != nil {
		s.Src.Translate.For = s.Env.TargetIDs
	}
	if s.Src.Prev != nil {
		s.Src.Prev.For = s.Env.TargetIDs
	}
	if s.Src.Next != nil {
		s.Src.Next.For = s.Env.TargetIDs
	}

	rs := s.Env.MakeRuleSet(s.SendCopy, s.Src)

	rule := rs.ForwardRules["test_rule"]
	rule.CopyOnce = s.CopyOnce
	rule.Indelible = s.Indelible
	rule.Exclude = s.ExcludePattern
	rule.Include = s.IncludePattern
	if s.SubmatchPattern != "" {
		rule.IncludeSubmatch = []*domain.SubmatchRule{{
			Regexp:         regexp.QuoteMeta(s.SubmatchPattern),
			CompiledRegexp: regexp.MustCompile(regexp.QuoteMeta(s.SubmatchPattern)),
			Group:          0,
			Match:          []string{s.SubmatchPattern},
		}}
	}
	rule.Check = s.CheckChatID

	if len(s.ReplaceFrom) > 0 {
		for _, dstID := range s.Env.TargetIDs {
			dst := rs.Destinations[dstID]
			for i := range s.ReplaceFrom {
				dst.ReplaceFragments = append(dst.ReplaceFragments, &domain.ReplaceFragment{
					From: s.ReplaceFrom[i],
					To:   s.ReplaceTo[i],
				})
			}
		}
	}

	s.Env.Handler.SetRuleSet(rs)
}
