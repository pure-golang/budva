package integration

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pure-golang/budva-claude/internal/domain"
)

func TestForwardPipeline(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test")
	}

	s := setupSuite(t)
	t.Cleanup(func() {
		if err := s.Close(); err != nil {
			t.Errorf("stack close: %v", err)
		}
	})

	t.Run("copy_with_transform", func(t *testing.T) {
		s.Telegram.Reset()
		// Arrange
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		rs := s.makeRuleSet(true)
		rs.Sources[s.SourceID].Sign = &domain.Sign{Title: "TestSign", For: s.TargetIDs}
		s.Handler.SetRuleSet(rs)

		msg := &domain.Message{
			ChatID: s.SourceID, ID: 1, CanBeSaved: true,
			Content: domain.MessageContent{Type: domain.ContentText, Text: &domain.FormattedText{Text: "hello"}},
		}
		s.Telegram.PutMessage(msg)

		// Act
		s.Handler.OnNewMessage(ctx, msg)
		s.Queue.ProcessAll()

		// Assert
		msgs := s.Telegram.MessagesInChat(s.TargetIDs[0])
		require.NotEmpty(t, msgs)
		assert.Contains(t, msgs[0].Content.Text.Text, "TestSign")
	})

	t.Run("edit_sync", func(t *testing.T) {
		s.Telegram.Reset()
		// Arrange
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		rs := s.makeRuleSet(true)
		s.Handler.SetRuleSet(rs)

		msg := &domain.Message{
			ChatID: s.SourceID, ID: 2, CanBeSaved: true,
			Content: domain.MessageContent{Type: domain.ContentText, Text: &domain.FormattedText{Text: "original"}},
		}
		s.Telegram.PutMessage(msg)
		s.Handler.OnNewMessage(ctx, msg)
		s.Queue.ProcessAll()

		for _, target := range s.TargetIDs {
			for _, m := range s.Telegram.MessagesInChat(target) {
				s.Handler.OnMessageSendSucceeded(target, m.ID, m.ID)
			}
		}

		// Act
		editMsg := &domain.Message{
			ChatID: s.SourceID, ID: 2, CanBeSaved: true,
			Content: domain.MessageContent{Type: domain.ContentText, Text: &domain.FormattedText{Text: "updated"}},
		}
		s.Telegram.PutMessage(editMsg)
		s.Handler.OnEditedMessage(ctx, editMsg)
		s.Queue.ProcessAll()

		// Assert
		for _, target := range s.TargetIDs {
			msgs := s.Telegram.MessagesInChat(target)
			require.NotEmpty(t, msgs)
			found := false
			for _, m := range msgs {
				if m.Content.Text != nil && m.Content.Text.Text == "updated" {
					found = true
				}
			}
			assert.True(t, found, "target %d should have updated message", target)
		}
	})

	t.Run("delete_sync", func(t *testing.T) {
		s.Telegram.Reset()
		// Arrange
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		rs := s.makeRuleSet(true)
		s.Handler.SetRuleSet(rs)

		msg := &domain.Message{
			ChatID: s.SourceID, ID: 3, CanBeSaved: true,
			Content: domain.MessageContent{Type: domain.ContentText, Text: &domain.FormattedText{Text: "to delete"}},
		}
		s.Telegram.PutMessage(msg)
		s.Handler.OnNewMessage(ctx, msg)
		s.Queue.ProcessAll()

		for _, target := range s.TargetIDs {
			for _, m := range s.Telegram.MessagesInChat(target) {
				s.Handler.OnMessageSendSucceeded(target, m.ID, m.ID)
			}
		}
		s.Queue.ProcessAll()

		// Act
		s.Handler.OnDeletedMessages(ctx, s.SourceID, []domain.MessageID{3}, true)
		s.Queue.ProcessAll()

		// Assert
		for _, target := range s.TargetIDs {
			msgs := s.Telegram.MessagesInChat(target)
			assert.Empty(t, msgs, "target %d should have no messages after delete", target)
		}
	})

	t.Run("filter_exclude", func(t *testing.T) {
		s.Telegram.Reset()
		// Arrange
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		rs := s.makeRuleSet(true)
		rs.ForwardRules["test_rule"].Exclude = "SPAM"
		s.Handler.SetRuleSet(rs)

		msg := &domain.Message{
			ChatID: s.SourceID, ID: 4, CanBeSaved: true,
			Content: domain.MessageContent{Type: domain.ContentText, Text: &domain.FormattedText{Text: "contains SPAM word"}},
		}
		s.Telegram.PutMessage(msg)

		// Act
		s.Handler.OnNewMessage(ctx, msg)
		s.Queue.ProcessAll()

		// Assert
		for _, target := range s.TargetIDs {
			msgs := s.Telegram.MessagesInChat(target)
			assert.Empty(t, msgs, "target %d should have no messages (excluded)", target)
		}
	})
}
