package domain

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAuthorizationState_String(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		state AuthorizationState
		want  string
	}{
		{name: "wait_phone", state: AuthStateWaitPhone, want: "waitPhone"},
		{name: "wait_code", state: AuthStateWaitCode, want: "waitCode"},
		{name: "wait_password", state: AuthStateWaitPassword, want: "waitPassword"},
		{name: "ready", state: AuthStateReady, want: "ready"},
		{name: "closing", state: AuthStateClosing, want: "closing"},
		{name: "closed", state: AuthStateClosed, want: "closed"},
		{name: "unknown", state: AuthorizationState(99), want: "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Act
			got := tt.state.String()

			// Assert
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestFormattedText_DeepCopy(t *testing.T) {
	t.Parallel()

	t.Run("nil_receiver", func(t *testing.T) {
		t.Parallel()

		// Arrange
		var ft *FormattedText

		// Act
		got := ft.DeepCopy()

		// Assert
		assert.Nil(t, got)
	})

	t.Run("with_entities", func(t *testing.T) {
		t.Parallel()

		// Arrange
		ft := &FormattedText{
			Text: "hello",
			Entities: []TextEntity{
				{Offset: 0, Length: 5, Type: TextEntityBold},
			},
		}

		// Act
		got := ft.DeepCopy()

		// Assert
		require.NotNil(t, got)
		assert.Equal(t, ft.Text, got.Text)
		assert.Equal(t, ft.Entities, got.Entities)
		// Проверка независимости копии
		got.Entities[0].Offset = 99
		assert.NotEqual(t, ft.Entities[0].Offset, got.Entities[0].Offset)
	})

	t.Run("without_entities", func(t *testing.T) {
		t.Parallel()

		// Arrange
		ft := &FormattedText{Text: "plain"}

		// Act
		got := ft.DeepCopy()

		// Assert
		require.NotNil(t, got)
		assert.Equal(t, "plain", got.Text)
		assert.Nil(t, got.Entities)
	})
}

func TestContentTypeByFileExt(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		ext  string
		want MessageContentType
	}{
		{name: "png", ext: ".png", want: ContentPhoto},
		{name: "jpg", ext: ".jpg", want: ContentPhoto},
		{name: "jpeg", ext: ".jpeg", want: ContentPhoto},
		{name: "gif", ext: ".gif", want: ContentPhoto},
		{name: "webp", ext: ".webp", want: ContentPhoto},
		{name: "mp4", ext: ".mp4", want: ContentVideo},
		{name: "mov", ext: ".mov", want: ContentVideo},
		{name: "avi", ext: ".avi", want: ContentVideo},
		{name: "mkv", ext: ".mkv", want: ContentVideo},
		{name: "webm", ext: ".webm", want: ContentVideo},
		{name: "mp3", ext: ".mp3", want: ContentAudio},
		{name: "wav", ext: ".wav", want: ContentAudio},
		{name: "ogg", ext: ".ogg", want: ContentAudio},
		{name: "m4a", ext: ".m4a", want: ContentAudio},
		{name: "aac", ext: ".aac", want: ContentAudio},
		{name: "flac", ext: ".flac", want: ContentAudio},
		{name: "wma", ext: ".wma", want: ContentAudio},
		{name: "opus", ext: ".opus", want: ContentAudio},
		{name: "pdf_fallback_to_document", ext: ".pdf", want: ContentDocument},
		{name: "unknown_ext", ext: ".xyz", want: ContentDocument},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Act
			got := ContentTypeByFileExt(tt.ext)

			// Assert
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestMessageContentType_IsMediaType(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		ct   MessageContentType
		want bool
	}{
		{name: "photo", ct: ContentPhoto, want: true},
		{name: "video", ct: ContentVideo, want: true},
		{name: "document", ct: ContentDocument, want: true},
		{name: "audio", ct: ContentAudio, want: true},
		{name: "animation", ct: ContentAnimation, want: true},
		{name: "voice_note", ct: ContentVoiceNote, want: true},
		{name: "text", ct: ContentText, want: false},
		{name: "system", ct: ContentSystem, want: false},
		{name: "unknown", ct: ContentUnknown, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Act
			got := tt.ct.IsMediaType()

			// Assert
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestMessageContentType_HasCaption(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		ct   MessageContentType
		want bool
	}{
		{name: "photo_has_caption", ct: ContentPhoto, want: true},
		{name: "text_no_caption", ct: ContentText, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Act
			got := tt.ct.HasCaption()

			// Assert
			assert.Equal(t, tt.want, got)
		})
	}
}
