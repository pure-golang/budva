package shared

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/cucumber/godog"
	"github.com/zelenin/go-tdlib/client"

	"github.com/pure-golang/budva-claude/internal/domain"
	testsupport "github.com/pure-golang/budva-claude/internal/test/support"
)

// RegisterSyncSteps регистрирует шаги эпика 05_sync.
func RegisterSyncSteps(ctx *godog.ScenarioContext, s *ScenarioCtx) {
	ctx.Given(`^правило пересылки в режиме "([^"]*)" с опцией copy_once$`, func(mode string) error {
		s.DeliveryMode = mode
		s.SendCopy = (mode == "копия")
		s.CopyOnce = true
		s.Src.Prev = &domain.Prev{
			Title: domain.PrevTitle,
			For:   s.Env.TargetIDs,
		}
		s.Src.Next = &domain.Next{
			Title: domain.NextTitle,
			For:   s.Env.TargetIDs,
		}
		return nil
	})

	ctx.Given(`^правило пересылки в режиме "([^"]*)" без опции copy_once$`, func(mode string) error {
		s.DeliveryMode = mode
		s.SendCopy = (mode == "копия")
		s.CopyOnce = false
		return nil
	})

	ctx.Given(`^правило пересылки в режиме "([^"]*)" с опцией indelible$`, func(mode string) error {
		s.DeliveryMode = mode
		s.SendCopy = (mode == "копия")
		s.Indelible = true
		return nil
	})

	ctx.Given(`^правило пересылки в режиме "([^"]*)" без опции indelible$`, func(mode string) error {
		s.DeliveryMode = mode
		s.SendCopy = (mode == "копия")
		s.Indelible = false
		return nil
	})

	ctx.Given(`^пользователь ранее отправил сообщение с текстом "([^"]*)"$`, func(text string) error {
		s.ApplyRuleSet()

		s.MessageText = text
		msg, err := s.Env.PutMessage(s.Env.SourceID, TextContent(text), s.Prefix)
		if err != nil {
			return err
		}
		s.SentMsg = msg

		s.Env.Handler.OnNewMessage(context.Background(), msg)
		s.Env.DrainQueue()

		return nil
	})

	ctx.Given(`^пользователь ранее отправил сообщение$`, func() error {
		s.ApplyRuleSet()

		s.MessageText = "test message"
		msg, err := s.Env.PutMessage(s.Env.SourceID, TextContent(s.MessageText), s.Prefix)
		if err != nil {
			return err
		}
		s.SentMsg = msg

		s.Env.Handler.OnNewMessage(context.Background(), msg)
		s.Env.DrainQueue()

		return nil
	})

	ctx.Given(`^сообщение было скопировано в целевые чаты$`, func() error {
		for _, targetID := range s.Env.TargetIDs {
			if _, err := s.Env.CheckLastMessage(targetID, s.Prefix); err != nil {
				return err
			}
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
	})

	ctx.When(`^пользователь редактирует сообщение на "([^"]*)"$`, func(newText string) error {
		if s.SentMsg == nil {
			return fmt.Errorf("no previously sent message")
		}

		// Реально редактируем SOURCE сообщение через TDLib.
		editedText := &client.FormattedText{
			Text: testsupport.PrefixText(s.Prefix, newText),
		}
		if _, err := s.Env.Telegram.EditMessageText(&client.EditMessageTextRequest{
			ChatId:              s.Env.SourceID,
			MessageId:           s.SentMsg.Id,
			InputMessageContent: &client.InputMessageText{Text: editedText},
		}); err != nil {
			return fmt.Errorf("edit source message: %w", err)
		}

		// Поллим: ждём применения edit.
		// Для copy_once (versioning) transform добавляет Markdown-ссылку `[Prev](url)`;
		// TDLib ParseTextEntities выносит URL в TextEntityTypeTextUrl, оставляя в plain text
		// только заголовок. Поэтому проверяем entities, а не Contains(text, "https://t.me").
		// basicGroup-таргеты не поддерживают GetMessageLink — для них ждём только обновление
		// текста без ссылки.
		deadline := time.After(10 * time.Second)
		expectLink := s.CopyOnce
		if expectLink {
			anySupportsLink := false
			for _, targetID := range s.Env.TargetIDs {
				if s.Env.SupportsMessageLink(targetID) {
					anySupportsLink = true
					break
				}
			}
			if !anySupportsLink {
				expectLink = false
			}
		}
	editPoll:
		for {
			s.Env.DrainQueue()
			for _, targetID := range s.Env.TargetIDs {
				msg, err := s.Env.CheckLastMessage(targetID, s.Prefix)
				if err != nil {
					continue
				}
				text := MessageCaption(msg)
				if text == nil {
					continue
				}
				if expectLink && HasTMeEntity(text) {
					break editPoll
				}
				if !expectLink && strings.Contains(text.Text, newText) {
					break editPoll
				}
			}
			select {
			case <-deadline:
				if expectLink {
					return fmt.Errorf("versioning: link not found in target within 10s")
				}
				break editPoll
			case <-time.After(200 * time.Millisecond):
			}
		}

		s.MessageText = newText
		return nil
	})

	ctx.When(`^пользователь удаляет оригинальное сообщение$`, func() error {
		if s.SentMsg == nil {
			return fmt.Errorf("no previously sent message")
		}

		s.Env.Handler.OnDeletedMessages(
			context.Background(),
			s.Env.SourceID,
			[]int64{s.SentMsg.Id},
			true,
		)
		if s.SkipRetryDrain {
			s.Env.Queue.ProcessBatch()
		} else {
			s.Env.DrainQueue()
		}

		return nil
	})

	ctx.Then(`^в целевом чате появляется новая копия с текстом "([^"]*)"$`, func(text string) error {
		for _, targetID := range s.Env.TargetIDs {
			msg, err := s.Env.CheckLastMessage(targetID, s.Prefix)
			if err != nil {
				return err
			}
			caption := MessageCaption(msg)
			if caption == nil || !strings.Contains(caption.Text, text) {
				return fmt.Errorf("no message containing text %q in target chat %d", text, targetID)
			}
		}
		return nil
	})

	ctx.Then(`^новая копия содержит ссылку на предыдущую версию$`, func() error {
		for _, targetID := range s.Env.TargetIDs {
			// Навигационные ссылки работают только в supergroups/channels;
			// basicGroup пропускаем по бизнес-правилу из feature-файла.
			if !s.Env.SupportsMessageLink(targetID) {
				continue
			}
			msg, err := s.Env.CheckLastMessage(targetID, s.Prefix)
			if err != nil {
				return err
			}
			caption := MessageCaption(msg)
			if caption == nil || !HasTMeEntity(caption) {
				return fmt.Errorf("no message with link to previous version in target chat %d", targetID)
			}
		}
		return nil
	})

	ctx.Then(`^предыдущая копия обновляется со ссылкой на новую версию$`, func() error {
		for _, targetID := range s.Env.TargetIDs {
			if _, err := s.Env.CheckLastMessage(targetID, s.Prefix); err != nil {
				return err
			}
		}
		return nil
	})

	ctx.Then(`^существующая копия в целевых чатах обновляется на "([^"]*)"$`, func(text string) error {
		for _, targetID := range s.Env.TargetIDs {
			msg, err := s.Env.CheckLastMessage(targetID, s.Prefix)
			if err != nil {
				return err
			}
			caption := MessageCaption(msg)
			if caption == nil || !strings.Contains(caption.Text, text) {
				return fmt.Errorf("no message with text %q in target chat %d", text, targetID)
			}
		}
		return nil
	})

	ctx.Then(`^копии остаются в целевых чатах$`, func() error {
		for _, targetID := range s.Env.TargetIDs {
			if _, err := s.Env.CheckLastMessage(targetID, s.Prefix); err != nil {
				return err
			}
		}
		return nil
	})

	ctx.Then(`^копии удаляются из всех целевых чатов$`, func() error {
		for _, targetID := range s.Env.TargetIDs {
			if err := s.Env.CheckNoMessage(targetID, s.Prefix); err != nil {
				return err
			}
		}
		return nil
	})

	// --- Retry eventual consistency ---
	// Маппинг temp↔permanent в state хранится как "newMsgId:chatID:tmpID → permID".
	// Чтобы детерминированно симулировать «permanent ещё не записан», берём tmpID
	// через обратный индекс "tmpMsgId:chatID:permID → tmpID" и удаляем исходный ключ.

	ctx.Given(`^permanent ID ещё не записан в хранилище$`, func() error {
		for _, targetID := range s.Env.TargetIDs {
			msg, err := s.Env.CheckLastMessage(targetID, s.Prefix)
			if err != nil {
				continue
			}
			tmpID := s.Env.State.GetTmpMessageID(msg.ChatId, msg.Id)
			if tmpID == 0 {
				return fmt.Errorf("tmp ID mapping missing for perm %d in chat %d", msg.Id, msg.ChatId)
			}
			s.TmpIDByTarget[targetID] = tmpID
			s.PermIDByTarget[targetID] = msg.Id
			if err := s.Env.State.Delete(fmt.Sprintf("newMsgId:%d:%d", msg.ChatId, tmpID)); err != nil {
				return err
			}
		}
		s.SkipRetryDrain = true
		return nil
	})

	ctx.When(`^permanent ID записывается в хранилище$`, func() error {
		for targetID, tmpID := range s.TmpIDByTarget {
			permID := s.PermIDByTarget[targetID]
			if err := s.Env.State.Set(fmt.Sprintf("newMsgId:%d:%d", targetID, tmpID), fmt.Sprintf("%d", permID)); err != nil {
				return err
			}
		}
		return nil
	})

	ctx.Then(`^после повторной попытки копии удаляются из целевых чатов$`, func() error {
		s.Env.DrainQueue()

		for _, targetID := range s.Env.TargetIDs {
			if err := s.Env.CheckNoMessage(targetID, s.Prefix); err != nil {
				return err
			}
		}
		return nil
	})
}
