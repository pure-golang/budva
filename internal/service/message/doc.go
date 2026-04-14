// Package message извлекает и формирует контент сообщений.
//
// Использование:
//
//	svc := message.New(logger)
//	text := svc.GetFormattedText(msg)
//	isSystem := svc.IsSystemMessage(msg)
//	data := svc.GetReplyMarkupData(msg)
//	content := svc.BuildInputContent(msg, text)
//
// Пакет не читает переменные окружения напрямую.
//
// Ограничения:
//
//   - Работает с domain.Message и domain.FormattedText.
//   - Определяет тип контента (текст, фото, видео и т.д.).
package message
