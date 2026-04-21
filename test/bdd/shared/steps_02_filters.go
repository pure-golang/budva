package shared

import "github.com/cucumber/godog"

// RegisterFiltersSteps регистрирует шаги эпика 02_filters.
func RegisterFiltersSteps(ctx *godog.ScenarioContext, s *State) {
	ctx.Given(`^фильтр исключения с паттерном "([^"]*)"$`, func(pattern string) error {
		s.ExcludePattern = pattern
		return nil
	})

	ctx.Given(`^фильтр включения с паттерном "([^"]*)"$`, func(pattern string) error {
		s.IncludePattern = pattern
		return nil
	})

	ctx.Given(`^фильтр submatch с паттерном "([^"]*)"$`, func(pattern string) error {
		s.SubmatchPattern = pattern
		return nil
	})

	ctx.When(`^пользователь отправляет сообщение без запрещённого паттерна$`, func() error {
		return sendSourceMessage(s, "normal text")
	})

	// --- Check/Other dedup ---

	ctx.Given(`^назначен check-чат для отклонённых сообщений$`, func() error {
		if len(s.Env.Fixtures.Chats) > 2 {
			s.CheckChatID = s.Env.Fixtures.Chats[2].ChatID
		} else {
			s.CheckChatID = -1004000
		}
		return nil
	})

	ctx.Then(`^сообщение появляется в check-чате ровно один раз$`, func() error {
		if _, err := s.Env.CheckLastMessage(s.CheckChatID, s.Prefix); err != nil {
			return err
		}
		return nil
	})
}
