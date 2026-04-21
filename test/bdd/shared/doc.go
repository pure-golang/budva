// Package shared реализует общий runtime BDD-слоя для per-epic пакетов в test/bdd/NN_*.
//
// Использование:
//
//	func Test(t *testing.T) { shared.RunEpic(t) }
//
// Конфигурация:
//
//	BDD_PATHS — список директорий или .feature-файлов через запятую для локального дебага
//	и точечного запуска godog-сценариев (default: test/bdd/<epic>)
//
// Ограничения:
//
//   - Пакет меняет cwd на корень репозитория при первом RunEpic.
//   - Пакет переиспользует один LiveStack на процесс и не предназначен для изолированного lifecycle на каждый сценарий.
//   - Параллельные per-epic бинарники сериализуются через advisory lock на TDLib state.
package shared
