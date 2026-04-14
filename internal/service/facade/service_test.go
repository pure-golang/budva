package facade

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/pure-golang/budva-claude/internal/domain"
	"github.com/pure-golang/budva-claude/internal/service/facade/mocks"
)

func TestSendMessage_Success(t *testing.T) {
	t.Parallel()

	// Arrange
	gw := mocks.NewTelegramGateway(t)
	svc := New(gw)
	gw.EXPECT().SendMessage(mock.Anything, int64(100), mock.MatchedBy(func(c domain.InputMessageContent) bool {
		return c.Type == domain.ContentText && c.Text.Text == "hello"
	})).Return(int64(1), nil)

	// Act
	err := svc.SendMessage(context.Background(), 100, "hello")

	// Assert
	require.NoError(t, err)
}

func TestSendMessageAlbum_with_files_and_text(t *testing.T) {
	t.Parallel()

	// Arrange
	gw := mocks.NewTelegramGateway(t)
	svc := New(gw)
	items := []domain.AlbumItem{
		{Text: "photo caption", FilePath: "/tmp/image.jpg"},
		{Text: "just text"},
		{Text: "video", FilePath: "/tmp/clip.mp4"},
	}
	gw.EXPECT().SendMessageAlbum(mock.Anything, int64(200), mock.MatchedBy(func(contents []domain.InputMessageContent) bool {
		if len(contents) != 3 {
			return false
		}
		return contents[0].Type == domain.ContentPhoto &&
			contents[0].FilePath == "/tmp/image.jpg" &&
			contents[1].Type == domain.ContentText &&
			contents[1].FilePath == "" &&
			contents[2].Type == domain.ContentVideo &&
			contents[2].FilePath == "/tmp/clip.mp4"
	})).Return([]int64{1, 2, 3}, nil)

	// Act
	err := svc.SendMessageAlbum(context.Background(), 200, items)

	// Assert
	require.NoError(t, err)
}

func TestSendMessageAlbum_empty(t *testing.T) {
	t.Parallel()

	// Arrange
	gw := mocks.NewTelegramGateway(t)
	svc := New(gw)
	gw.EXPECT().SendMessageAlbum(mock.Anything, int64(200), mock.Anything).Return(nil, nil)

	// Act
	err := svc.SendMessageAlbum(context.Background(), 200, nil)

	// Assert
	require.NoError(t, err)
}

func TestGetStatus_Success(t *testing.T) {
	t.Parallel()

	// Arrange
	gw := mocks.NewTelegramGateway(t)
	svc := New(gw)
	gw.EXPECT().GetOption(mock.Anything, "version").Return("1.8.30", nil)
	gw.EXPECT().GetMe(mock.Anything).Return(int64(12345), nil)

	// Act
	resp, err := svc.GetStatus(context.Background())

	// Assert
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, "1.8.30", resp.TDLibVersion)
	assert.Equal(t, int64(12345), resp.UserID)
	// В тестовом бинаре debug.ReadBuildInfo() возвращает "(devel)"
	assert.Equal(t, "(devel)", resp.ReleaseVersion)
}

func TestGetStatus_GetOption_error(t *testing.T) {
	t.Parallel()

	// Arrange
	gw := mocks.NewTelegramGateway(t)
	svc := New(gw)
	gw.EXPECT().GetOption(mock.Anything, "version").Return("", errors.New("connection lost"))

	// Act
	resp, err := svc.GetStatus(context.Background())

	// Assert
	require.Error(t, err)
	assert.Nil(t, resp)
}

func TestGetStatus_GetMe_error(t *testing.T) {
	t.Parallel()

	// Arrange
	gw := mocks.NewTelegramGateway(t)
	svc := New(gw)
	gw.EXPECT().GetOption(mock.Anything, "version").Return("1.8.30", nil)
	gw.EXPECT().GetMe(mock.Anything).Return(int64(0), errors.New("not authorized"))

	// Act
	resp, err := svc.GetStatus(context.Background())

	// Assert
	require.Error(t, err)
	assert.Nil(t, resp)
}

func TestForwardMessage_Success(t *testing.T) {
	t.Parallel()

	// Arrange
	gw := mocks.NewTelegramGateway(t)
	svc := New(gw)
	gw.EXPECT().ForwardMessages(mock.Anything, int64(100), int64(100), []int64{int64(5)}).Return([]int64{int64(6)}, nil)

	// Act
	err := svc.ForwardMessage(context.Background(), 100, 5)

	// Assert
	require.NoError(t, err)
}

func TestDeleteMessages_Success(t *testing.T) {
	t.Parallel()

	// Arrange
	gw := mocks.NewTelegramGateway(t)
	svc := New(gw)
	gw.EXPECT().DeleteMessages(mock.Anything, int64(100), []int64{int64(1), int64(2)}, true).Return(nil)

	// Act
	err := svc.DeleteMessages(context.Background(), 100, []int64{1, 2})

	// Assert
	require.NoError(t, err)
}

func TestUpdateMessage_Success(t *testing.T) {
	t.Parallel()

	// Arrange
	gw := mocks.NewTelegramGateway(t)
	svc := New(gw)
	gw.EXPECT().EditMessageText(mock.Anything, int64(100), int64(5), &domain.FormattedText{Text: "updated"}).Return(nil)

	// Act
	err := svc.UpdateMessage(context.Background(), 100, 5, "updated")

	// Assert
	require.NoError(t, err)
}
