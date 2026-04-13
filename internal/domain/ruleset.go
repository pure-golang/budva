package domain

// RuleSet описывает полную конфигурацию правил пересылки.
type RuleSet struct {
	// Sources — настройки источников по идентификатору чата.
	Sources map[ChatID]*Source
	// Destinations — настройки получателей по идентификатору чата.
	Destinations map[ChatID]*Destination
	// ForwardRules — правила пересылки по идентификатору правила.
	ForwardRules map[ForwardRuleID]*ForwardRule
	// UniqueSources — множество уникальных идентификаторов источников.
	UniqueSources map[ChatID]struct{}
	// UniqueDestinations — множество уникальных идентификаторов получателей.
	UniqueDestinations map[ChatID]struct{}
	// OrderedForwardRules — идентификаторы правил в порядке обработки.
	OrderedForwardRules []ForwardRuleID
}
