package steps

import (
	"context"
	"fmt"

	"github.com/cucumber/godog"

	"github.com/pure-golang/budva-claude/internal/domain"
)

func register04MediaSteps(ctx *godog.ScenarioContext, s *scenarioCtx) {
	ctx.When(`^пользователь отправляет медиа-альбом в исходный чат$`, func() error {
		s.applyRuleSet()

		albumID := int64(12345)

		// Создаём 3 сообщения в альбоме
		for i := int64(1); i <= 3; i++ {
			msg := &domain.Message{
				ChatID:       s.env.SourceID,
				ID:           100 + i,
				CanBeSaved:   true,
				MediaAlbumID: albumID,
				Content: domain.MessageContent{
					Type: domain.ContentPhoto,
					Text: &domain.FormattedText{Text: fmt.Sprintf("photo %d", i)},
				},
			}
			s.env.TelegramFake.PutMessage(msg)
			s.env.Handler.OnNewMessage(context.Background(), msg)
		}

		s.env.DrainQueue()

		return nil
	})

	ctx.Then(`^медиа-альбом появляется во всех целевых чатах$`, func() error {
		for _, targetID := range s.env.TargetIDs {
			msgs := s.env.TelegramFake.MessagesInChat(targetID)
			if len(msgs) == 0 {
				return fmt.Errorf("no messages in target chat %d", targetID)
			}
		}
		return nil
	})
}
