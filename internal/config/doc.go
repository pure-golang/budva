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
//	ENVIRONMENT            — окружение: development, staging, production (default: development)
//	TELEGRAM_API_ID        — идентификатор приложения Telegram (required)
//	TELEGRAM_API_HASH      — хеш приложения Telegram (required)
//	TELEGRAM_PHONE         — номер телефона для авторизации (required)
//	TELEGRAM_DATABASE_DIR  — путь к директории TDLib (default: .data/tdlib)
//	TELEGRAM_FILES_DIR     — путь к файлам TDLib (default: .data/tdlib-files)
//	TELEGRAM_SYSTEM_LANG   — код языка системы (default: en)
//	TELEGRAM_DEVICE_MODEL  — модель устройства (default: Server)
//	TELEGRAM_USE_TEST_DC   — использовать тестовый DC (default: false)
//	TELEGRAM_LOG_VERBOSITY — уровень логирования TDLib (default: 0)
//	RULESET_PATH           — путь к файлу правил пересылки (default: ruleset.yml)
//	STORAGE_PATH           — путь к директории BadgerDB (default: .data/badger)
//	WEBSERVER_HOST         — хост HTTP-сервера (из adapters/httpserver/std)
//	WEBSERVER_PORT         — порт HTTP-сервера (required, из adapters/httpserver/std)
//	GRPC_HOST              — хост gRPC-сервера (из adapters/grpc/std)
//	GRPC_PORT              — порт gRPC-сервера (required, из adapters/grpc/std)
//
// Ограничения:
//
//   - Config передаётся по значению.
//   - Мутация глобального состояния через t.Setenv запрещает t.Parallel().
package config
