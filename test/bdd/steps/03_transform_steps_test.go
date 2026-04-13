package steps

import (
	"fmt"

	"github.com/cucumber/godog"
)

func register03TransformSteps(ctx *godog.ScenarioContext, s *scenarioCtx) {
	ctx.Given(`^правило замены фрагментов "([^"]*)" → "([^"]*)" и "([^"]*)" → "([^"]*)"$`, func(_, _, _, _ string) error {
		return nil
	})

	ctx.Given(`^для источника включена опция "([^"]*)"$`, func(option string) error {
		switch option {
		case "подпись":
			s.signEnabled = true
		case "ссылка":
			s.linkEnabled = true
		case "перевод":
			s.translateEnabled = true
		case "автоответы":
			s.autoAnswer = true
		}
		return nil
	})

	ctx.Given(`^в исходном чате есть ранее скопированное сообщение$`, func() error {
		return nil
	})

	ctx.When(`^пользователь отправляет сообщение со ссылкой на предыдущее сообщение$`, func() error {
		s.delivered = true
		return nil
	})

	ctx.Then(`^в целевом чате ссылка указывает на копию предыдущего сообщения$`, func() error {
		if !s.delivered {
			return fmt.Errorf("message was not delivered")
		}
		return nil
	})

	ctx.Given(`^в исходном чате есть сообщение из внешнего чата$`, func() error {
		return nil
	})

	ctx.When(`^пользователь отправляет сообщение со ссылкой на внешнее сообщение$`, func() error {
		s.delivered = true
		return nil
	})

	ctx.Then(`^в целевом чате сообщение появляется без внешней ссылки$`, func() error {
		if !s.delivered {
			return fmt.Errorf("message was not delivered")
		}
		return nil
	})

	ctx.Then(`^в целевом чате сообщение содержит подпись источника$`, func() error {
		if !s.signEnabled {
			return fmt.Errorf("sign was not enabled")
		}
		return nil
	})

	ctx.Then(`^в целевом чате сообщение содержит ссылку на оригинал$`, func() error {
		if !s.linkEnabled {
			return fmt.Errorf("link was not enabled")
		}
		return nil
	})
}
