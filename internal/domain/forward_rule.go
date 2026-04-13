package domain

import "regexp"

// ForwardRuleID — идентификатор правила пересылки.
type ForwardRuleID = string

// FiltersMode — результат проверки сообщения фильтром.
type FiltersMode = string

const (
	// FiltersOK — сообщение прошло фильтрацию.
	FiltersOK FiltersMode = "ok"
	// FiltersCheck — сообщение направлено в чат проверки.
	FiltersCheck FiltersMode = "check"
	// FiltersOther — сообщение направлено в чат прочих.
	FiltersOther FiltersMode = "other"
)

// ForwardRule описывает правило пересылки сообщений из одного чата в другие.
type ForwardRule struct {
	// ID — уникальный идентификатор правила, заполняется при загрузке.
	ID ForwardRuleID `yaml:"-"`
	// From — идентификатор чата-источника.
	From ChatID `yaml:"from"`
	// To — список идентификаторов чатов-получателей.
	To []ChatID `yaml:"to"`
	// SendCopy — отправлять копию вместо форварда.
	SendCopy bool `yaml:"sendCopy"`
	// CopyOnce — при редактировании создавать новую копию вместо обновления.
	CopyOnce bool `yaml:"copyOnce"`
	// Indelible — не удалять копии при удалении оригинала.
	Indelible bool `yaml:"indelible"`
	// Exclude — регулярное выражение для исключения сообщений.
	Exclude string `yaml:"exclude"`
	// Include — регулярное выражение для включения сообщений.
	Include string `yaml:"include"`
	// IncludeSubmatch — правила фильтрации по подстрокам.
	IncludeSubmatch []*SubmatchRule `yaml:"includeSubmatch"`
	// Other — чат для сообщений, не прошедших включающий фильтр.
	Other ChatID `yaml:"other"`
	// Check — чат для сообщений, попавших под исключающий фильтр.
	Check ChatID `yaml:"check"`
}

// SubmatchRule описывает правило фильтрации по группам регулярного выражения.
type SubmatchRule struct {
	// Regexp — исходное регулярное выражение.
	Regexp string `yaml:"regexp"`
	// CompiledRegexp — скомпилированное регулярное выражение.
	CompiledRegexp *regexp.Regexp `yaml:"-"`
	// Group — номер группы для сравнения.
	Group int `yaml:"group"`
	// Match — список строк для сравнения с найденной подстрокой.
	Match []string `yaml:"match"`
}
