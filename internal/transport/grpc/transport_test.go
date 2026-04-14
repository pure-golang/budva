package grpc

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/pure-golang/budva-claude/internal/domain"
	"github.com/pure-golang/budva-claude/internal/transport/grpc/pb"
)

type mockFacade struct {
	getMessage      func(ctx context.Context, chatID, messageID int64) (*domain.Message, error)
	getChatHistory func(ctx context.Context, chatID, fromMessageID int64, offset, limit int32) ([]*domain.Message, error)
	sendMessage     func(ctx context.Context, chatID int64, text string) error
	sendAlbum       func(ctx context.Context, chatID int64, texts []string) error
	forwardMessage  func(ctx context.Context, chatID, messageID int64) error
	updateMessage   func(ctx context.Context, chatID, messageID int64, text string) error
	deleteMessages  func(ctx context.Context, chatID int64, messageIDs []int64) error
	getMessageLink  func(ctx context.Context, chatID, messageID int64) (string, error)
	getMessageLInfo func(ctx context.Context, link string) (*domain.MessageLinkInfo, error)
}

func (m *mockFacade) GetMessage(ctx context.Context, chatID, messageID int64) (*domain.Message, error) {
	if m.getMessage != nil {
		return m.getMessage(ctx, chatID, messageID)
	}
	return nil, errors.New("not mocked")
}

func (m *mockFacade) GetChatHistory(ctx context.Context, chatID, fromMessageID int64, offset, limit int32) ([]*domain.Message, error) {
	if m.getChatHistory != nil {
		return m.getChatHistory(ctx, chatID, fromMessageID, offset, limit)
	}
	return nil, nil
}

func (m *mockFacade) SendMessage(ctx context.Context, chatID int64, text string) error {
	if m.sendMessage != nil {
		return m.sendMessage(ctx, chatID, text)
	}
	return nil
}

func (m *mockFacade) SendMessageAlbum(ctx context.Context, chatID int64, texts []string) error {
	if m.sendAlbum != nil {
		return m.sendAlbum(ctx, chatID, texts)
	}
	return nil
}

func (m *mockFacade) ForwardMessage(ctx context.Context, chatID, messageID int64) error {
	if m.forwardMessage != nil {
		return m.forwardMessage(ctx, chatID, messageID)
	}
	return nil
}

func (m *mockFacade) UpdateMessage(ctx context.Context, chatID, messageID int64, text string) error {
	if m.updateMessage != nil {
		return m.updateMessage(ctx, chatID, messageID, text)
	}
	return nil
}

func (m *mockFacade) DeleteMessages(ctx context.Context, chatID int64, messageIDs []int64) error {
	if m.deleteMessages != nil {
		return m.deleteMessages(ctx, chatID, messageIDs)
	}
	return nil
}

func (m *mockFacade) GetMessageLink(ctx context.Context, chatID, messageID int64) (string, error) {
	if m.getMessageLink != nil {
		return m.getMessageLink(ctx, chatID, messageID)
	}
	return "", nil
}

func (m *mockFacade) GetMessageLinkInfo(ctx context.Context, link string) (*domain.MessageLinkInfo, error) {
	if m.getMessageLInfo != nil {
		return m.getMessageLInfo(ctx, link)
	}
	return nil, errors.New("not mocked")
}

func TestGetMessages_Success(t *testing.T) {
	t.Parallel()

	// Arrange
	facade := &mockFacade{
		getMessage: func(_ context.Context, chatID, messageID int64) (*domain.Message, error) {
			return &domain.Message{
				ChatID:  chatID,
				ID:      messageID,
				Content: domain.MessageContent{Type: domain.ContentText, Text: &domain.FormattedText{Text: "hello"}},
			}, nil
		},
	}
	tr := New(facade)

	// Act
	resp, err := tr.GetMessages(context.Background(), &pb.GetMessagesRequest{
		ChatId:     100,
		MessageIds: []int64{1, 2},
	})

	// Assert
	require.NoError(t, err)
	assert.Len(t, resp.GetMessages(), 2)
	assert.Equal(t, "hello", resp.GetMessages()[0].GetText())
}

func TestGetMessages_PartialFailure(t *testing.T) {
	t.Parallel()

	// Arrange
	call := 0
	facade := &mockFacade{
		getMessage: func(_ context.Context, _, _ int64) (*domain.Message, error) {
			call++
			if call == 1 {
				return nil, errors.New("not found")
			}
			return &domain.Message{
				ChatID:  100,
				ID:      2,
				Content: domain.MessageContent{Type: domain.ContentText, Text: &domain.FormattedText{Text: "ok"}},
			}, nil
		},
	}
	tr := New(facade)

	// Act
	resp, err := tr.GetMessages(context.Background(), &pb.GetMessagesRequest{
		ChatId:     100,
		MessageIds: []int64{1, 2},
	})

	// Assert
	require.NoError(t, err)
	assert.Len(t, resp.GetMessages(), 1)
}

func TestSendMessage_Success(t *testing.T) {
	t.Parallel()

	// Arrange
	var gotChatID int64
	var gotText string
	facade := &mockFacade{
		sendMessage: func(_ context.Context, chatID int64, text string) error {
			gotChatID = chatID
			gotText = text
			return nil
		},
	}
	tr := New(facade)

	// Act
	resp, err := tr.SendMessage(context.Background(), &pb.SendMessageRequest{
		Message: &pb.NewMessage{ChatId: 100, Text: "hello"},
	})

	// Assert
	require.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, int64(100), gotChatID)
	assert.Equal(t, "hello", gotText)
}

func TestSendMessage_NilMessage(t *testing.T) {
	t.Parallel()
	tr := New(&mockFacade{})

	_, err := tr.SendMessage(context.Background(), &pb.SendMessageRequest{})

	require.Error(t, err)
	assert.Equal(t, codes.InvalidArgument, status.Code(err))
}

func TestSendMessage_FacadeError(t *testing.T) {
	t.Parallel()
	facade := &mockFacade{
		sendMessage: func(_ context.Context, _ int64, _ string) error {
			return errors.New("send failed")
		},
	}
	tr := New(facade)

	_, err := tr.SendMessage(context.Background(), &pb.SendMessageRequest{
		Message: &pb.NewMessage{ChatId: 100, Text: "hello"},
	})

	require.Error(t, err)
	assert.Equal(t, codes.Internal, status.Code(err))
}

func TestSendMessageAlbum_Success(t *testing.T) {
	t.Parallel()
	facade := &mockFacade{}
	tr := New(facade)

	resp, err := tr.SendMessageAlbum(context.Background(), &pb.SendMessageAlbumRequest{
		Messages: []*pb.NewMessage{
			{ChatId: 100, Text: "one"},
			{ChatId: 100, Text: "two"},
		},
	})

	require.NoError(t, err)
	assert.NotNil(t, resp)
}

func TestSendMessageAlbum_EmptyMessages(t *testing.T) {
	t.Parallel()
	tr := New(&mockFacade{})

	_, err := tr.SendMessageAlbum(context.Background(), &pb.SendMessageAlbumRequest{})

	require.Error(t, err)
	assert.Equal(t, codes.InvalidArgument, status.Code(err))
}

func TestForwardMessage_Success(t *testing.T) {
	t.Parallel()
	tr := New(&mockFacade{})

	resp, err := tr.ForwardMessage(context.Background(), &pb.ForwardMessageRequest{
		ChatId: 100, MessageId: 1,
	})

	require.NoError(t, err)
	assert.NotNil(t, resp)
}

func TestGetMessage_Success(t *testing.T) {
	t.Parallel()
	facade := &mockFacade{
		getMessage: func(_ context.Context, chatID, messageID int64) (*domain.Message, error) {
			return &domain.Message{
				ChatID:  chatID,
				ID:      messageID,
				Content: domain.MessageContent{Type: domain.ContentText, Text: &domain.FormattedText{Text: "result"}},
			}, nil
		},
	}
	tr := New(facade)

	resp, err := tr.GetMessage(context.Background(), &pb.GetMessageRequest{ChatId: 100, MessageId: 1})

	require.NoError(t, err)
	assert.Equal(t, "result", resp.GetMessage().GetText())
	assert.Equal(t, int64(100), resp.GetMessage().GetChatId())
}

func TestUpdateMessage_Success(t *testing.T) {
	t.Parallel()
	tr := New(&mockFacade{})

	resp, err := tr.UpdateMessage(context.Background(), &pb.UpdateMessageRequest{
		Message: &pb.Message{ChatId: 100, Id: 1, Text: "updated"},
	})

	require.NoError(t, err)
	assert.NotNil(t, resp)
}

func TestUpdateMessage_NilMessage(t *testing.T) {
	t.Parallel()
	tr := New(&mockFacade{})

	_, err := tr.UpdateMessage(context.Background(), &pb.UpdateMessageRequest{})

	require.Error(t, err)
	assert.Equal(t, codes.InvalidArgument, status.Code(err))
}

func TestDeleteMessages_Success(t *testing.T) {
	t.Parallel()
	tr := New(&mockFacade{})

	resp, err := tr.DeleteMessages(context.Background(), &pb.DeleteMessagesRequest{
		ChatId: 100, MessageIds: []int64{1, 2, 3},
	})

	require.NoError(t, err)
	assert.NotNil(t, resp)
}

func TestGetMessageLink_Success(t *testing.T) {
	t.Parallel()
	facade := &mockFacade{
		getMessageLink: func(_ context.Context, _, _ int64) (string, error) {
			return "https://t.me/c/100/1", nil
		},
	}
	tr := New(facade)

	resp, err := tr.GetMessageLink(context.Background(), &pb.GetMessageLinkRequest{ChatId: 100, MessageId: 1})

	require.NoError(t, err)
	assert.Equal(t, "https://t.me/c/100/1", resp.GetLink())
}

func TestGetMessageLinkInfo_Success(t *testing.T) {
	t.Parallel()
	facade := &mockFacade{
		getMessageLInfo: func(_ context.Context, _ string) (*domain.MessageLinkInfo, error) {
			return &domain.MessageLinkInfo{ChatID: 100, MessageID: 1}, nil
		},
	}
	tr := New(facade)

	resp, err := tr.GetMessageLinkInfo(context.Background(), &pb.GetMessageLinkInfoRequest{Link: "https://t.me/c/100/1"})

	require.NoError(t, err)
	assert.Equal(t, int64(100), resp.GetMessage().GetChatId())
	assert.Equal(t, int64(1), resp.GetMessage().GetId())
}

func TestGetChatHistory_Success(t *testing.T) {
	t.Parallel()

	// Arrange
	facade := &mockFacade{
		getChatHistory: func(_ context.Context, chatID, _ int64, _, _ int32) ([]*domain.Message, error) {
			return []*domain.Message{
				{ChatID: chatID, ID: 1, Content: domain.MessageContent{Type: domain.ContentText, Text: &domain.FormattedText{Text: "msg1"}}},
				{ChatID: chatID, ID: 2, Content: domain.MessageContent{Type: domain.ContentText, Text: &domain.FormattedText{Text: "msg2"}}},
			}, nil
		},
	}
	tr := New(facade)

	// Act
	resp, err := tr.GetChatHistory(context.Background(), &pb.GetChatHistoryRequest{
		ChatId: 100, FromMessageId: 0, Offset: 0, Limit: 10,
	})

	// Assert
	require.NoError(t, err)
	assert.Len(t, resp.GetMessages(), 2)
	assert.Equal(t, "msg1", resp.GetMessages()[0].GetText())
}

func TestGetChatHistory_Empty(t *testing.T) {
	t.Parallel()

	// Arrange
	tr := New(&mockFacade{})

	// Act
	resp, err := tr.GetChatHistory(context.Background(), &pb.GetChatHistoryRequest{
		ChatId: 100, Limit: 10,
	})

	// Assert
	require.NoError(t, err)
	assert.Empty(t, resp.GetMessages())
}

func TestDomainToProto_Nil(t *testing.T) {
	t.Parallel()
	assert.Nil(t, domainToProto(nil))
}

func TestDomainToProto_WithForwardInfo(t *testing.T) {
	t.Parallel()
	msg := &domain.Message{
		ChatID: 100,
		ID:     1,
		Content: domain.MessageContent{
			Type: domain.ContentText,
			Text: &domain.FormattedText{Text: "hello"},
		},
		ForwardInfo: &domain.MessageForwardInfo{OriginChatID: 50},
	}

	result := domainToProto(msg)
	assert.True(t, result.GetForward())
	assert.Equal(t, "hello", result.GetText())
}
