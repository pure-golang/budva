// Package support предоставляет общие тестовые хелперы для integration, bdd и e2e слоёв.
//
// Основные компоненты:
//
//   - LiveStack — собранный стек handler + services + реальный TDLib + real BadgerDB.
//   - Fixtures / ChatFixture — загрузка и маппинг тестовых чатов из .config/stand.json.
//
// Ограничения:
//
//   - NewLiveStack(fixturesPath) создаёт экземпляр; Start() инициализирует TDLib и handler pipeline.
//   - Start() создаёт long-lived context, запускает processUpdates горутину для drain updates.
//   - Close() обязателен: освобождает ресурсы, останавливает горутины, канселит context.
//   - Fixtures загружаются из JSON; ChatByName работает после LoadFixtures или SaveFixtures.
package support
