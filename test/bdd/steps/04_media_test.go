//go:build bdd

package steps

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/cucumber/godog"
	"github.com/zelenin/go-tdlib/client"
)

var testPhotos = []string{
	"test/bdd/testdata/photo1.png",
	"test/bdd/testdata/photo2.png",
	"test/bdd/testdata/photo3.png",
}

func register04MediaSteps(ctx *godog.ScenarioContext, s *scenarioCtx) {
	ctx.When(`^пользователь отправляет медиа-альбом в исходный чат$`, func() error {
		s.applyRuleSet()

		// Отправляем реальный альбом из 3 фото через TDLib SendMessageAlbum.
		// Prefix добавляется к первому элементу внутри PutAlbum.
		contents := make([]client.InputMessageContent, 0, len(testPhotos))
		for _, photo := range testPhotos {
			absPath, err := filepath.Abs(photo)
			if err != nil {
				return fmt.Errorf("resolve photo path: %w", err)
			}
			contents = append(contents, &client.InputMessagePhoto{
				Photo: &client.InputFileLocal{Path: absPath},
			})
		}

		albumMsgs, err := s.env.PutAlbum(s.env.SourceID, contents, s.prefix)
		if err != nil {
			return err
		}

		for _, msg := range albumMsgs {
			s.env.Handler.OnNewMessage(context.Background(), msg)
		}
		s.env.DrainQueue()

		return nil
	})

	ctx.Then(`^медиа-альбом появляется во всех целевых чатах в правильном порядке$`, func() error {
		for _, targetID := range s.env.TargetIDs {
			msgs, err := s.env.CheckAlbumMessages(targetID, s.prefix, int32(len(testPhotos)))
			if err != nil {
				return err
			}
			for i, msg := range msgs {
				if _, ok := msg.Content.(*client.MessagePhoto); !ok {
					return fmt.Errorf("album position %d in target %d: expected photo, got %T",
						i, targetID, msg.Content)
				}
			}
		}
		return nil
	})
}
