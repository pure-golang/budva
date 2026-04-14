// Package dedup отслеживает, в какие чаты уже отправлено сообщение.
//
// Использование:
//
//	tracker := dedup.NewTracker(destinationIDs)
//	sent := tracker.TryMark(chatID)
//
// Пакет не читает переменные окружения напрямую.
//
// Ограничения:
//
//   - Tracker потокобезопасен.
//   - Один Tracker на одно исходное сообщение.
package dedup
