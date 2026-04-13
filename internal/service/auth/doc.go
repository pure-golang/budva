// Package auth реализует state machine авторизации Telegram через TDLib.
//
// Использование:
//
//	svc := auth.New(telegramAuth, logger)
//	svc.HandleState(state)
//	svc.SubmitPhone(phone)
//	svc.SubmitCode(code)
//	svc.SubmitPassword(password)
//
// Пакет не читает переменные окружения напрямую.
//
// Ограничения:
//
//   - Зависит от TDLib через частично применяемый интерфейс telegramAuth.
//   - Оповещает подписчиков об изменении состояния авторизации.
package auth
