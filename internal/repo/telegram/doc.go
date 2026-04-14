// Package telegram реализует обёртку над TDLib для работы с Telegram API.
//
// Использование:
//
//	r := telegram.New(cfg, logger)
//	if err := r.Start(ctx); err != nil { ... }
//	defer r.Close()
//
// Конфигурация:
//
//	TELEGRAM_API_ID        — идентификатор приложения Telegram (required)
//	TELEGRAM_API_HASH      — хеш приложения Telegram (required)
//	TELEGRAM_PHONE         — номер телефона для авторизации (required)
//	TELEGRAM_DATABASE_DIR  — путь к директории TDLib (default: .data/tdlib)
//	TELEGRAM_FILES_DIR     — путь к файлам TDLib (default: .data/tdlib-files)
//	TELEGRAM_SYSTEM_LANG   — код языка системы (default: en)
//	TELEGRAM_DEVICE_MODEL  — модель устройства (default: Server)
//	TELEGRAM_USE_TEST_DC   — использовать тестовый DC (default: false)
//	TELEGRAM_LOG_VERBOSITY — уровень логирования TDLib (default: 0)
//
// Ограничения:
//
//   - Текущая реализация — заглушка. Реальная интеграция с TDLib будет добавлена позже.
//   - Start() инициализирует TDLib-клиент.
//   - Close() завершает сессию и освобождает ресурсы.
package telegram
