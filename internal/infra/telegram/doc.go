// Package telegram реализует обёртку над TDLib для работы с Telegram API.
//
// Использование:
//
//	r := telegram.New(cfg)
//	if err := r.Start(ctx); err != nil { ... }
//	defer r.Close()
//	events := r.AuthStates() // канал событий авторизации
//
// Конфигурация:
//
//	TELEGRAM_API_ID            — идентификатор приложения Telegram (required)
//	TELEGRAM_API_HASH          — хеш приложения Telegram (required)
//	TELEGRAM_PHONE             — номер телефона для авторизации (required)
//	TELEGRAM_DATABASE_DIR      — путь к директории TDLib (default: .data/tdlib)
//	TELEGRAM_FILES_DIR         — путь к файлам TDLib (default: .data/tdlib-files)
//	TELEGRAM_SYSTEM_LANG       — код языка системы (default: en)
//	TELEGRAM_DEVICE_MODEL      — модель устройства (default: Server)
//	TELEGRAM_LOG_VERBOSITY     — уровень логирования TDLib (default: 0)
//	TELEGRAM_USE_FILE_DB       — файловый кеш TDLib (default: true)
//	TELEGRAM_USE_CHAT_INFO_DB  — кеш информации о чатах (default: true)
//	TELEGRAM_USE_MESSAGE_DB    — кеш сообщений (default: true)
//	TELEGRAM_USE_SECRET_CHATS  — поддержка секретных чатов (default: false)
//	TELEGRAM_SYSTEM_VERSION    — версия системы (default: "")
//	TELEGRAM_APP_VERSION       — версия приложения (default: 1.0.0)
//	TELEGRAM_LOG_DIR           — директория логов TDLib (default: .data/tdlib-logs)
//	TELEGRAM_LOG_MAX_SIZE      — макс размер лог-файла в MB (default: 10)
//
// Ограничения:
//
//   - Start() инициализирует TDLib-клиент, настраивает логирование и запускает цикл авторизации.
//   - SubmitPhone/SubmitCode/SubmitPassword делегируют ввод в TDLib authorizer.
//   - Close() завершает TDLib-сессию и освобождает ресурсы.
//   - ParseTextEntities/GetMarkdownText — статические вызовы TDLib, работают до авторизации.
//   - GetOption — метод *Repo, обёртка над client.GetOption; доступен до авторизации.
//   - CreateNewSupergroupChat/CreateNewBasicGroupChat/SetSupergroupUsername/DeleteChat — методы для cmd/stand.
//   - SendMessageAndWait блокирует до получения permanent ID (таймаут 60 сек), подписывается через pendingSends; поддерживает retry при FLOOD_WAIT.
//   - Updates() выдаёт отфильтрованные `client.Type`; resolve UpdateMessageEdited через
//     GetMessage — ответственность потребителя (cmd/engine/main.go, internal/test/support/live_stack.go).
package telegram
