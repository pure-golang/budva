// Package facade реализует API-фасад для управления Telegram через gRPC и GraphQL.
//
// Использование:
//
//	svc := facade.New(telegramRepo)
//
// Пакет не читает переменные окружения напрямую.
//
// Ограничения:
//
//   - Один сервис обслуживает оба транспорта (gRPC и GraphQL).
//   - SendMessageAlbum маршрутизирует медиа через inputMessageByFileExt: расширение
//     файла определяет тип (photo/video/audio/animation/document), без FilePath
//     — plain text.
//   - Возвращаемые типы — raw TDLib (`*client.Message`, `*client.MessageLinkInfo`);
//     конвертация в proto/GraphQL DTO живёт в transport-слое.
package facade
