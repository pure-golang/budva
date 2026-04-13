// Package controller реализует HTTP-эндпоинты для health/ready/live проверок.
//
// Использование:
//
//	ctrl := controller.New(pingers...)
//	ctrl.EnrichRoutes(mux)
//
// Пакет не читает переменные окружения напрямую.
//
// Ограничения:
//
//   - /ready проверяет все pingers с таймаутом 3 секунды.
//   - /live всегда возвращает 200.
//   - /healthcheck возвращает 200 если все pingers доступны.
package controller
