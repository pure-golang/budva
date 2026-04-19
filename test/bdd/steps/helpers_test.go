//go:build bdd

package steps

import (
	"strings"

	"github.com/zelenin/go-tdlib/client"
)

// textContent — короткий конструктор InputMessageText для BDD-шагов.
func textContent(text string) client.InputMessageContent {
	return &client.InputMessageText{Text: &client.FormattedText{Text: text}}
}

// messageCaption возвращает Text/Caption из Content независимо от типа сообщения.
// Используется в BDD-шагах для проверки, что сообщение не пустое/совпадает.
func messageCaption(msg *client.Message) *client.FormattedText {
	if msg == nil || msg.Content == nil {
		return nil
	}
	switch c := msg.Content.(type) {
	case *client.MessageText:
		return c.Text
	case *client.MessagePhoto:
		return c.Caption
	case *client.MessageVideo:
		return c.Caption
	case *client.MessageDocument:
		return c.Caption
	case *client.MessageAudio:
		return c.Caption
	case *client.MessageAnimation:
		return c.Caption
	}
	return nil
}

// hasTMeEntity проверяет, содержит ли FormattedText TextEntityTypeTextUrl с URL,
// начинающимся на https://t.me/. TDLib ParseTextEntities выносит URL из Markdown
// `[title](url)` в entities, оставляя в plain text только title.
func hasTMeEntity(ft *client.FormattedText) bool {
	if ft == nil {
		return false
	}
	for _, e := range ft.Entities {
		if tu, ok := e.Type.(*client.TextEntityTypeTextUrl); ok && strings.HasPrefix(tu.Url, "https://t.me") {
			return true
		}
	}
	return false
}
