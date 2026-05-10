// Package handler содержит обработчики обновлений Telegram.
//
// Использование:
//
//	h := handler.New(telegram, state, messages, filters, transform, albums, queue, limiter, newTracker)
//	h.SetRuleSet(rs)
//	go h.Run(ctx)
//
// Для синтетических событий в тестах и BDD:
//
//	h.OnNewMessage(ctx, msg)
//	h.OnEditedMessage(ctx, msg)
//	h.OnDeletedMessages(ctx, chatID, messageIDs, isPermanent)
//	h.OnMessageSendSucceeded(chatID, oldID, newID)
//
// Пакет не читает переменные окружения напрямую.
//
// Ограничения:
//
//   - Run ждёт готовности telegramRepo.ClientDone() и читает telegramRepo.Updates().
//   - Тяжёлые операции ставятся в очередь задач через taskQueue.
//   - RuleSet обновляется атомарно через SetRuleSet.
package handler
