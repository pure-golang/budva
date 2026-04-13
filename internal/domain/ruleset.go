package domain

// RuleSet описывает полную конфигурацию правил пересылки.
type RuleSet struct {
	// Sources — настройки источников по идентификатору чата.
	Sources map[ChatID]*Source `yaml:"sources"`
	// Destinations — настройки получателей по идентификатору чата.
	Destinations map[ChatID]*Destination `yaml:"destinations"`
	// ForwardRules — правила пересылки по идентификатору правила.
	ForwardRules map[ForwardRuleID]*ForwardRule `yaml:"forwardRules"`
	// UniqueSources — множество уникальных идентификаторов источников.
	UniqueSources map[ChatID]struct{} `yaml:"-"`
	// UniqueDestinations — множество уникальных идентификаторов получателей.
	UniqueDestinations map[ChatID]struct{} `yaml:"-"`
	// OrderedForwardRules — идентификаторы правил в порядке обработки.
	OrderedForwardRules []ForwardRuleID `yaml:"-"`
}
