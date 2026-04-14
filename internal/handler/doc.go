// Package handler содержит обработчики обновлений Telegram.
//
// Использование:
//
//	h := handler.New(telegram, state, messages, filters, transform, albums, queue, logger)
//	h.SetRuleSet(rs)
//	h.OnNewMessage(ctx, msg)
//	h.OnEditedMessage(ctx, msg)
//	h.OnDeletedMessages(ctx, chatID, messageIDs, isPermanent)
//	h.OnMessageSendSucceeded(chatID, oldID, newID)
//
// Пакет не читает переменные окружения напрямую.
//
// Ограничения:
//
//   - Обработчики вызываются из update dispatcher в cmd/engine.
//   - Тяжёлые операции ставятся в очередь задач через taskQueue.
//   - RuleSet обновляется атомарно через SetRuleSet.
package handler
