// Package forwarder отправляет и копирует сообщения в целевые чаты.
//
// Использование:
//
//	svc := forwarder.New(sender, stateStore, transformer, logger)
//	err := svc.Forward(ctx, rule, message)
//	err := svc.SendCopy(ctx, rule, message)
//
// Пакет не читает переменные окружения напрямую.
//
// Ограничения:
//
//   - Зависит от sender, stateStore, transformer через частично применяемые интерфейсы.
//   - Обрабатывает медиа-альбомы как группу.
//   - Записывает маппинг temp→real message ID в state.
package forwarder
