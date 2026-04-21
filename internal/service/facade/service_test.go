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

func TestNew(t *testing.T) {
	t.Parallel()

	// Arrange
	gw := mocks.NewTelegramRepo(t)

	// Act
	svc := New(gw)

	// Assert
	require.NotNil(t, svc)
}

func TestService_GetMessage(t *testing.T) {
	t.Parallel()

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		// Arrange
		gw := mocks.NewTelegramRepo(t)
		svc := New(gw)
		want := &client.Message{Id: 42}
		gw.EXPECT().GetMessage(mock.MatchedBy(func(req *client.GetMessageRequest) bool {
			return req.ChatId == 100 && req.MessageId == 42
		})).Return(want, nil)

		// Act
		got, err := svc.GetMessage(context.Background(), 100, 42)

		// Assert
		require.NoError(t, err)
		assert.Same(t, want, got)
	})

	t.Run("error", func(t *testing.T) {
		t.Parallel()

		// Arrange
		gw := mocks.NewTelegramRepo(t)
		svc := New(gw)
		wantErr := errors.New("not found")
		gw.EXPECT().GetMessage(mock.Anything).Return(nil, wantErr)

		// Act
		got, err := svc.GetMessage(context.Background(), 1, 2)

		// Assert
		require.ErrorIs(t, err, wantErr)
		assert.Nil(t, got)
	})
}

func TestService_SendMessage(t *testing.T) {
	t.Parallel()

	t.Run("success", func(t *testing.T) {
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
	})

	t.Run("error", func(t *testing.T) {
		t.Parallel()

		// Arrange
		gw := mocks.NewTelegramRepo(t)
		svc := New(gw)
		wantErr := errors.New("send failed")
		gw.EXPECT().SendMessage(mock.Anything).Return(nil, wantErr)

		// Act
		err := svc.SendMessage(context.Background(), 100, "hello")

		// Assert
		require.ErrorIs(t, err, wantErr)
	})
}

func TestService_SendMessageAlbum(t *testing.T) {
	t.Parallel()

	t.Run("mixed files and text", func(t *testing.T) {
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
	})

	t.Run("nil items", func(t *testing.T) {
		t.Parallel()

		// Arrange
		gw := mocks.NewTelegramRepo(t)
		svc := New(gw)
		gw.EXPECT().SendMessageAlbum(mock.MatchedBy(func(req *client.SendMessageAlbumRequest) bool {
			return req.ChatId == 200 && len(req.InputMessageContents) == 0
		})).Return(&client.Messages{}, nil)

		// Act
		err := svc.SendMessageAlbum(context.Background(), 200, nil)

		// Assert
		require.NoError(t, err)
	})

	t.Run("all supported file extensions", func(t *testing.T) {
		t.Parallel()

		// Arrange
		type contentCheck func(client.InputMessageContent) bool
		extensions := []struct {
			name     string
			filePath string
			check    contentCheck
		}{
			{
				name:     "photo jpeg",
				filePath: "/tmp/a.jpeg",
				check: func(c client.InputMessageContent) bool {
					_, ok := c.(*client.InputMessagePhoto)
					return ok
				},
			},
			{
				name:     "photo png uppercase",
				filePath: "/tmp/a.PNG",
				check: func(c client.InputMessageContent) bool {
					_, ok := c.(*client.InputMessagePhoto)
					return ok
				},
			},
			{
				name:     "photo webp",
				filePath: "/tmp/a.webp",
				check: func(c client.InputMessageContent) bool {
					_, ok := c.(*client.InputMessagePhoto)
					return ok
				},
			},
			{
				name:     "video mov",
				filePath: "/tmp/a.mov",
				check: func(c client.InputMessageContent) bool {
					_, ok := c.(*client.InputMessageVideo)
					return ok
				},
			},
			{
				name:     "video avi",
				filePath: "/tmp/a.avi",
				check: func(c client.InputMessageContent) bool {
					_, ok := c.(*client.InputMessageVideo)
					return ok
				},
			},
			{
				name:     "video mkv",
				filePath: "/tmp/a.mkv",
				check: func(c client.InputMessageContent) bool {
					_, ok := c.(*client.InputMessageVideo)
					return ok
				},
			},
			{
				name:     "audio mp3",
				filePath: "/tmp/a.mp3",
				check: func(c client.InputMessageContent) bool {
					_, ok := c.(*client.InputMessageAudio)
					return ok
				},
			},
			{
				name:     "audio ogg",
				filePath: "/tmp/a.ogg",
				check: func(c client.InputMessageContent) bool {
					_, ok := c.(*client.InputMessageAudio)
					return ok
				},
			},
			{
				name:     "audio m4a",
				filePath: "/tmp/a.m4a",
				check: func(c client.InputMessageContent) bool {
					_, ok := c.(*client.InputMessageAudio)
					return ok
				},
			},
			{
				name:     "audio flac",
				filePath: "/tmp/a.flac",
				check: func(c client.InputMessageContent) bool {
					_, ok := c.(*client.InputMessageAudio)
					return ok
				},
			},
			{
				name:     "audio wav",
				filePath: "/tmp/a.wav",
				check: func(c client.InputMessageContent) bool {
					_, ok := c.(*client.InputMessageAudio)
					return ok
				},
			},
			{
				name:     "animation gif",
				filePath: "/tmp/a.gif",
				check: func(c client.InputMessageContent) bool {
					_, ok := c.(*client.InputMessageAnimation)
					return ok
				},
			},
			{
				name:     "document unknown",
				filePath: "/tmp/a.pdf",
				check: func(c client.InputMessageContent) bool {
					_, ok := c.(*client.InputMessageDocument)
					return ok
				},
			},
			{
				name:     "document no extension",
				filePath: "/tmp/plainfile",
				check: func(c client.InputMessageContent) bool {
					_, ok := c.(*client.InputMessageDocument)
					return ok
				},
			},
		}
		for _, tt := range extensions {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				// Arrange
				gw := mocks.NewTelegramRepo(t)
				svc := New(gw)
				gw.EXPECT().SendMessageAlbum(mock.MatchedBy(func(req *client.SendMessageAlbumRequest) bool {
					return len(req.InputMessageContents) == 1 && tt.check(req.InputMessageContents[0])
				})).Return(&client.Messages{}, nil)

				// Act
				err := svc.SendMessageAlbum(context.Background(), 1, []domain.AlbumItem{{FilePath: tt.filePath, Text: "cap"}})

				// Assert
				require.NoError(t, err)
			})
		}
	})

	t.Run("error", func(t *testing.T) {
		t.Parallel()

		// Arrange
		gw := mocks.NewTelegramRepo(t)
		svc := New(gw)
		wantErr := errors.New("album failed")
		gw.EXPECT().SendMessageAlbum(mock.Anything).Return(nil, wantErr)

		// Act
		err := svc.SendMessageAlbum(context.Background(), 1, []domain.AlbumItem{{Text: "x"}})

		// Assert
		require.ErrorIs(t, err, wantErr)
	})
}

func TestService_ForwardMessage(t *testing.T) {
	t.Parallel()

	t.Run("success", func(t *testing.T) {
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
	})

	t.Run("error", func(t *testing.T) {
		t.Parallel()

		// Arrange
		gw := mocks.NewTelegramRepo(t)
		svc := New(gw)
		wantErr := errors.New("forward failed")
		gw.EXPECT().ForwardMessages(mock.Anything).Return(nil, wantErr)

		// Act
		err := svc.ForwardMessage(context.Background(), 100, 5)

		// Assert
		require.ErrorIs(t, err, wantErr)
	})
}

func TestService_UpdateMessage(t *testing.T) {
	t.Parallel()

	t.Run("success", func(t *testing.T) {
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
	})

	t.Run("error", func(t *testing.T) {
		t.Parallel()

		// Arrange
		gw := mocks.NewTelegramRepo(t)
		svc := New(gw)
		wantErr := errors.New("edit failed")
		gw.EXPECT().EditMessageText(mock.Anything).Return(nil, wantErr)

		// Act
		err := svc.UpdateMessage(context.Background(), 100, 5, "updated")

		// Assert
		require.ErrorIs(t, err, wantErr)
	})
}

func TestService_DeleteMessages(t *testing.T) {
	t.Parallel()

	t.Run("success", func(t *testing.T) {
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
	})

	t.Run("empty slice", func(t *testing.T) {
		t.Parallel()

		// Arrange
		gw := mocks.NewTelegramRepo(t)
		svc := New(gw)
		gw.EXPECT().DeleteMessages(mock.MatchedBy(func(req *client.DeleteMessagesRequest) bool {
			return req.ChatId == 100 && req.Revoke && len(req.MessageIds) == 0
		})).Return(&client.Ok{}, nil)

		// Act
		err := svc.DeleteMessages(context.Background(), 100, nil)

		// Assert
		require.NoError(t, err)
	})

	t.Run("error", func(t *testing.T) {
		t.Parallel()

		// Arrange
		gw := mocks.NewTelegramRepo(t)
		svc := New(gw)
		wantErr := errors.New("delete failed")
		gw.EXPECT().DeleteMessages(mock.Anything).Return(nil, wantErr)

		// Act
		err := svc.DeleteMessages(context.Background(), 100, []int64{1})

		// Assert
		require.ErrorIs(t, err, wantErr)
	})
}

func TestService_GetChatHistory(t *testing.T) {
	t.Parallel()

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		// Arrange
		gw := mocks.NewTelegramRepo(t)
		svc := New(gw)
		wantMsgs := []*client.Message{{Id: 1}, {Id: 2}}
		gw.EXPECT().GetChatHistory(mock.MatchedBy(func(req *client.GetChatHistoryRequest) bool {
			return req.ChatId == 100 && req.FromMessageId == 5 && req.Offset == -1 && req.Limit == 50
		})).Return(&client.Messages{Messages: wantMsgs, TotalCount: 2}, nil)

		// Act
		got, err := svc.GetChatHistory(context.Background(), 100, 5, -1, 50)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, wantMsgs, got)
	})

	t.Run("error", func(t *testing.T) {
		t.Parallel()

		// Arrange
		gw := mocks.NewTelegramRepo(t)
		svc := New(gw)
		wantErr := errors.New("history failed")
		gw.EXPECT().GetChatHistory(mock.Anything).Return(nil, wantErr)

		// Act
		got, err := svc.GetChatHistory(context.Background(), 100, 0, 0, 10)

		// Assert
		require.ErrorIs(t, err, wantErr)
		assert.Nil(t, got)
	})
}

func TestService_GetMessages(t *testing.T) {
	t.Parallel()

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		// Arrange
		gw := mocks.NewTelegramRepo(t)
		svc := New(gw)
		wantMsgs := []*client.Message{{Id: 1}, {Id: 2}}
		gw.EXPECT().GetMessages(mock.MatchedBy(func(req *client.GetMessagesRequest) bool {
			return req.ChatId == 100 && len(req.MessageIds) == 2 &&
				req.MessageIds[0] == 1 && req.MessageIds[1] == 2
		})).Return(&client.Messages{Messages: wantMsgs, TotalCount: 2}, nil)

		// Act
		got, err := svc.GetMessages(context.Background(), 100, []int64{1, 2})

		// Assert
		require.NoError(t, err)
		assert.Equal(t, wantMsgs, got)
	})

	t.Run("error", func(t *testing.T) {
		t.Parallel()

		// Arrange
		gw := mocks.NewTelegramRepo(t)
		svc := New(gw)
		wantErr := errors.New("get messages failed")
		gw.EXPECT().GetMessages(mock.Anything).Return(nil, wantErr)

		// Act
		got, err := svc.GetMessages(context.Background(), 100, []int64{1})

		// Assert
		require.ErrorIs(t, err, wantErr)
		assert.Nil(t, got)
	})
}

func TestService_GetMessageLink(t *testing.T) {
	t.Parallel()

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		// Arrange
		gw := mocks.NewTelegramRepo(t)
		svc := New(gw)
		gw.EXPECT().GetMessageLink(mock.MatchedBy(func(req *client.GetMessageLinkRequest) bool {
			return req.ChatId == 100 && req.MessageId == 5 && !req.ForAlbum
		})).Return(&client.MessageLink{Link: "https://t.me/x/5"}, nil)

		// Act
		got, err := svc.GetMessageLink(context.Background(), 100, 5)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, "https://t.me/x/5", got)
	})

	t.Run("error", func(t *testing.T) {
		t.Parallel()

		// Arrange
		gw := mocks.NewTelegramRepo(t)
		svc := New(gw)
		wantErr := errors.New("link failed")
		gw.EXPECT().GetMessageLink(mock.Anything).Return(nil, wantErr)

		// Act
		got, err := svc.GetMessageLink(context.Background(), 100, 5)

		// Assert
		require.ErrorIs(t, err, wantErr)
		assert.Empty(t, got)
	})
}

func TestService_GetMessageLinkInfo(t *testing.T) {
	t.Parallel()

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		// Arrange
		gw := mocks.NewTelegramRepo(t)
		svc := New(gw)
		want := &client.MessageLinkInfo{ChatId: 100, IsPublic: true}
		gw.EXPECT().GetMessageLinkInfo(mock.MatchedBy(func(req *client.GetMessageLinkInfoRequest) bool {
			return req.Url == "https://t.me/x/5"
		})).Return(want, nil)

		// Act
		got, err := svc.GetMessageLinkInfo(context.Background(), "https://t.me/x/5")

		// Assert
		require.NoError(t, err)
		assert.Same(t, want, got)
	})

	t.Run("error", func(t *testing.T) {
		t.Parallel()

		// Arrange
		gw := mocks.NewTelegramRepo(t)
		svc := New(gw)
		wantErr := errors.New("link info failed")
		gw.EXPECT().GetMessageLinkInfo(mock.Anything).Return(nil, wantErr)

		// Act
		got, err := svc.GetMessageLinkInfo(context.Background(), "https://t.me/x/5")

		// Assert
		require.ErrorIs(t, err, wantErr)
		assert.Nil(t, got)
	})
}

func TestService_GetStatus(t *testing.T) {
	t.Parallel()

	t.Run("success returns version and user id", func(t *testing.T) {
		t.Parallel()

		// Arrange
		gw := mocks.NewTelegramRepo(t)
		svc := New(gw)
		gw.EXPECT().GetOption(mock.Anything).Return(&client.OptionValueString{Value: "1.8.35"}, nil)
		gw.EXPECT().GetMe().Return(&client.User{Id: 42}, nil)

		// Act
		resp, err := svc.GetStatus(context.Background())

		// Assert
		require.NoError(t, err)
		require.NotNil(t, resp)
		assert.Equal(t, int64(42), resp.UserID)
		assert.Equal(t, "1.8.35", resp.TDLibVersion)
		// ReleaseVersion всегда заполнен: "unknown" или версия модуля из debug.BuildInfo
		assert.NotEmpty(t, resp.ReleaseVersion)
	})

	t.Run("GetOption error", func(t *testing.T) {
		t.Parallel()

		// Arrange
		gw := mocks.NewTelegramRepo(t)
		svc := New(gw)
		wantErr := errors.New("tdlib unavailable")
		gw.EXPECT().GetOption(mock.Anything).Return(nil, wantErr)

		// Act
		resp, err := svc.GetStatus(context.Background())

		// Assert
		require.ErrorIs(t, err, wantErr)
		assert.Nil(t, resp)
	})

	t.Run("GetMe error", func(t *testing.T) {
		t.Parallel()

		// Arrange
		gw := mocks.NewTelegramRepo(t)
		svc := New(gw)
		wantErr := errors.New("not authorized")
		gw.EXPECT().GetOption(mock.Anything).Return(&client.OptionValueString{Value: "1.8.35"}, nil)
		gw.EXPECT().GetMe().Return(nil, wantErr)

		// Act
		resp, err := svc.GetStatus(context.Background())

		// Assert
		require.ErrorIs(t, err, wantErr)
		assert.Nil(t, resp)
	})
}

func TestInputMessageByFileExt(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		filePath string
		assertOn func(t *testing.T, c client.InputMessageContent, path string)
	}{
		{
			name:     "photo_jpg",
			filePath: "/x/a.jpg",
			assertOn: func(t *testing.T, c client.InputMessageContent, path string) {
				photo, ok := c.(*client.InputMessagePhoto)
				require.True(t, ok)
				local, ok := photo.Photo.(*client.InputFileLocal)
				require.True(t, ok)
				assert.Equal(t, path, local.Path)
				assert.Equal(t, "cap", photo.Caption.Text)
			},
		},
		{
			name:     "photo_jpeg_uppercase",
			filePath: "/x/a.JPEG",
			assertOn: func(t *testing.T, c client.InputMessageContent, _ string) {
				_, ok := c.(*client.InputMessagePhoto)
				assert.True(t, ok)
			},
		},
		{
			name:     "photo_png",
			filePath: "/x/a.png",
			assertOn: func(t *testing.T, c client.InputMessageContent, _ string) {
				_, ok := c.(*client.InputMessagePhoto)
				assert.True(t, ok)
			},
		},
		{
			name:     "photo_webp",
			filePath: "/x/a.webp",
			assertOn: func(t *testing.T, c client.InputMessageContent, _ string) {
				_, ok := c.(*client.InputMessagePhoto)
				assert.True(t, ok)
			},
		},
		{
			name:     "video_mp4",
			filePath: "/x/a.mp4",
			assertOn: func(t *testing.T, c client.InputMessageContent, _ string) {
				_, ok := c.(*client.InputMessageVideo)
				assert.True(t, ok)
			},
		},
		{
			name:     "video_mov",
			filePath: "/x/a.mov",
			assertOn: func(t *testing.T, c client.InputMessageContent, _ string) {
				_, ok := c.(*client.InputMessageVideo)
				assert.True(t, ok)
			},
		},
		{
			name:     "video_avi",
			filePath: "/x/a.avi",
			assertOn: func(t *testing.T, c client.InputMessageContent, _ string) {
				_, ok := c.(*client.InputMessageVideo)
				assert.True(t, ok)
			},
		},
		{
			name:     "video_mkv",
			filePath: "/x/a.mkv",
			assertOn: func(t *testing.T, c client.InputMessageContent, _ string) {
				_, ok := c.(*client.InputMessageVideo)
				assert.True(t, ok)
			},
		},
		{
			name:     "audio_mp3",
			filePath: "/x/a.mp3",
			assertOn: func(t *testing.T, c client.InputMessageContent, _ string) {
				_, ok := c.(*client.InputMessageAudio)
				assert.True(t, ok)
			},
		},
		{
			name:     "audio_ogg",
			filePath: "/x/a.ogg",
			assertOn: func(t *testing.T, c client.InputMessageContent, _ string) {
				_, ok := c.(*client.InputMessageAudio)
				assert.True(t, ok)
			},
		},
		{
			name:     "audio_m4a",
			filePath: "/x/a.m4a",
			assertOn: func(t *testing.T, c client.InputMessageContent, _ string) {
				_, ok := c.(*client.InputMessageAudio)
				assert.True(t, ok)
			},
		},
		{
			name:     "audio_flac",
			filePath: "/x/a.flac",
			assertOn: func(t *testing.T, c client.InputMessageContent, _ string) {
				_, ok := c.(*client.InputMessageAudio)
				assert.True(t, ok)
			},
		},
		{
			name:     "audio_wav",
			filePath: "/x/a.wav",
			assertOn: func(t *testing.T, c client.InputMessageContent, _ string) {
				_, ok := c.(*client.InputMessageAudio)
				assert.True(t, ok)
			},
		},
		{
			name:     "animation_gif",
			filePath: "/x/a.gif",
			assertOn: func(t *testing.T, c client.InputMessageContent, _ string) {
				_, ok := c.(*client.InputMessageAnimation)
				assert.True(t, ok)
			},
		},
		{
			name:     "document_pdf",
			filePath: "/x/a.pdf",
			assertOn: func(t *testing.T, c client.InputMessageContent, _ string) {
				_, ok := c.(*client.InputMessageDocument)
				assert.True(t, ok)
			},
		},
		{
			name:     "document_unknown_ext",
			filePath: "/x/a.xyz",
			assertOn: func(t *testing.T, c client.InputMessageContent, _ string) {
				_, ok := c.(*client.InputMessageDocument)
				assert.True(t, ok)
			},
		},
		{
			name:     "document_no_extension",
			filePath: "/x/plainfile",
			assertOn: func(t *testing.T, c client.InputMessageContent, _ string) {
				_, ok := c.(*client.InputMessageDocument)
				assert.True(t, ok)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			caption := &client.FormattedText{Text: "cap"}

			// Act
			got := inputMessageByFileExt(tt.filePath, caption)

			// Assert
			tt.assertOn(t, got, tt.filePath)
		})
	}
}

func TestReleaseVersion(t *testing.T) {
	t.Parallel()

	// Arrange, Act
	got := releaseVersion()

	// Assert
	// В тестовом бинарнике debug.ReadBuildInfo() может возвращать "(devel)"
	// или пустую main version; в любом случае функция должна вернуть non-empty string
	assert.NotEmpty(t, got)
}
