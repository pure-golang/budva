package shared

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/cucumber/godog"
	"github.com/zelenin/go-tdlib/client"
)

// testPhotos — фикстуры для медиа-альбомов (используются только в RegisterMediaSteps).
var testPhotos = []string{
	"test/bdd/testdata/photo1.png",
	"test/bdd/testdata/photo2.png",
	"test/bdd/testdata/photo3.png",
}

// RegisterMediaSteps регистрирует шаги эпика 04_media.
func RegisterMediaSteps(ctx *godog.ScenarioContext, s *State) {
	ctx.When(`^пользователь отправляет медиа-альбом в исходный чат$`, func() error {
		s.ApplyRuleSet()

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

		albumMsgs, err := s.Env.PutAlbum(s.Env.SourceID, contents, s.Prefix)
		if err != nil {
			return err
		}

		for _, msg := range albumMsgs {
			s.Env.Handler.OnNewMessage(context.Background(), msg)
		}
		s.Env.DrainQueue()

		return nil
	})

	ctx.Then(`^медиа-альбом появляется во всех целевых чатах в правильном порядке$`, func() error {
		for _, targetID := range s.Env.TargetIDs {
			msgs, err := s.Env.CheckAlbumMessages(targetID, s.Prefix, int32(len(testPhotos))) //nolint:gosec // test-data фиксированного размера, overflow невозможен
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
