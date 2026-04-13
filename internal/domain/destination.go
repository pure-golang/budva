package domain

// Destination описывает настройки чата-получателя.
type Destination struct {
	// ChatID — идентификатор чата-получателя, заполняется при загрузке.
	ChatID ChatID `yaml:"-"`
	// ReplaceMyselfLinks — настройки замены ссылок на свои сообщения.
	ReplaceMyselfLinks *ReplaceMyselfLinks `yaml:"replaceMyselfLinks"`
	// ReplaceFragments — правила замены фрагментов текста.
	ReplaceFragments []*ReplaceFragment `yaml:"replaceFragments"`
}

// ReplaceMyselfLinks описывает настройки замены ссылок на свои сообщения.
type ReplaceMyselfLinks struct {
	// Run — включить замену ссылок.
	Run bool `yaml:"run"`
	// DeleteExternal — удалять ссылки на сообщения в чужих чатах.
	DeleteExternal bool `yaml:"deleteExternal"`
	// DeletedLinkText — текст-заменитель для удалённой ссылки.
	DeletedLinkText string `yaml:"deletedLinkText"`
}

// DeletedLink — маркер удалённой внешней ссылки.
const DeletedLink = "DELETED_LINK"

// ReplaceFragment описывает правило замены фрагмента текста.
type ReplaceFragment struct {
	// From — исходный текст.
	From string `yaml:"from"`
	// To — текст замены.
	To string `yaml:"to"`
}
