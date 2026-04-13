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
	ID ForwardRuleID
	// From — идентификатор чата-источника.
	From ChatID
	// To — список идентификаторов чатов-получателей.
	To []ChatID
	// SendCopy — отправлять копию вместо форварда.
	SendCopy bool
	// CopyOnce — при редактировании создавать новую копию вместо обновления.
	CopyOnce bool
	// Indelible — не удалять копии при удалении оригинала.
	Indelible bool
	// Exclude — регулярное выражение для исключения сообщений.
	Exclude string
	// Include — регулярное выражение для включения сообщений.
	Include string
	// IncludeSubmatch — правила фильтрации по подстрокам.
	IncludeSubmatch []*SubmatchRule
	// Other — чат для сообщений, не прошедших включающий фильтр.
	Other ChatID
	// Check — чат для сообщений, попавших под исключающий фильтр.
	Check ChatID
}

// SubmatchRule описывает правило фильтрации по группам регулярного выражения.
type SubmatchRule struct {
	// Regexp — исходное регулярное выражение.
	Regexp string
	// CompiledRegexp — скомпилированное регулярное выражение.
	CompiledRegexp *regexp.Regexp
	// Group — номер группы для сравнения.
	Group int
	// Match — список строк для сравнения с найденной подстрокой.
	Match []string
}
