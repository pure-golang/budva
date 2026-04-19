package facade

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/zelenin/go-tdlib/client"

	"github.com/pure-golang/budva-claude/internal/domain"
	"github.com/pure-golang/budva-claude/internal/service/facade/mocks"
)

func TestSendMessage_Success(t *testing.T) {
	t.Parallel()

	// Arrange
	gw := mocks.NewTelegramRepo(t)
	svc := New(gw)
	gw.EXPECT().SendMessage(mock.MatchedBy(func(req *client.SendMessageRequest) bool {
		if req.ChatId != 100 {
			return false
		}
		text, ok := req.InputMessageContent.(*client.InputMessageText)
		return ok && text.Text != nil && text.Text.Text == "hello"
	})).Return(&client.Message{Id: 1}, nil)

	// Act
	err := svc.SendMessage(context.Background(), 100, "hello")

	// Assert
	require.NoError(t, err)
}

func TestSendMessageAlbum_with_files_and_text(t *testing.T) {
	t.Parallel()

	// Arrange
	gw := mocks.NewTelegramRepo(t)
	svc := New(gw)
	items := []domain.AlbumItem{
		{Text: "photo caption", FilePath: "/tmp/image.jpg"},
		{Text: "just text"},
		{Text: "video", FilePath: "/tmp/clip.mp4"},
	}
	gw.EXPECT().SendMessageAlbum(mock.MatchedBy(func(req *client.SendMessageAlbumRequest) bool {
		if req.ChatId != 200 || len(req.InputMessageContents) != 3 {
			return false
		}
		photo, photoOK := req.InputMessageContents[0].(*client.InputMessagePhoto)
		text, textOK := req.InputMessageContents[1].(*client.InputMessageText)
		video, videoOK := req.InputMessageContents[2].(*client.InputMessageVideo)
		if !photoOK || !textOK || !videoOK {
			return false
		}
		photoFile, _ := photo.Photo.(*client.InputFileLocal)
		videoFile, _ := video.Video.(*client.InputFileLocal)
		return photoFile != nil && photoFile.Path == "/tmp/image.jpg" &&
			text.Text != nil && text.Text.Text == "just text" &&
			videoFile != nil && videoFile.Path == "/tmp/clip.mp4"
	})).Return(&client.Messages{TotalCount: 3}, nil)

	// Act
	err := svc.SendMessageAlbum(context.Background(), 200, items)

	// Assert
	require.NoError(t, err)
}

func TestSendMessageAlbum_empty(t *testing.T) {
	t.Parallel()

	// Arrange
	gw := mocks.NewTelegramRepo(t)
	svc := New(gw)
	gw.EXPECT().SendMessageAlbum(mock.Anything).Return(&client.Messages{}, nil)

	// Act
	err := svc.SendMessageAlbum(context.Background(), 200, nil)

	// Assert
	require.NoError(t, err)
}

func TestGetStatus_GetMe_error(t *testing.T) {
	t.Parallel()

	// Arrange
	gw := mocks.NewTelegramRepo(t)
	svc := New(gw)
	gw.EXPECT().GetMe().Return(nil, errors.New("not authorized"))

	// Act
	resp, err := svc.GetStatus(context.Background())

	// Assert
	require.Error(t, err)
	assert.Nil(t, resp)
}

func TestForwardMessage_Success(t *testing.T) {
	t.Parallel()

	// Arrange
	gw := mocks.NewTelegramRepo(t)
	svc := New(gw)
	gw.EXPECT().ForwardMessages(mock.MatchedBy(func(req *client.ForwardMessagesRequest) bool {
		return req.ChatId == 100 && req.FromChatId == 100 &&
			len(req.MessageIds) == 1 && req.MessageIds[0] == 5
	})).Return(&client.Messages{TotalCount: 1}, nil)

	// Act
	err := svc.ForwardMessage(context.Background(), 100, 5)

	// Assert
	require.NoError(t, err)
}

func TestDeleteMessages_Success(t *testing.T) {
	t.Parallel()

	// Arrange
	gw := mocks.NewTelegramRepo(t)
	svc := New(gw)
	gw.EXPECT().DeleteMessages(mock.MatchedBy(func(req *client.DeleteMessagesRequest) bool {
		return req.ChatId == 100 && req.Revoke &&
			len(req.MessageIds) == 2 && req.MessageIds[0] == 1 && req.MessageIds[1] == 2
	})).Return(&client.Ok{}, nil)

	// Act
	err := svc.DeleteMessages(context.Background(), 100, []int64{1, 2})

	// Assert
	require.NoError(t, err)
}

func TestUpdateMessage_Success(t *testing.T) {
	t.Parallel()

	// Arrange
	gw := mocks.NewTelegramRepo(t)
	svc := New(gw)
	gw.EXPECT().EditMessageText(mock.MatchedBy(func(req *client.EditMessageTextRequest) bool {
		if req.ChatId != 100 || req.MessageId != 5 {
			return false
		}
		text, ok := req.InputMessageContent.(*client.InputMessageText)
		return ok && text.Text != nil && text.Text.Text == "updated"
	})).Return(&client.Message{Id: 5}, nil)

	// Act
	err := svc.UpdateMessage(context.Background(), 100, 5, "updated")

	// Assert
	require.NoError(t, err)
}
