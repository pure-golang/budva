package shared

import (
	"context"
	"fmt"
	"strings"

	"github.com/cucumber/godog"

	"github.com/pure-golang/budva-claude/internal/domain"
)

// RegisterAllSteps регистрирует шаги всех эпиков и общие Given-шаги.
// Используется каждым per-epic initScenario, поскольку feature-файлы свободно
// переиспользуют шаги между эпиками (например, «правило пересылки в режиме»
// из 01_delivery встречается в 02..05).
func RegisterAllSteps(ctx *godog.ScenarioContext, s *State) {
	RegisterDeliverySteps(ctx, s)
	RegisterFiltersSteps(ctx, s)
	RegisterTransformSteps(ctx, s)
	RegisterMediaSteps(ctx, s)
	RegisterSyncSteps(ctx, s)
	RegisterAutoSteps(ctx, s)
	RegisterCommonSteps(ctx, s)
}

func setDeliveryMode(s *State, mode string) {
	s.DeliveryMode = mode
	s.SendCopy = (mode == "копия")
}

func enableSourceOption(s *State, option string) {
	switch option {
	case "подпись":
		s.Src.Sign = &domain.Sign{
			Title: domain.SignTitle,
			For:   s.Env.TargetIDs,
		}
	case "ссылка":
		s.Src.Link = &domain.Link{
			Title: domain.LinkTitle,
			For:   s.Env.TargetIDs,
		}
	case "перевод":
		s.Src.Translate = &domain.Translate{
			Lang: "ru",
			For:  s.Env.TargetIDs,
		}
	case "автоответы":
		s.Src.AutoAnswer = true
	}
}

func sendSourceMessage(s *State, text string) error {
	s.ApplyRuleSet()

	if text == "" {
		text = "test message"
	}

	s.MessageText = text

	msg, err := s.Env.PutMessage(s.Env.SourceID, TextContent(text), s.Prefix)
	if err != nil {
		return err
	}
	s.SentMsg = msg

	s.Env.Handler.OnNewMessage(context.Background(), msg)
	s.Env.DrainQueue()

	return nil
}

func assertMessageInAllTargets(s *State) error {
	for _, targetID := range s.Env.TargetIDs {
		if _, err := s.Env.CheckLastMessage(targetID, s.Prefix); err != nil {
			return err
		}
	}
	return nil
}

func assertNoMessageInTargets(s *State) error {
	for _, targetID := range s.Env.TargetIDs {
		if err := s.Env.CheckNoMessage(targetID, s.Prefix); err != nil {
			return err
		}
	}
	return nil
}

func assertMessageTextInAllTargets(s *State, expected string) error {
	for _, targetID := range s.Env.TargetIDs {
		msg, err := s.Env.CheckLastMessage(targetID, s.Prefix)
		if err != nil {
			return err
		}
		caption := MessageCaption(msg)
		if caption == nil || !strings.Contains(caption.Text, expected) {
			return fmt.Errorf("no message containing text %q in target chat %d", expected, targetID)
		}
	}
	return nil
}

func waitForCopyMappings(s *State) error {
	if err := assertMessageInAllTargets(s); err != nil {
		return err
	}

	// processUpdates записывает temp→perm маппинг напрямую в state;
	// DrainQueue обрабатывает оставшиеся handler tasks.
	s.Env.DrainQueue()

	// Ждём, что processUpdates успел записать temp→perm mapping для всех копий:
	// без этого handler.deleteMessages на последующем шаге увидит GetNewMessageID=0
	// и не сможет разослать каскадное удаление.
	if s.SentMsg != nil {
		if err := s.Env.WaitForCopyMappings(s.Env.SourceID, s.SentMsg.Id); err != nil {
			return err
		}
	}

	return nil
}

// RegisterCommonSteps регистрирует общие шаги, которые используются в нескольких
// эпиках и не принадлежат одному конкретному steps_NN_*.go.
func RegisterCommonSteps(ctx *godog.ScenarioContext, s *State) {
	ctx.Given(`^исходный чат типа "([^"]*)"$`, func(srcType string) error {
		s.SourceType = srcType
		fix, err := s.Env.ChatByName(srcType)
		if err != nil {
			return err
		}
		s.Env.SourceID = fix.ChatID
		s.Src.ChatID = fix.ChatID
		return nil
	})

	ctx.Given(`^целевой чат типа "([^"]*)"$`, func(dstType string) error {
		fix, err := s.Env.ChatByName(dstType)
		if err != nil {
			return err
		}
		s.Env.TargetIDs = []int64{fix.ChatID}
		return nil
	})

	ctx.Given(`^правило пересылки в режиме "([^"]*)"$`, func(mode string) error {
		setDeliveryMode(s, mode)
		return nil
	})

	ctx.Given(`^для источника включена опция "([^"]*)"$`, func(option string) error {
		enableSourceOption(s, option)
		return nil
	})

	ctx.Given(`^пользователь ранее отправил сообщение с текстом "([^"]*)"$`, func(text string) error {
		return sendSourceMessage(s, text)
	})

	ctx.Given(`^пользователь ранее отправил сообщение$`, func() error {
		return sendSourceMessage(s, "")
	})

	ctx.Given(`^сообщение было скопировано в целевые чаты$`, func() error {
		return waitForCopyMappings(s)
	})

	ctx.When(`^пользователь отправляет сообщение в исходный чат$`, func() error {
		return sendSourceMessage(s, s.MessageText)
	})

	ctx.When(`^пользователь отправляет сообщение с текстом "([^"]*)"$`, func(text string) error {
		return sendSourceMessage(s, text)
	})

	ctx.Then(`^сообщение появляется во всех целевых чатах$`, func() error {
		return assertMessageInAllTargets(s)
	})

	ctx.Then(`^сообщение не появляется в целевых чатах$`, func() error {
		return assertNoMessageInTargets(s)
	})

	ctx.Then(`^сообщение с текстом "([^"]*)" появляется во всех целевых чатах$`, func(expected string) error {
		return assertMessageTextInAllTargets(s, expected)
	})
}
