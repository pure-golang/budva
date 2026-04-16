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

		// Отправляем 3 фото-сообщения с одинаковым albumID
		for i := 1; i <= 3; i++ {
			msg, err := s.env.PutMessage(context.Background(), s.env.SourceID, domain.InputMessageContent{
				Type: domain.ContentPhoto,
				Text: &domain.FormattedText{Text: fmt.Sprintf("photo %d", i)},
			})
			if err != nil {
				return err
			}
			// Устанавливаем albumID вручную (TDLib не гарантирует albumID при отдельной отправке)
			msg.MediaAlbumID = 12345
			s.env.Handler.OnNewMessage(context.Background(), msg)
		}

		s.env.DrainQueue()

		return nil
	})

	ctx.Then(`^медиа-альбом появляется во всех целевых чатах$`, func() error {
		for _, targetID := range s.env.TargetIDs {
			msgs, err := s.env.MessagesInChat(context.Background(), targetID)
			if err != nil {
				return err
			}
			if len(msgs) == 0 {
				return fmt.Errorf("no messages in target chat %d", targetID)
			}
		}
		return nil
	})
}
