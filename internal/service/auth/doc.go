// Package auth реализует state machine авторизации Telegram.
//
// Использование:
//
//	svc := auth.New(logger)
//	svc.Subscribe(func(state domain.AuthorizationState, extra any) { ... })
//	svc.SetState(domain.AuthStateReady, nil)
//	input := svc.ReadInput()
//
// Пакет не читает переменные окружения напрямую.
//
// Ограничения:
//
//   - Оповещает подписчиков синхронно при изменении состояния.
//   - InputChan() используется для ввода телефона, кода и пароля.
package auth
