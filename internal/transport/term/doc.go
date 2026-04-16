// Package term реализует терминальный транспорт для авторизации и CLI-команд.
//
// Использование:
//
//	t := term.New(authService, telegramRepo, termIO, phoneNumber)
//	t.Run(ctx, shutdownFunc)
//	defer t.Close()
//
// Пакет не читает переменные окружения напрямую.
//
// Ограничения:
//
//   - Интерактивный ввод: телефон, код подтверждения, пароль.
//   - Блокирует горутину до завершения контекста.
//   - Зависит от authService, telegramRepo и termIO через частично применяемые интерфейсы.
package term
