// Package http реализует HTTP-транспорт с REST и GraphQL эндпоинтами.
//
// Текущая реализация — placeholder; HTTP-сервер пока собирается в cmd/facade
// напрямую через ahttp.NewDefault и controller.EnrichRoutes.
//
// Конфигурация:
//
//	WEBSERVER_HOST — хост HTTP-сервера (из adapters/httpserver/std)
//	WEBSERVER_PORT — порт HTTP-сервера (required, из adapters/httpserver/std)
//
// Ограничения:
//
//   - REST эндпоинты для авторизации будут добавлены при реализации auth flow.
//   - GraphQL эндпоинт будет добавлен при реализации facade.
package http
