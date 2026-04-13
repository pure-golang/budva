// Package handler содержит обработчики обновлений Telegram.
//
// Использование:
//
//	h := handler.New(deps, logger)
//	h.OnNewMessage(ctx, chatID, messageID, text)
//	h.OnEditedMessage(ctx, chatID, messageID, text)
//	h.OnDeletedMessages(ctx, chatID, messageIDs)
//	h.OnMessageSendSucceeded(ctx, chatID, tempID, realID)
//
// Пакет не читает переменные окружения напрямую.
//
// Ограничения:
//
//   - Обработчики вызываются из очереди задач последовательно.
//   - Зависят от сервисов через частично применяемые интерфейсы.
package handler
