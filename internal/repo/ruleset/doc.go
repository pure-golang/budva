// Package ruleset загружает и отслеживает YAML-файл правил пересылки.
//
// Использование:
//
//	r := ruleset.New(cfg, logger)
//	rs, err := r.Load()
//	r.WatchContext(ctx, func() { /* reload callback */ })
//	defer r.Close()
//
// Конфигурация:
//
//	RULESET_PATH — путь к YAML-файлу с правилами пересылки (default: ruleset.yml)
//
// Ограничения:
//
//   - Load() читает и валидирует файл, возвращает domain.RuleSet.
//   - WatchContext() запускает fsnotify-наблюдатель за файлом.
//   - Close() останавливает наблюдатель.
//   - Идентификаторы чатов в YAML записываются положительными, transform
//     инвертирует их (Telegram использует отрицательные ID для групп и каналов).
package ruleset
