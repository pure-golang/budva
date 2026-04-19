//go:build bdd

package steps

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

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
		contents := make([]client.InputMessageContent, 0, len(testPhotos))
		for i, photo := range testPhotos {
			absPath, err := filepath.Abs(photo)
			if err != nil {
				return fmt.Errorf("resolve photo path: %w", err)
			}
			contents = append(contents, &client.InputMessagePhoto{
				Photo:   &client.InputFileLocal{Path: absPath},
				Caption: &client.FormattedText{Text: fmt.Sprintf("photo %d", i+1)},
			})
		}

		albumMsgs, err := s.env.PutAlbum(context.Background(), s.env.SourceID, contents, s.prefix)
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
			msgs, err := s.env.CheckAlbumMessages(context.Background(), targetID, s.prefix, len(testPhotos))
			if err != nil {
				return err
			}
			for i, msg := range msgs {
				expected := fmt.Sprintf("photo %d", i+1)
				text := messageCaption(msg)
				got := "<nil>"
				if text != nil {
					got = text.Text
				}
				if text == nil || !strings.Contains(text.Text, expected) {
					return fmt.Errorf("album order: expected %q at position %d in target %d, got %q",
						expected, i, targetID, got)
				}
			}
		}
		return nil
	})
}
