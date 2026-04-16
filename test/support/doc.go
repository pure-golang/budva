// Package support предоставляет общие тестовые хелперы для integration, bdd и e2e слоёв.
//
// Основные компоненты:
//
//   - LiveStack — собранный стек handler + services + реальный TDLib + real BadgerDB.
//   - Fixtures / ChatFixture — загрузка и маппинг тестовых чатов из .config/stand.json.
//
// Ограничения:
//
//   - LiveStack требует: TDLib собран, .env с credentials, cmd/stand --up выполнен.
//   - LiveStack создаёт временную директорию и BadgerDB; вызов Close() обязателен для освобождения ресурсов.
//   - Fixtures загружаются из JSON; ChatByName работает после LoadFixtures или SaveFixtures.
package support
