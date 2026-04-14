// Package facade реализует API-фасад для управления Telegram через gRPC и GraphQL.
//
// Использование:
//
//	svc := facade.New(telegramGateway)
//
// Пакет не читает переменные окружения напрямую.
//
// Ограничения:
//
//   - Один сервис обслуживает оба транспорта (gRPC и GraphQL).
//   - Методы SendMessage и SendMessageAlbum работают только с текстовым контентом;
//     поддержка медиа будет добавлена позже.
package facade
