package steps

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/cucumber/godog"

	"github.com/pure-golang/budva-claude/internal/domain"
)

var testPhotos = []string{
	"test/bdd/testdata/photo1.png",
	"test/bdd/testdata/photo2.png",
	"test/bdd/testdata/photo3.png",
}

func register04MediaSteps(ctx *godog.ScenarioContext, s *scenarioCtx) {
	ctx.When(`^пользователь отправляет медиа-альбом в исходный чат$`, func() error {
		s.applyRuleSet()

		// Отправляем реальный альбом из 3 фото через TDLib SendMessageAlbum
		contents := make([]domain.InputMessageContent, 0, len(testPhotos))
		for i, photo := range testPhotos {
			absPath, err := filepath.Abs(photo)
			if err != nil {
				return fmt.Errorf("resolve photo path: %w", err)
			}
			contents = append(contents, domain.InputMessageContent{
				Type:     domain.ContentPhoto,
				FilePath: absPath,
				Text:     &domain.FormattedText{Text: fmt.Sprintf("photo %d", i+1)},
			})
		}

		albumMsgs, err := s.env.PutAlbum(context.Background(), s.env.SourceID, contents, s.prefix)
		if err != nil {
			return err
		}

		// Handler получает каждое сообщение альбома индивидуально (как в production)
		for _, msg := range albumMsgs {
			s.env.Handler.OnNewMessage(context.Background(), msg)
		}
		s.env.DrainQueue()

		return nil
	})

	ctx.Then(`^медиа-альбом появляется во всех целевых чатах$`, func() error {
		for _, targetID := range s.env.TargetIDs {
			if _, err := s.env.CheckLastMessage(context.Background(), targetID, s.prefix); err != nil {
				return err
			}
		}
		return nil
	})
}
