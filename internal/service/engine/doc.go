// Package engine реализует диспетчер обновлений Telegram.
//
// Использование:
//
//	svc := engine.New(deps, logger)
//	svc.HandleNewMessage(ctx, message)
//	svc.HandleEditedMessage(ctx, message)
//	svc.HandleDeletedMessages(ctx, chatID, messageIDs)
//
// Пакет не читает переменные окружения напрямую.
//
// Ограничения:
//
//   - Оркестрирует обработку обновлений через очередь задач.
//   - Зависит от сервисов через частично применяемые интерфейсы.
package engine
