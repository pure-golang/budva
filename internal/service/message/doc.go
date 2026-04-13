// Package message извлекает и формирует контент сообщений.
//
// Использование:
//
//	svc := message.New(logger)
//	text := svc.ExtractText(messageContent)
//	content := svc.BuildContent(text)
//
// Пакет не читает переменные окружения напрямую.
//
// Ограничения:
//
//   - Работает с FormattedText из TDLib.
//   - Определяет тип контента (текст, фото, видео и т.д.).
package message
