// Package message извлекает и формирует контент сообщений.
//
// Использование:
//
//	svc := message.New()
//	text := svc.GetFormattedText(msg)
//	isSystem := svc.IsSystemMessage(msg)
//	data := svc.GetReplyMarkupData(msg)
//	content := svc.BuildInputContent(msg, text)
//
// Пакет не читает переменные окружения напрямую.
//
// Ограничения:
//
//   - Работает напрямую с raw-TDLib типами (`*client.Message`, `*client.FormattedText`,
//     `client.InputMessageContent`). Доменных дублей не создаётся (см. x-tdlib).
//   - Type-switch по `client.MessageContent` разделяет поддерживаемые типы контента
//     (text, photo, video, document, audio, animation, voice note); остальные
//     (sticker, location, chat events) считаются системными и игнорируются.
package message
