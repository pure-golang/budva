package steps

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

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

	ctx.Then(`^медиа-альбом появляется во всех целевых чатах в правильном порядке$`, func() error {
		for _, targetID := range s.env.TargetIDs {
			msgs, err := s.env.CheckAlbumMessages(context.Background(), targetID, s.prefix, len(testPhotos))
			if err != nil {
				return err
			}
			// Проверяем порядок: photo 1, photo 2, photo 3
			for i, msg := range msgs {
				expected := fmt.Sprintf("photo %d", i+1)
				if msg.Content.Text == nil || !strings.Contains(msg.Content.Text.Text, expected) {
					got := "<nil>"
					if msg.Content.Text != nil {
						got = msg.Content.Text.Text
					}
					return fmt.Errorf("album order: expected %q at position %d in target %d, got %q",
						expected, i, targetID, got)
				}
			}
		}
		return nil
	})
}
