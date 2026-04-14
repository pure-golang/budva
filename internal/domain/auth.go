package domain

// AuthorizationState описывает состояние авторизации в Telegram.
type AuthorizationState int

const (
	// AuthStateWaitPhone — ожидание ввода номера телефона.
	AuthStateWaitPhone AuthorizationState = iota
	// AuthStateWaitCode — ожидание ввода кода подтверждения.
	AuthStateWaitCode
	// AuthStateWaitPassword — ожидание ввода пароля двухфакторной аутентификации.
	AuthStateWaitPassword
	// AuthStateReady — авторизация завершена.
	AuthStateReady
	// AuthStateClosing — клиент закрывается.
	AuthStateClosing
	// AuthStateClosed — клиент закрыт.
	AuthStateClosed
)

// String возвращает строковое представление состояния авторизации.
func (s AuthorizationState) String() string {
	switch s {
	case AuthStateWaitPhone:
		return "waitPhone"
	case AuthStateWaitCode:
		return "waitCode"
	case AuthStateWaitPassword:
		return "waitPassword"
	case AuthStateReady:
		return "ready"
	case AuthStateClosing:
		return "closing"
	case AuthStateClosed:
		return "closed"
	default:
		return "unknown"
	}
}

// WaitPasswordState содержит дополнительную информацию о состоянии ожидания пароля.
type WaitPasswordState struct {
	// PasswordHint — подсказка для пароля.
	PasswordHint string
}
