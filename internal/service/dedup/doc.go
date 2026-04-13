// Package dedup отслеживает, в какие чаты уже отправлено сообщение.
//
// Использование:
//
//	svc := dedup.New()
//	tracker := svc.NewTracker(destinationIDs)
//	sent := tracker.TryMark(chatID)
//
// Пакет не читает переменные окружения напрямую.
//
// Ограничения:
//
//   - Tracker потокобезопасен.
//   - Один Tracker на одно исходное сообщение.
package dedup
