// Package facade реализует API-фасад для управления Telegram через gRPC и GraphQL.
//
// Использование:
//
//	svc := facade.New(gateway, logger)
//	status, err := svc.GetStatus(ctx)
//	messages, err := svc.GetMessages(ctx, chatID, messageIDs)
//
// Пакет не читает переменные окружения напрямую.
//
// Ограничения:
//
//   - Зависит от Telegram-шлюза через частично применяемый интерфейс gateway.
//   - Один сервис обслуживает оба транспорта (gRPC и GraphQL).
package facade
