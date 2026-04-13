package domain

// Destination описывает настройки чата-получателя.
type Destination struct {
	// ChatID — идентификатор чата-получателя, заполняется при загрузке.
	ChatID ChatID
	// ReplaceMyselfLinks — настройки замены ссылок на свои сообщения.
	ReplaceMyselfLinks *ReplaceMyselfLinks
	// ReplaceFragments — правила замены фрагментов текста.
	ReplaceFragments []*ReplaceFragment
}

// ReplaceMyselfLinks описывает настройки замены ссылок на свои сообщения.
type ReplaceMyselfLinks struct {
	// Run — включить замену ссылок.
	Run bool
	// DeleteExternal — удалять ссылки на сообщения в чужих чатах.
	DeleteExternal bool
	// DeletedLinkText — текст-заменитель для удалённой ссылки.
	DeletedLinkText string
}

// DeletedLink — маркер удалённой внешней ссылки.
const DeletedLink = "DELETED_LINK"

// ReplaceFragment описывает правило замены фрагмента текста.
type ReplaceFragment struct {
	// From — исходный текст.
	From string
	// To — текст замены.
	To string
}
