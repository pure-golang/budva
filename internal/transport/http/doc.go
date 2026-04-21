// Package http реализует HTTP-транспорт с REST-эндпоинтами для авторизации.
//
// Использование:
//
//	tr := http.New(authService, resolver)
//	tr.EnrichRoutes(mux)
//
// Эндпоинты:
//
//	GET  /api/auth/telegram/state    — текущее состояние авторизации (включает password_hint в состоянии waitPassword)
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
//   - GraphQL-маршруты `/graphql` и `/playground` добавляются только при ненулевом resolver.
package http
