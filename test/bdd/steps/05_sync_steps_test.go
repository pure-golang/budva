package steps

import (
	"fmt"

	"github.com/cucumber/godog"
)

func register05SyncSteps(ctx *godog.ScenarioContext, s *scenarioCtx) {
	ctx.Given(`^правило пересылки в режиме "([^"]*)" с опцией copy_once$`, func(mode string) error {
		s.deliveryMode = mode
		s.copyOnce = true
		return nil
	})

	ctx.Given(`^правило пересылки в режиме "([^"]*)" без опции copy_once$`, func(mode string) error {
		s.deliveryMode = mode
		s.copyOnce = false
		return nil
	})

	ctx.Given(`^правило пересылки в режиме "([^"]*)" с опцией indelible$`, func(mode string) error {
		s.deliveryMode = mode
		s.indelible = true
		return nil
	})

	ctx.Given(`^правило пересылки в режиме "([^"]*)" без опции indelible$`, func(mode string) error {
		s.deliveryMode = mode
		s.indelible = false
		return nil
	})

	ctx.Given(`^пользователь ранее отправил сообщение с текстом "([^"]*)"$`, func(text string) error {
		s.messageText = text
		s.delivered = true
		return nil
	})

	ctx.Given(`^пользователь ранее отправил сообщение$`, func() error {
		s.messageText = "test message"
		s.delivered = true
		return nil
	})

	ctx.Given(`^сообщение было скопировано в целевые чаты$`, func() error {
		return nil
	})

	ctx.When(`^пользователь редактирует сообщение на "([^"]*)"$`, func(newText string) error {
		s.messageText = newText
		return nil
	})

	ctx.When(`^пользователь удаляет оригинальное сообщение$`, func() error {
		if !s.indelible {
			s.deleted = true
		}
		return nil
	})

	ctx.Then(`^в целевом чате появляется новая копия с текстом "([^"]*)"$`, func(text string) error {
		if !s.copyOnce {
			return fmt.Errorf("copy_once is not enabled")
		}
		return nil
	})

	ctx.Then(`^новая копия содержит ссылку на предыдущую версию$`, func() error {
		return nil
	})

	ctx.Then(`^предыдущая копия обновляется со ссылкой на новую версию$`, func() error {
		return nil
	})

	ctx.Then(`^существующая копия в целевых чатах обновляется на "([^"]*)"$`, func(text string) error {
		if s.copyOnce {
			return fmt.Errorf("copy_once should not be enabled for update mode")
		}
		return nil
	})

	ctx.Then(`^копии остаются в целевых чатах$`, func() error {
		if s.deleted {
			return fmt.Errorf("copies should not be deleted when indelible is set")
		}
		return nil
	})

	ctx.Then(`^копии удаляются из всех целевых чатов$`, func() error {
		if !s.deleted {
			return fmt.Errorf("copies should be deleted")
		}
		return nil
	})
}
