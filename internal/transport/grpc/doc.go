// Package grpc реализует gRPC-транспорт для фасада.
//
// Текущая реализация — placeholder; gRPC-сервер будет добавлен
// при реализации protobuf-определений и facade service.
//
// Конфигурация:
//
//	GRPC_HOST              — хост gRPC-сервера (из adapters/grpc/std)
//	GRPC_PORT              — порт gRPC-сервера (required, из adapters/grpc/std)
//	GRPC_ENABLE_REFLECTION — включить gRPC reflection (default: true, из adapters/grpc/std)
//
// Ограничения:
//
//   - Реализует proto-сервис FacadeGRPC (определение в будущем).
//   - Reflection включён по умолчанию для поддержки grpcurl.
package grpc
