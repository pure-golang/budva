// Package auth оркестрирует авторизацию Telegram через telegramRepo.
//
// Использование:
//
//	svc := auth.New(telegramRepo)
//	svc.Start(ctx)
//	svc.Subscribe(func(state domain.AuthorizationState, extra any) { ... })
//	svc.InputChan() <- "+79261234567"
//
// Пакет не читает переменные окружения напрямую.
//
// Ограничения:
//
//   - Слушает AuthStates() канал telegramRepo и отправляет пользовательский ввод через SubmitPhone/Code/Password.
//   - Оповещает подписчиков синхронно при изменении состояния.
//   - InputChan() используется для ввода телефона, кода и пароля.
//   - Зависит от telegramRepo через частично применяемый интерфейс.
package auth
