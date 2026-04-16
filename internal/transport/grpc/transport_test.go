package grpc

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/pure-golang/budva-claude/internal/domain"
	"github.com/pure-golang/budva-claude/internal/transport/grpc/mocks"
	"github.com/pure-golang/budva-claude/internal/transport/grpc/pb"
)

func TestGetMessages_Success(t *testing.T) {
	t.Parallel()

	// Arrange
	facade := mocks.NewFacadeService(t)
	facade.EXPECT().GetMessages(mock.Anything, int64(100), []int64{1, 2}).
		Return([]*domain.Message{
			{ChatID: 100, ID: 1, Content: domain.MessageContent{Type: domain.ContentText, Text: &domain.FormattedText{Text: "hello"}}},
			{ChatID: 100, ID: 2, Content: domain.MessageContent{Type: domain.ContentText, Text: &domain.FormattedText{Text: "world"}}},
		}, nil)
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

func TestGetMessages_Error(t *testing.T) {
	t.Parallel()

	// Arrange
	facade := mocks.NewFacadeService(t)
	facade.EXPECT().GetMessages(mock.Anything, int64(100), []int64{1, 2}).
		Return(nil, errors.New("batch failed"))
	tr := New(facade)

	// Act
	resp, err := tr.GetMessages(context.Background(), &pb.GetMessagesRequest{
		ChatId:     100,
		MessageIds: []int64{1, 2},
	})

	// Assert
	require.Error(t, err)
	assert.Nil(t, resp)
}

func TestSendMessage_Success(t *testing.T) {
	t.Parallel()

	// Arrange
	facade := mocks.NewFacadeService(t)
	facade.EXPECT().SendMessage(mock.Anything, int64(100), "hello").
		Return(nil)
	tr := New(facade)

	// Act
	resp, err := tr.SendMessage(context.Background(), &pb.SendMessageRequest{
		Message: &pb.NewMessage{ChatId: 100, Text: "hello"},
	})

	// Assert
	require.NoError(t, err)
	assert.NotNil(t, resp)
}

func TestSendMessage_NilMessage(t *testing.T) {
	t.Parallel()

	// Arrange
	tr := New(mocks.NewFacadeService(t))

	// Act
	_, err := tr.SendMessage(context.Background(), &pb.SendMessageRequest{})

	// Assert
	require.Error(t, err)
	assert.Equal(t, codes.InvalidArgument, status.Code(err))
}

func TestSendMessage_FacadeError(t *testing.T) {
	t.Parallel()

	// Arrange
	facade := mocks.NewFacadeService(t)
	facade.EXPECT().SendMessage(mock.Anything, int64(100), "hello").
		Return(errors.New("send failed"))
	tr := New(facade)

	// Act
	_, err := tr.SendMessage(context.Background(), &pb.SendMessageRequest{
		Message: &pb.NewMessage{ChatId: 100, Text: "hello"},
	})

	// Assert
	require.Error(t, err)
	assert.Equal(t, codes.Internal, status.Code(err))
}

func TestSendMessageAlbum_Success(t *testing.T) {
	t.Parallel()

	// Arrange
	facade := mocks.NewFacadeService(t)
	facade.EXPECT().SendMessageAlbum(mock.Anything, int64(100), []domain.AlbumItem{
		{Text: "one"},
		{Text: "two"},
	}).Return(nil)
	tr := New(facade)

	// Act
	resp, err := tr.SendMessageAlbum(context.Background(), &pb.SendMessageAlbumRequest{
		Messages: []*pb.NewMessage{
			{ChatId: 100, Text: "one"},
			{ChatId: 100, Text: "two"},
		},
	})

	// Assert
	require.NoError(t, err)
	assert.NotNil(t, resp)
}

func TestSendMessageAlbum_EmptyMessages(t *testing.T) {
	t.Parallel()

	// Arrange
	tr := New(mocks.NewFacadeService(t))

	// Act
	_, err := tr.SendMessageAlbum(context.Background(), &pb.SendMessageAlbumRequest{})

	// Assert
	require.Error(t, err)
	assert.Equal(t, codes.InvalidArgument, status.Code(err))
}

func TestForwardMessage_Success(t *testing.T) {
	t.Parallel()

	// Arrange
	facade := mocks.NewFacadeService(t)
	facade.EXPECT().ForwardMessage(mock.Anything, int64(100), int64(1)).
		Return(nil)
	tr := New(facade)

	// Act
	resp, err := tr.ForwardMessage(context.Background(), &pb.ForwardMessageRequest{
		ChatId: 100, MessageId: 1,
	})

	// Assert
	require.NoError(t, err)
	assert.NotNil(t, resp)
}

func TestGetMessage_Success(t *testing.T) {
	t.Parallel()

	// Arrange
	facade := mocks.NewFacadeService(t)
	facade.EXPECT().GetMessage(mock.Anything, int64(100), int64(1)).
		Return(&domain.Message{
			ChatID:  100,
			ID:      1,
			Content: domain.MessageContent{Type: domain.ContentText, Text: &domain.FormattedText{Text: "result"}},
		}, nil)
	tr := New(facade)

	// Act
	resp, err := tr.GetMessage(context.Background(), &pb.GetMessageRequest{ChatId: 100, MessageId: 1})

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "result", resp.GetMessage().GetText())
	assert.Equal(t, int64(100), resp.GetMessage().GetChatId())
}

func TestUpdateMessage_Success(t *testing.T) {
	t.Parallel()

	// Arrange
	facade := mocks.NewFacadeService(t)
	facade.EXPECT().UpdateMessage(mock.Anything, int64(100), int64(1), "updated").
		Return(nil)
	tr := New(facade)

	// Act
	resp, err := tr.UpdateMessage(context.Background(), &pb.UpdateMessageRequest{
		Message: &pb.Message{ChatId: 100, Id: 1, Text: "updated"},
	})

	// Assert
	require.NoError(t, err)
	assert.NotNil(t, resp)
}

func TestUpdateMessage_NilMessage(t *testing.T) {
	t.Parallel()

	// Arrange
	tr := New(mocks.NewFacadeService(t))

	// Act
	_, err := tr.UpdateMessage(context.Background(), &pb.UpdateMessageRequest{})

	// Assert
	require.Error(t, err)
	assert.Equal(t, codes.InvalidArgument, status.Code(err))
}

func TestDeleteMessages_Success(t *testing.T) {
	t.Parallel()

	// Arrange
	facade := mocks.NewFacadeService(t)
	facade.EXPECT().DeleteMessages(mock.Anything, int64(100), []int64{1, 2, 3}).
		Return(nil)
	tr := New(facade)

	// Act
	resp, err := tr.DeleteMessages(context.Background(), &pb.DeleteMessagesRequest{
		ChatId: 100, MessageIds: []int64{1, 2, 3},
	})

	// Assert
	require.NoError(t, err)
	assert.NotNil(t, resp)
}

func TestGetMessageLink_Success(t *testing.T) {
	t.Parallel()

	// Arrange
	facade := mocks.NewFacadeService(t)
	facade.EXPECT().GetMessageLink(mock.Anything, int64(100), int64(1)).
		Return("https://t.me/c/100/1", nil)
	tr := New(facade)

	// Act
	resp, err := tr.GetMessageLink(context.Background(), &pb.GetMessageLinkRequest{ChatId: 100, MessageId: 1})

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "https://t.me/c/100/1", resp.GetLink())
}

func TestGetMessageLinkInfo_Success(t *testing.T) {
	t.Parallel()

	// Arrange
	facade := mocks.NewFacadeService(t)
	facade.EXPECT().GetMessageLinkInfo(mock.Anything, "https://t.me/c/100/1").
		Return(&domain.MessageLinkInfo{ChatID: 100, MessageID: 1}, nil)
	tr := New(facade)

	// Act
	resp, err := tr.GetMessageLinkInfo(context.Background(), &pb.GetMessageLinkInfoRequest{Link: "https://t.me/c/100/1"})

	// Assert
	require.NoError(t, err)
	assert.Equal(t, int64(100), resp.GetMessage().GetChatId())
	assert.Equal(t, int64(1), resp.GetMessage().GetId())
}

func TestGetChatHistory_Success(t *testing.T) {
	t.Parallel()

	// Arrange
	facade := mocks.NewFacadeService(t)
	facade.EXPECT().GetChatHistory(mock.Anything, int64(100), int64(0), int32(0), int32(10)).
		Return([]*domain.Message{
			{ChatID: 100, ID: 1, Content: domain.MessageContent{Type: domain.ContentText, Text: &domain.FormattedText{Text: "msg1"}}},
			{ChatID: 100, ID: 2, Content: domain.MessageContent{Type: domain.ContentText, Text: &domain.FormattedText{Text: "msg2"}}},
		}, nil)
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
	facade := mocks.NewFacadeService(t)
	facade.EXPECT().GetChatHistory(mock.Anything, int64(100), int64(0), int32(0), int32(10)).
		Return(nil, nil)
	tr := New(facade)

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

	// Act
	result := domainToProto(nil)

	// Assert
	assert.Nil(t, result)
}

func TestDomainToProto_WithForwardInfo(t *testing.T) {
	t.Parallel()

	// Arrange
	msg := &domain.Message{
		ChatID: 100,
		ID:     1,
		Content: domain.MessageContent{
			Type: domain.ContentText,
			Text: &domain.FormattedText{Text: "hello"},
		},
		ForwardInfo: &domain.MessageForwardInfo{OriginChatID: 50},
	}

	// Act
	result := domainToProto(msg)

	// Assert
	assert.True(t, result.GetForward())
	assert.Equal(t, "hello", result.GetText())
}
