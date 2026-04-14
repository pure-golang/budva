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
	t.Cleanup(func() { tearDownSuite(t, s) })

	t.Run("copy_with_transform", func(t *testing.T) {
		resetState(t, s)
		// Arrange
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		rs := s.makeRuleSet(true)
		rs.Sources[s.sourceID].Sign = &domain.Sign{Title: "TestSign", For: s.targets}
		s.handler.SetRuleSet(rs)

		msg := &domain.Message{
			ChatID: s.sourceID, ID: 1, CanBeSaved: true,
			Content: domain.MessageContent{Type: domain.ContentText, Text: &domain.FormattedText{Text: "hello"}},
		}
		s.telegram.PutMessage(msg)

		// Act
		s.handler.OnNewMessage(ctx, msg)
		s.queue.ProcessAll()

		// Assert
		msgs := s.telegram.MessagesInChat(s.targets[0])
		require.NotEmpty(t, msgs)
		assert.Contains(t, msgs[0].Content.Text.Text, "TestSign")
	})

	t.Run("edit_sync", func(t *testing.T) {
		resetState(t, s)
		// Arrange
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		rs := s.makeRuleSet(true)
		s.handler.SetRuleSet(rs)

		msg := &domain.Message{
			ChatID: s.sourceID, ID: 2, CanBeSaved: true,
			Content: domain.MessageContent{Type: domain.ContentText, Text: &domain.FormattedText{Text: "original"}},
		}
		s.telegram.PutMessage(msg)
		s.handler.OnNewMessage(ctx, msg)
		s.queue.ProcessAll()

		for _, target := range s.targets {
			for _, m := range s.telegram.MessagesInChat(target) {
				s.handler.OnMessageSendSucceeded(target, m.ID, m.ID)
			}
		}

		// Act
		editMsg := &domain.Message{
			ChatID: s.sourceID, ID: 2, CanBeSaved: true,
			Content: domain.MessageContent{Type: domain.ContentText, Text: &domain.FormattedText{Text: "updated"}},
		}
		s.telegram.PutMessage(editMsg)
		s.handler.OnEditedMessage(ctx, editMsg)
		s.queue.ProcessAll()

		// Assert
		for _, target := range s.targets {
			msgs := s.telegram.MessagesInChat(target)
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
		resetState(t, s)
		// Arrange
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		rs := s.makeRuleSet(true)
		s.handler.SetRuleSet(rs)

		msg := &domain.Message{
			ChatID: s.sourceID, ID: 3, CanBeSaved: true,
			Content: domain.MessageContent{Type: domain.ContentText, Text: &domain.FormattedText{Text: "to delete"}},
		}
		s.telegram.PutMessage(msg)
		s.handler.OnNewMessage(ctx, msg)
		s.queue.ProcessAll()

		for _, target := range s.targets {
			for _, m := range s.telegram.MessagesInChat(target) {
				s.handler.OnMessageSendSucceeded(target, m.ID, m.ID)
			}
		}
		s.queue.ProcessAll()

		// Act
		s.handler.OnDeletedMessages(ctx, s.sourceID, []domain.MessageID{3}, true)
		s.queue.ProcessAll()

		// Assert
		for _, target := range s.targets {
			msgs := s.telegram.MessagesInChat(target)
			assert.Empty(t, msgs, "target %d should have no messages after delete", target)
		}
	})

	t.Run("filter_exclude", func(t *testing.T) {
		resetState(t, s)
		// Arrange
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		rs := s.makeRuleSet(true)
		rs.ForwardRules["test_rule"].Exclude = "SPAM"
		s.handler.SetRuleSet(rs)

		msg := &domain.Message{
			ChatID: s.sourceID, ID: 4, CanBeSaved: true,
			Content: domain.MessageContent{Type: domain.ContentText, Text: &domain.FormattedText{Text: "contains SPAM word"}},
		}
		s.telegram.PutMessage(msg)

		// Act
		s.handler.OnNewMessage(ctx, msg)
		s.queue.ProcessAll()

		// Assert
		for _, target := range s.targets {
			msgs := s.telegram.MessagesInChat(target)
			assert.Empty(t, msgs, "target %d should have no messages (excluded)", target)
		}
	})
}
