package message

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/pure-golang/budva-claude/internal/domain"
)

func TestGetFormattedText(t *testing.T) {
	t.Parallel()
	svc := New()

	tests := []struct {
		name string
		msg  *domain.Message
		want *domain.FormattedText
	}{
		{
			name: "text message returns formatted text",
			msg: &domain.Message{
				Content: domain.MessageContent{
					Type: domain.ContentText,
					Text: &domain.FormattedText{Text: "hello"},
				},
			},
			want: &domain.FormattedText{Text: "hello"},
		},
		{
			name: "photo message returns caption",
			msg: &domain.Message{
				Content: domain.MessageContent{
					Type: domain.ContentPhoto,
					Text: &domain.FormattedText{Text: "caption"},
				},
			},
			want: &domain.FormattedText{Text: "caption"},
		},
		{
			name: "system message returns nil",
			msg: &domain.Message{
				Content: domain.MessageContent{Type: domain.ContentSystem},
			},
			want: nil,
		},
		{
			name: "unknown content returns nil",
			msg: &domain.Message{
				Content: domain.MessageContent{Type: domain.ContentUnknown},
			},
			want: nil,
		},
		{
			name: "nil message returns nil",
			msg:  nil,
			want: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Act
			got := svc.GetFormattedText(tt.msg)

			// Assert
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestIsSystemMessage(t *testing.T) {
	t.Parallel()
	svc := New()

	tests := []struct {
		name string
		msg  *domain.Message
		want bool
	}{
		{
			name: "system message returns true",
			msg:  &domain.Message{Content: domain.MessageContent{Type: domain.ContentSystem}},
			want: true,
		},
		{
			name: "text message returns false",
			msg:  &domain.Message{Content: domain.MessageContent{Type: domain.ContentText}},
			want: false,
		},
		{
			name: "nil message returns false",
			msg:  nil,
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Act + Assert
			assert.Equal(t, tt.want, svc.IsSystemMessage(tt.msg))
		})
	}
}

func TestGetReplyMarkupData(t *testing.T) {
	t.Parallel()
	svc := New()

	tests := []struct {
		name string
		msg  *domain.Message
		want []byte
	}{
		{
			name: "message with callback data",
			msg: &domain.Message{
				ReplyMarkup: &domain.ReplyMarkup{CallbackData: []byte("data")},
			},
			want: []byte("data"),
		},
		{
			name: "message without reply markup",
			msg:  &domain.Message{},
			want: nil,
		},
		{
			name: "nil message",
			msg:  nil,
			want: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Act + Assert
			assert.Equal(t, tt.want, svc.GetReplyMarkupData(tt.msg))
		})
	}
}

func TestBuildInputContent_Photo(t *testing.T) {
	t.Parallel()

	// Arrange
	svc := New()
	msg := &domain.Message{
		Content: domain.MessageContent{
			Type:            domain.ContentPhoto,
			FileID:          "file123",
			ThumbnailFileID: "thumb456",
			Width:           800,
			Height:          600,
		},
	}
	text := &domain.FormattedText{Text: "caption"}

	// Act
	got := svc.BuildInputContent(msg, text)

	// Assert
	assert.Equal(t, domain.ContentPhoto, got.Type)
	assert.Equal(t, text, got.Text)
	assert.Equal(t, "file123", got.FileID)
	assert.Equal(t, "thumb456", got.ThumbnailFileID)
	assert.Equal(t, int32(800), got.Width)
	assert.Equal(t, int32(600), got.Height)
	assert.Empty(t, got.FileName)
	assert.Empty(t, got.MimeType)
}

func TestBuildInputContent_Text_InvertsLinkPreview(t *testing.T) {
	t.Parallel()

	// Arrange
	svc := New()
	msg := &domain.Message{
		Content: domain.MessageContent{
			Type:               domain.ContentText,
			DisableLinkPreview: false,
		},
	}
	text := &domain.FormattedText{Text: "hello"}

	// Act
	got := svc.BuildInputContent(msg, text)

	// Assert
	assert.True(t, got.DisableLinkPreview)
}

func TestBuildInputContent_Document(t *testing.T) {
	t.Parallel()

	// Arrange
	svc := New()
	msg := &domain.Message{
		Content: domain.MessageContent{
			Type:     domain.ContentDocument,
			FileID:   "doc123",
			FileName: "report.pdf",
			MimeType: "application/pdf",
		},
	}
	text := &domain.FormattedText{Text: "doc"}

	// Act
	got := svc.BuildInputContent(msg, text)

	// Assert
	assert.Equal(t, "doc123", got.FileID)
	assert.Equal(t, "report.pdf", got.FileName)
	assert.Equal(t, "application/pdf", got.MimeType)
	assert.Zero(t, got.Width)
}

func TestBuildInputContent_VoiceNote(t *testing.T) {
	t.Parallel()

	// Arrange
	svc := New()
	msg := &domain.Message{
		Content: domain.MessageContent{
			Type:     domain.ContentVoiceNote,
			Duration: 30,
			FileID:   "voice123",
		},
	}
	text := &domain.FormattedText{Text: "voice"}

	// Act
	got := svc.BuildInputContent(msg, text)

	// Assert
	assert.Equal(t, int32(30), got.Duration)
	assert.Empty(t, got.FileID)
}
