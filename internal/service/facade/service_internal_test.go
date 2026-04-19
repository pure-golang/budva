package facade

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zelenin/go-tdlib/client"
)

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
