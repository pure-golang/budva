// Package http реализует HTTP-транспорт с REST и GraphQL эндпоинтами.
//
// Использование:
//
//	srv := http.New(cfg, mux, logger)
//	srv.Run(ctx)
//
// Конфигурация:
//
//	WEBSERVER_HOST — хост HTTP-сервера
//	WEBSERVER_PORT — порт HTTP-сервера (required)
//
// Ограничения:
//
//   - REST эндпоинты для авторизации: /api/auth/telegram/*.
//   - GraphQL эндпоинт: /graphql с playground на /playground.
//   - Health-проверки через controller.EnrichRoutes.
package http
