package grpc

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/zelenin/go-tdlib/client"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/pure-golang/budva-claude/internal/domain"
	"github.com/pure-golang/budva-claude/internal/transport/grpc/mocks"
	"github.com/pure-golang/budva-claude/internal/transport/grpc/pb"
)

func textMessage(chatID, id int64, text string) *client.Message {
	return &client.Message{
		ChatId:  chatID,
		Id:      id,
		Content: &client.MessageText{Text: &client.FormattedText{Text: text}},
	}
}

func TestGetMessages_Success(t *testing.T) {
	t.Parallel()

	// Arrange
	facadeMock := mocks.NewFacadeService(t)
	facadeMock.EXPECT().GetMessages(mock.Anything, int64(100), []int64{1, 2}).
		Return([]*client.Message{textMessage(100, 1, "hello"), textMessage(100, 2, "world")}, nil)
	tr := New(facadeMock)

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
	facadeMock := mocks.NewFacadeService(t)
	facadeMock.EXPECT().GetMessages(mock.Anything, int64(100), []int64{1, 2}).
		Return(nil, errors.New("batch failed"))
	tr := New(facadeMock)

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
	facadeMock := mocks.NewFacadeService(t)
	facadeMock.EXPECT().SendMessage(mock.Anything, int64(100), "hello").Return(nil)
	tr := New(facadeMock)

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
	facadeMock := mocks.NewFacadeService(t)
	facadeMock.EXPECT().SendMessage(mock.Anything, int64(100), "hello").Return(errors.New("send failed"))
	tr := New(facadeMock)

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
	facadeMock := mocks.NewFacadeService(t)
	facadeMock.EXPECT().SendMessageAlbum(mock.Anything, int64(100), []domain.AlbumItem{
		{Text: "one"},
		{Text: "two"},
	}).Return(nil)
	tr := New(facadeMock)

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
	facadeMock := mocks.NewFacadeService(t)
	facadeMock.EXPECT().ForwardMessage(mock.Anything, int64(100), int64(1)).Return(nil)
	tr := New(facadeMock)

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
	facadeMock := mocks.NewFacadeService(t)
	facadeMock.EXPECT().GetMessage(mock.Anything, int64(100), int64(1)).
		Return(textMessage(100, 1, "result"), nil)
	tr := New(facadeMock)

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
	facadeMock := mocks.NewFacadeService(t)
	facadeMock.EXPECT().UpdateMessage(mock.Anything, int64(100), int64(1), "updated").Return(nil)
	tr := New(facadeMock)

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
	facadeMock := mocks.NewFacadeService(t)
	facadeMock.EXPECT().DeleteMessages(mock.Anything, int64(100), []int64{1, 2, 3}).Return(nil)
	tr := New(facadeMock)

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
	facadeMock := mocks.NewFacadeService(t)
	facadeMock.EXPECT().GetMessageLink(mock.Anything, int64(100), int64(1)).
		Return("https://t.me/c/100/1", nil)
	tr := New(facadeMock)

	// Act
	resp, err := tr.GetMessageLink(context.Background(), &pb.GetMessageLinkRequest{ChatId: 100, MessageId: 1})

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "https://t.me/c/100/1", resp.GetLink())
}

func TestGetMessageLinkInfo_Success(t *testing.T) {
	t.Parallel()

	// Arrange
	facadeMock := mocks.NewFacadeService(t)
	facadeMock.EXPECT().GetMessageLinkInfo(mock.Anything, "https://t.me/c/100/1").
		Return(&client.MessageLinkInfo{ChatId: 100, Message: &client.Message{Id: 1}}, nil)
	tr := New(facadeMock)

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
	facadeMock := mocks.NewFacadeService(t)
	facadeMock.EXPECT().GetChatHistory(mock.Anything, int64(100), int64(0), int32(0), int32(10)).
		Return([]*client.Message{textMessage(100, 1, "msg1"), textMessage(100, 2, "msg2")}, nil)
	tr := New(facadeMock)

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
	facadeMock := mocks.NewFacadeService(t)
	facadeMock.EXPECT().GetChatHistory(mock.Anything, int64(100), int64(0), int32(0), int32(10)).
		Return(nil, nil)
	tr := New(facadeMock)

	// Act
	resp, err := tr.GetChatHistory(context.Background(), &pb.GetChatHistoryRequest{
		ChatId: 100, Limit: 10,
	})

	// Assert
	require.NoError(t, err)
	assert.Empty(t, resp.GetMessages())
}

func TestClientMessageToProto_Nil(t *testing.T) {
	t.Parallel()
	assert.Nil(t, clientMessageToProto(nil))
}

func TestClientMessageToProto_WithForwardInfo(t *testing.T) {
	t.Parallel()

	// Arrange
	msg := textMessage(100, 1, "hello")
	msg.ForwardInfo = &client.MessageForwardInfo{}

	// Act
	result := clientMessageToProto(msg)

	// Assert
	assert.True(t, result.GetForward())
	assert.Equal(t, "hello", result.GetText())
}

func TestClientMessageToProto_PhotoContent(t *testing.T) {
	t.Parallel()

	// Arrange
	msg := &client.Message{
		ChatId:  100,
		Id:      1,
		Content: &client.MessagePhoto{Caption: &client.FormattedText{Text: "photo caption"}},
	}

	// Act
	result := clientMessageToProto(msg)

	// Assert
	assert.Equal(t, "photo caption", result.GetText())
}

func TestClientMessageToProto_PhotoContent_NilCaption(t *testing.T) {
	t.Parallel()

	// Arrange
	msg := &client.Message{
		ChatId:  100,
		Id:      1,
		Content: &client.MessagePhoto{Caption: nil},
	}

	// Act
	result := clientMessageToProto(msg)

	// Assert
	assert.Equal(t, "", result.GetText())
}

func TestClientMessageToProto_VideoContent(t *testing.T) {
	t.Parallel()

	// Arrange
	msg := &client.Message{
		ChatId:  100,
		Id:      1,
		Content: &client.MessageVideo{Caption: &client.FormattedText{Text: "video caption"}},
	}

	// Act
	result := clientMessageToProto(msg)

	// Assert
	assert.Equal(t, "video caption", result.GetText())
}

func TestClientMessageToProto_VideoContent_NilCaption(t *testing.T) {
	t.Parallel()

	// Arrange
	msg := &client.Message{
		ChatId:  100,
		Id:      1,
		Content: &client.MessageVideo{Caption: nil},
	}

	// Act
	result := clientMessageToProto(msg)

	// Assert
	assert.Equal(t, "", result.GetText())
}

func TestClientMessageToProto_DocumentContent(t *testing.T) {
	t.Parallel()

	// Arrange
	msg := &client.Message{
		ChatId:  100,
		Id:      1,
		Content: &client.MessageDocument{Caption: &client.FormattedText{Text: "doc caption"}},
	}

	// Act
	result := clientMessageToProto(msg)

	// Assert
	assert.Equal(t, "doc caption", result.GetText())
}

func TestClientMessageToProto_DocumentContent_NilCaption(t *testing.T) {
	t.Parallel()

	// Arrange
	msg := &client.Message{
		ChatId:  100,
		Id:      1,
		Content: &client.MessageDocument{Caption: nil},
	}

	// Act
	result := clientMessageToProto(msg)

	// Assert
	assert.Equal(t, "", result.GetText())
}

func TestClientMessageToProto_UnknownContent(t *testing.T) {
	t.Parallel()

	// Arrange
	msg := &client.Message{
		ChatId:  100,
		Id:      1,
		Content: &client.MessageSticker{},
	}

	// Act
	result := clientMessageToProto(msg)

	// Assert
	assert.Equal(t, "", result.GetText())
	assert.Equal(t, int64(100), result.GetChatId())
}

func TestClientMessageToProto_NilText(t *testing.T) {
	t.Parallel()

	// Arrange
	msg := &client.Message{
		ChatId:  100,
		Id:      1,
		Content: &client.MessageText{Text: nil},
	}

	// Act
	result := clientMessageToProto(msg)

	// Assert
	assert.Equal(t, "", result.GetText())
}

func TestGetMessages_WithNilInSlice(t *testing.T) {
	t.Parallel()

	// Arrange
	facadeMock := mocks.NewFacadeService(t)
	facadeMock.EXPECT().GetMessages(mock.Anything, int64(100), []int64{1, 2}).
		Return([]*client.Message{textMessage(100, 1, "hello"), nil}, nil)
	tr := New(facadeMock)

	// Act
	resp, err := tr.GetMessages(context.Background(), &pb.GetMessagesRequest{
		ChatId:     100,
		MessageIds: []int64{1, 2},
	})

	// Assert
	require.NoError(t, err)
	assert.Len(t, resp.GetMessages(), 1)
}

func TestGetChatHistory_Error(t *testing.T) {
	t.Parallel()

	// Arrange
	facadeMock := mocks.NewFacadeService(t)
	facadeMock.EXPECT().GetChatHistory(mock.Anything, int64(100), int64(0), int32(0), int32(10)).
		Return(nil, errors.New("history failed"))
	tr := New(facadeMock)

	// Act
	resp, err := tr.GetChatHistory(context.Background(), &pb.GetChatHistoryRequest{
		ChatId: 100, Limit: 10,
	})

	// Assert
	require.Error(t, err)
	assert.Nil(t, resp)
	assert.Equal(t, codes.Internal, status.Code(err))
}

func TestSendMessageAlbum_FacadeError(t *testing.T) {
	t.Parallel()

	// Arrange
	facadeMock := mocks.NewFacadeService(t)
	facadeMock.EXPECT().SendMessageAlbum(mock.Anything, int64(100), mock.Anything).
		Return(errors.New("album failed"))
	tr := New(facadeMock)

	// Act
	_, err := tr.SendMessageAlbum(context.Background(), &pb.SendMessageAlbumRequest{
		Messages: []*pb.NewMessage{{ChatId: 100, Text: "one"}},
	})

	// Assert
	require.Error(t, err)
	assert.Equal(t, codes.Internal, status.Code(err))
}

func TestForwardMessage_Error(t *testing.T) {
	t.Parallel()

	// Arrange
	facadeMock := mocks.NewFacadeService(t)
	facadeMock.EXPECT().ForwardMessage(mock.Anything, int64(100), int64(1)).
		Return(errors.New("forward failed"))
	tr := New(facadeMock)

	// Act
	_, err := tr.ForwardMessage(context.Background(), &pb.ForwardMessageRequest{
		ChatId: 100, MessageId: 1,
	})

	// Assert
	require.Error(t, err)
	assert.Equal(t, codes.Internal, status.Code(err))
}

func TestGetMessage_Error(t *testing.T) {
	t.Parallel()

	// Arrange
	facadeMock := mocks.NewFacadeService(t)
	facadeMock.EXPECT().GetMessage(mock.Anything, int64(100), int64(1)).
		Return(nil, errors.New("not found"))
	tr := New(facadeMock)

	// Act
	_, err := tr.GetMessage(context.Background(), &pb.GetMessageRequest{ChatId: 100, MessageId: 1})

	// Assert
	require.Error(t, err)
	assert.Equal(t, codes.Internal, status.Code(err))
}

func TestUpdateMessage_FacadeError(t *testing.T) {
	t.Parallel()

	// Arrange
	facadeMock := mocks.NewFacadeService(t)
	facadeMock.EXPECT().UpdateMessage(mock.Anything, int64(100), int64(1), "text").
		Return(errors.New("update failed"))
	tr := New(facadeMock)

	// Act
	_, err := tr.UpdateMessage(context.Background(), &pb.UpdateMessageRequest{
		Message: &pb.Message{ChatId: 100, Id: 1, Text: "text"},
	})

	// Assert
	require.Error(t, err)
	assert.Equal(t, codes.Internal, status.Code(err))
}

func TestDeleteMessages_Error(t *testing.T) {
	t.Parallel()

	// Arrange
	facadeMock := mocks.NewFacadeService(t)
	facadeMock.EXPECT().DeleteMessages(mock.Anything, int64(100), []int64{1}).
		Return(errors.New("delete failed"))
	tr := New(facadeMock)

	// Act
	_, err := tr.DeleteMessages(context.Background(), &pb.DeleteMessagesRequest{
		ChatId: 100, MessageIds: []int64{1},
	})

	// Assert
	require.Error(t, err)
	assert.Equal(t, codes.Internal, status.Code(err))
}

func TestGetMessageLink_Error(t *testing.T) {
	t.Parallel()

	// Arrange
	facadeMock := mocks.NewFacadeService(t)
	facadeMock.EXPECT().GetMessageLink(mock.Anything, int64(100), int64(1)).
		Return("", errors.New("link failed"))
	tr := New(facadeMock)

	// Act
	_, err := tr.GetMessageLink(context.Background(), &pb.GetMessageLinkRequest{ChatId: 100, MessageId: 1})

	// Assert
	require.Error(t, err)
	assert.Equal(t, codes.Internal, status.Code(err))
}

func TestGetMessageLinkInfo_Error(t *testing.T) {
	t.Parallel()

	// Arrange
	facadeMock := mocks.NewFacadeService(t)
	facadeMock.EXPECT().GetMessageLinkInfo(mock.Anything, "https://t.me/c/1/1").
		Return(nil, errors.New("info failed"))
	tr := New(facadeMock)

	// Act
	_, err := tr.GetMessageLinkInfo(context.Background(), &pb.GetMessageLinkInfoRequest{Link: "https://t.me/c/1/1"})

	// Assert
	require.Error(t, err)
	assert.Equal(t, codes.Internal, status.Code(err))
}

func TestGetMessageLinkInfo_NilInfo(t *testing.T) {
	t.Parallel()

	// Arrange
	facadeMock := mocks.NewFacadeService(t)
	facadeMock.EXPECT().GetMessageLinkInfo(mock.Anything, "https://t.me/c/1/1").
		Return(nil, nil)
	tr := New(facadeMock)

	// Act
	resp, err := tr.GetMessageLinkInfo(context.Background(), &pb.GetMessageLinkInfoRequest{Link: "https://t.me/c/1/1"})

	// Assert
	require.NoError(t, err)
	assert.Equal(t, int64(0), resp.GetMessage().GetChatId())
	assert.Equal(t, int64(0), resp.GetMessage().GetId())
}

func TestGetMessageLinkInfo_NilMessage(t *testing.T) {
	t.Parallel()

	// Arrange
	facadeMock := mocks.NewFacadeService(t)
	facadeMock.EXPECT().GetMessageLinkInfo(mock.Anything, "https://t.me/c/1/1").
		Return(&client.MessageLinkInfo{ChatId: 42, Message: nil}, nil)
	tr := New(facadeMock)

	// Act
	resp, err := tr.GetMessageLinkInfo(context.Background(), &pb.GetMessageLinkInfoRequest{Link: "https://t.me/c/1/1"})

	// Assert
	require.NoError(t, err)
	assert.Equal(t, int64(42), resp.GetMessage().GetChatId())
	assert.Equal(t, int64(0), resp.GetMessage().GetId())
}
