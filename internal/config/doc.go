// Package config описывает конфигурацию приложения из переменных окружения.
//
// Использование:
//
//	var cfg config.Config
//	if err := aenv.InitConfig(&cfg); err != nil {
//	    return fmt.Errorf("config: %w", err)
//	}
//
// Конфигурация:
//
//	ENVIRONMENT       — окружение: development, staging, production (default: development)
//	TELEGRAM_API_ID   — идентификатор приложения Telegram (required)
//	TELEGRAM_API_HASH — хеш приложения Telegram (required)
//	TELEGRAM_PHONE    — номер телефона для авторизации (required)
//	RULESET_PATH      — путь к файлу правил пересылки (default: ruleset.yml)
//	STORAGE_PATH      — путь к директории BadgerDB (default: .data/badger)
//	WEB_HOST          — хост HTTP-сервера (default: 0.0.0.0)
//	WEB_PORT          — порт HTTP-сервера (default: 7070)
//	GRPC_HOST         — хост gRPC-сервера (default: 0.0.0.0)
//	GRPC_PORT         — порт gRPC-сервера (default: 50051)
//
// Ограничения:
//
//   - Config передаётся по значению.
//   - Мутация глобального состояния через t.Setenv запрещает t.Parallel().
package config
