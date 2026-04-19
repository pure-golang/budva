package transform

import (
	"github.com/zelenin/go-tdlib/client"
)

// deepCopyFormattedText делает полную копию *client.FormattedText, не разделяя
// слайс entities с исходником. Transform изменяет entities на месте
// (replaceMyselfLinks / addText), поэтому нужен honest-copy.
func deepCopyFormattedText(ft *client.FormattedText) *client.FormattedText {
	if ft == nil {
		return nil
	}
	result := &client.FormattedText{
		Text:     ft.Text,
		Entities: make([]*client.TextEntity, len(ft.Entities)),
	}
	for i, ent := range ft.Entities {
		if ent == nil {
			continue
		}
		result.Entities[i] = &client.TextEntity{
			Offset: ent.Offset,
			Length: ent.Length,
			Type:   ent.Type,
		}
	}
	return result
}

// entityURL извлекает URL из entity TDLib. Для TextEntityTypeUrl URL — это
// подстрока самого текста; для TextEntityTypeTextUrl URL хранится в Type.
// Возвращает пустую строку, если entity не является URL-entity.
func entityURL(text string, ent *client.TextEntity) string {
	switch t := ent.Type.(type) {
	case *client.TextEntityTypeUrl:
		return extractSubstring(text, ent.Offset, ent.Length)
	case *client.TextEntityTypeTextUrl:
		return t.Url
	default:
		return ""
	}
}

// isURLEntity — true, если entity представляет URL (любого из двух типов TDLib).
func isURLEntity(ent *client.TextEntity) bool {
	switch ent.Type.(type) {
	case *client.TextEntityTypeUrl, *client.TextEntityTypeTextUrl:
		return true
	}
	return false
}
