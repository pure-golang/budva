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
//	ENVIRONMENT              — окружение: development, staging, production (default: development)
//	TELEGRAM_API_ID          — идентификатор приложения Telegram (required)
//	TELEGRAM_API_HASH        — хеш приложения Telegram (required)
//	TELEGRAM_PHONE           — номер телефона для авторизации (required)
//	TELEGRAM_DATABASE_DIR    — путь к директории TDLib (default: .data/tdlib)
//	TELEGRAM_FILES_DIR       — путь к файлам TDLib (default: .data/tdlib-files)
//	TELEGRAM_SYSTEM_LANG     — код языка системы (default: en)
//	TELEGRAM_DEVICE_MODEL    — модель устройства (default: Server)
//	TELEGRAM_USE_TEST_DC     — использовать тестовый DC (default: false)
//	TELEGRAM_LOG_VERBOSITY   — уровень логирования TDLib (default: 0)
//	RULESET_PATH             — путь к файлу правил пересылки (default: ruleset.yml)
//	STORAGE_PATH             — путь к директории BadgerDB (default: .data/badger)
//	WEBSERVER_HOST           — хост HTTP-сервера (default: "")
//	WEBSERVER_PORT           — порт HTTP-сервера (required)
//	WEBSERVER_TLS_CERT_PATH  — путь к TLS-сертификату HTTP-сервера (default: "")
//	WEBSERVER_TLS_KEY_PATH   — путь к TLS-ключу HTTP-сервера (default: "")
//	WEBSERVER_READ_TIMEOUT   — таймаут чтения HTTP-сервера в секундах (default: 30)
//	GRPC_HOST                — хост gRPC-сервера (default: "")
//	GRPC_PORT                — порт gRPC-сервера (required)
//	GRPC_TLS_CERT_PATH       — путь к TLS-сертификату gRPC-сервера (default: "")
//	GRPC_TLS_KEY_PATH        — путь к TLS-ключу gRPC-сервера (default: "")
//	GRPC_ENABLE_REFLECTION   — включить gRPC reflection API (default: true)
//	LOG_PROVIDER             — провайдер логирования: dev, std_json, noop (default: std_json)
//	LOG_LEVEL                — уровень логирования: debug, info, warn, error (default: info)
//	TRACING_ENDPOINT         — endpoint Jaeger для трейсинга (default: "")
//	SERVICE_NAME             — имя сервиса для трейсинга (default: "")
//	APP_VERSION              — версия приложения для трейсинга (default: "")
//	METRICS_HOST             — хост сервера метрик (default: "")
//	METRICS_PORT             — порт сервера метрик (default: 0)
//	METRICS_READ_TIMEOUT     — таймаут чтения сервера метрик в секундах (default: 30)
//
// Ограничения:
//
//   - Config передаётся по значению.
//   - Мутация глобального состояния через t.Setenv запрещает t.Parallel().
package config
