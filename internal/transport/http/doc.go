// Package http реализует HTTP-транспорт с REST-эндпоинтами для авторизации.
//
// Использование:
//
//	tr := http.New(authSvc)
//	tr.EnrichRoutes(mux)
//
// Эндпоинты:
//
//	GET  /api/auth/telegram/state    — текущее состояние авторизации
//	POST /api/auth/telegram/phone    — отправка номера телефона
//	POST /api/auth/telegram/code     — отправка кода подтверждения
//	POST /api/auth/telegram/password — отправка пароля 2FA
//
// Конфигурация:
//
//	WEBSERVER_HOST — хост HTTP-сервера (из adapters/httpserver/std)
//	WEBSERVER_PORT — порт HTTP-сервера (required, из adapters/httpserver/std)
//
// Ограничения:
//
//   - GraphQL эндпоинт будет добавлен при реализации facade service.
package http
