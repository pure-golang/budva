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
			got := svc.GetFormattedText(tt.msg)
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
			assert.Equal(t, tt.want, svc.GetReplyMarkupData(tt.msg))
		})
	}
}

func TestBuildInputContent(t *testing.T) {
	t.Parallel()
	svc := New()

	msg := &domain.Message{
		Content: domain.MessageContent{
			Type:               domain.ContentPhoto,
			FileID:             "file123",
			ThumbnailFileID:    "thumb456",
			Width:              800,
			Height:             600,
			FileName:           "photo.jpg",
			MimeType:           "image/jpeg",
			DisableLinkPreview: true,
		},
	}
	text := &domain.FormattedText{Text: "new caption"}

	got := svc.BuildInputContent(msg, text)

	assert.Equal(t, domain.ContentPhoto, got.Type)
	assert.Equal(t, text, got.Text)
	assert.Equal(t, "file123", got.FileID)
	assert.Equal(t, "thumb456", got.ThumbnailFileID)
	assert.Equal(t, int32(800), got.Width)
	assert.Equal(t, int32(600), got.Height)
	assert.Equal(t, "photo.jpg", got.FileName)
	assert.Equal(t, "image/jpeg", got.MimeType)
	assert.True(t, got.DisableLinkPreview)
}
