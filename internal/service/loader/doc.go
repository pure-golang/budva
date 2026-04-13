// Package loader загружает конфигурацию правил и прогревает чаты при авторизации.
//
// Использование:
//
//	svc := loader.New(rulesetLoader, chatWarmer, logger)
//	rs, err := svc.Load(ctx)
//
// Пакет не читает переменные окружения напрямую.
//
// Ограничения:
//
//   - Вызывается после успешной авторизации.
//   - Загружает ruleset и прогревает чаты через GetChatHistory.
//   - Повторно вызывается при hot-reload ruleset.yml.
package loader
