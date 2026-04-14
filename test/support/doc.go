// Package support предоставляет общие тестовые хелперы для integration, bdd и e2e слоёв.
//
// Основные компоненты:
//
//   - FakeTelegram — in-memory реализация telegram gateway для тестов.
//   - Stack — собранный стек handler + services + fake telegram + real BadgerDB.
//
// Ограничения:
//
//   - Stack создаёт временную директорию и BadgerDB; вызов Close() обязателен для освобождения ресурсов.
package support
