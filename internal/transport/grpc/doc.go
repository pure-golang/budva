// Package grpc реализует gRPC-транспорт для фасада.
//
// Использование:
//
//	srv := grpc.New(cfg, facadeService, logger)
//	srv.Run(ctx)
//
// Конфигурация:
//
//	GRPC_HOST             — хост gRPC-сервера
//	GRPC_PORT             — порт gRPC-сервера (required)
//	GRPC_ENABLE_REFLECTION — включить gRPC reflection (default: true)
//
// Ограничения:
//
//   - Реализует proto-сервис FacadeGRPC.
//   - Reflection включён по умолчанию для поддержки grpcurl.
package grpc
