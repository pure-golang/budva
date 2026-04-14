package integration

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pure-golang/budva-claude/internal/domain"
	"github.com/pure-golang/budva-claude/test/support"
)

func newTestEnv(t *testing.T) *support.TestEnv {
	t.Helper()
	env, err := support.NewTestEnv()
	require.NoError(t, err)
	t.Cleanup(func() { env.Close() })
	return env
}

func TestForwardPipeline_CopyWithTransform(t *testing.T) {
	t.Parallel()

	// Arrange
	env := newTestEnv(t)

	src := &domain.Source{
		ChatID: env.SourceID,
		Sign: &domain.Sign{
			Title: "TestSign",
			For:   env.TargetIDs,
		},
		Translate: &domain.Translate{
			Lang: "ru",
			For:  env.TargetIDs,
		},
	}
	rs := env.MakeRuleSet(true, src)
	env.Handler.SetRuleSet(rs)

	msg := &domain.Message{
		ChatID:     env.SourceID,
		ID:         1,
		CanBeSaved: true,
		Content: domain.MessageContent{
			Type: domain.ContentText,
			Text: &domain.FormattedText{Text: "Hello world"},
		},
	}
	env.TelegramFake.PutMessage(msg)

	// Act
	env.Handler.OnNewMessage(context.Background(), msg)
	env.DrainQueue()

	// Assert — сообщения доставлены в целевые чаты с трансформациями
	for _, targetID := range env.TargetIDs {
		msgs := env.TelegramFake.MessagesInChat(targetID)
		require.NotEmpty(t, msgs, "target chat %d should have messages", targetID)

		// Проверяем что текст содержит перевод (prefix от FakeTelegram.TranslateText)
		found := false
		for _, m := range msgs {
			if m.Content.Text != nil {
				found = true
				assert.Contains(t, m.Content.Text.Text, "[ru]",
					"target %d: message should contain translation prefix", targetID)
			}
		}
		assert.True(t, found, "target %d: should have at least one text message", targetID)
	}
}

func TestForwardPipeline_EditSync(t *testing.T) {
	t.Parallel()

	// Arrange
	env := newTestEnv(t)

	rs := env.MakeRuleSet(true, nil)
	env.Handler.SetRuleSet(rs)

	msg := &domain.Message{
		ChatID:     env.SourceID,
		ID:         1,
		CanBeSaved: true,
		Content: domain.MessageContent{
			Type: domain.ContentText,
			Text: &domain.FormattedText{Text: "Original text"},
		},
	}
	env.TelegramFake.PutMessage(msg)

	// Act — отправляем сообщение
	env.Handler.OnNewMessage(context.Background(), msg)
	env.DrainQueue()

	// Имитируем OnMessageSendSucceeded: замена tmpMsgID на permanent ID
	for _, targetID := range env.TargetIDs {
		targetMsgs := env.TelegramFake.MessagesInChat(targetID)
		for _, m := range targetMsgs {
			newID := m.ID + 10000
			env.Handler.OnMessageSendSucceeded(targetID, m.ID, newID)
			// В реальном TDLib temp-сообщение заменяется permanent; имитируем
			env.TelegramFake.ReplaceMessageID(targetID, m.ID, newID)
		}
	}
	env.DrainQueue()

	// Редактируем сообщение
	editedMsg := &domain.Message{
		ChatID:     env.SourceID,
		ID:         1,
		CanBeSaved: true,
		Content: domain.MessageContent{
			Type: domain.ContentText,
			Text: &domain.FormattedText{Text: "Edited text"},
		},
	}
	env.Handler.OnEditedMessage(context.Background(), editedMsg)
	env.DrainQueue()

	// Assert — проверяем что в целевых чатах текст обновлён
	for _, targetID := range env.TargetIDs {
		found := false
		for _, m := range env.TelegramFake.MessagesInChat(targetID) {
			if m.Content.Text != nil && m.Content.Text.Text == "Edited text" {
				found = true
				break
			}
		}
		assert.True(t, found, "target %d: should have message with edited text", targetID)
	}
}

func TestForwardPipeline_DeleteSync(t *testing.T) {
	t.Parallel()

	// Arrange
	env := newTestEnv(t)

	rs := env.MakeRuleSet(true, nil)
	env.Handler.SetRuleSet(rs)

	msg := &domain.Message{
		ChatID:     env.SourceID,
		ID:         1,
		CanBeSaved: true,
		Content: domain.MessageContent{
			Type: domain.ContentText,
			Text: &domain.FormattedText{Text: "Will be deleted"},
		},
	}
	env.TelegramFake.PutMessage(msg)

	// Act — отправляем сообщение
	env.Handler.OnNewMessage(context.Background(), msg)
	env.DrainQueue()

	// Имитируем OnMessageSendSucceeded: замена tmpMsgID на permanent ID
	for _, targetID := range env.TargetIDs {
		targetMsgs := env.TelegramFake.MessagesInChat(targetID)
		for _, m := range targetMsgs {
			newID := m.ID + 10000
			env.Handler.OnMessageSendSucceeded(targetID, m.ID, newID)
			// В реальном TDLib temp-сообщение заменяется permanent; имитируем
			env.TelegramFake.ReplaceMessageID(targetID, m.ID, newID)
		}
	}
	env.DrainQueue()

	// Удаляем сообщение в источнике
	env.Handler.OnDeletedMessages(context.Background(), env.SourceID, []domain.MessageID{1}, true)
	env.DrainQueue()

	// Assert — целевые чаты не содержат удалённых сообщений
	for _, targetID := range env.TargetIDs {
		msgs := env.TelegramFake.MessagesInChat(targetID)
		for _, m := range msgs {
			if m.Content.Text != nil {
				assert.NotEqual(t, "Will be deleted", m.Content.Text.Text,
					"target %d: deleted message should be removed", targetID)
			}
		}
	}
}

func TestForwardPipeline_FilterExclude(t *testing.T) {
	t.Parallel()

	// Arrange
	env := newTestEnv(t)

	rs := env.MakeRuleSet(true, nil)
	rule := rs.ForwardRules["test_rule"]
	rule.Exclude = "SPAM"
	env.Handler.SetRuleSet(rs)

	msg := &domain.Message{
		ChatID:     env.SourceID,
		ID:         1,
		CanBeSaved: true,
		Content: domain.MessageContent{
			Type: domain.ContentText,
			Text: &domain.FormattedText{Text: "This is SPAM content"},
		},
	}
	env.TelegramFake.PutMessage(msg)

	// Act
	env.Handler.OnNewMessage(context.Background(), msg)
	env.DrainQueue()

	// Assert — целевые чаты не должны содержать сообщений
	for _, targetID := range env.TargetIDs {
		msgs := env.TelegramFake.MessagesInChat(targetID)
		assert.Empty(t, msgs, "target %d: should have no messages when exclude filter matches", targetID)
	}
}
