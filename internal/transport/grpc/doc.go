// Package grpc реализует gRPC-транспорт для фасада.
//
// Реализует proto-сервис FacadeGRPC с методами:
//
//   - GetMessages, GetChatHistory, GetMessage
//   - SendMessage, SendMessageAlbum, ForwardMessage
//   - UpdateMessage, DeleteMessages
//   - GetMessageLink, GetMessageLinkInfo
//
// Конфигурация:
//
//	GRPC_HOST              — хост gRPC-сервера (из adapters/grpc/std)
//	GRPC_PORT              — порт gRPC-сервера (required, из adapters/grpc/std)
//	GRPC_ENABLE_REFLECTION — включить gRPC reflection (default: true, из adapters/grpc/std)
//
// Ограничения:
//
//   - GetChatHistory не реализован (Unimplemented).
//   - Reflection включён по умолчанию для поддержки grpcurl.
package grpc
