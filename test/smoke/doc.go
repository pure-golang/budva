//go:build smoke

// Package smoke содержит smoke-тесты для проверки liveness/readiness собранного стека.
//
// Запуск:
//
//	go test -tags smoke -race -count=1 ./test/smoke/
//	task test:smoke
//
// Тесты поднимают Docker-стек через testcontainers-compose
// и проверяют health-эндпоинты HTTP-сервера.
//
// Ограничения:
//
//   - Требует Docker.
//   - Первый запуск медленный (сборка образа).
package smoke
