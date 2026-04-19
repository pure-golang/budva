package message

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/zelenin/go-tdlib/client"
)

func TestGetFormattedText(t *testing.T) {
	t.Parallel()
	svc := New()

	tests := []struct {
		name string
		msg  *client.Message
		want *client.FormattedText
	}{
		{
			name: "text message returns formatted text",
			msg: &client.Message{
				Content: &client.MessageText{
					Text: &client.FormattedText{Text: "hello"},
				},
			},
			want: &client.FormattedText{Text: "hello"},
		},
		{
			name: "photo message returns caption",
			msg: &client.Message{
				Content: &client.MessagePhoto{
					Caption: &client.FormattedText{Text: "caption"},
				},
			},
			want: &client.FormattedText{Text: "caption"},
		},
		{
			name: "system message (chat join) returns nil",
			msg: &client.Message{
				Content: &client.MessageChatJoinByLink{},
			},
			want: nil,
		},
		{
			name: "sticker returns nil",
			msg: &client.Message{
				Content: &client.MessageSticker{},
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
			assert.Equal(t, tt.want, svc.GetFormattedText(tt.msg))
		})
	}
}

func TestIsSystemMessage(t *testing.T) {
	t.Parallel()
	svc := New()

	tests := []struct {
		name string
		msg  *client.Message
		want bool
	}{
		{
			name: "chat join is system",
			msg:  &client.Message{Content: &client.MessageChatJoinByLink{}},
			want: true,
		},
		{
			name: "sticker is system",
			msg:  &client.Message{Content: &client.MessageSticker{}},
			want: true,
		},
		{
			name: "text is not system",
			msg:  &client.Message{Content: &client.MessageText{}},
			want: false,
		},
		{
			name: "photo is not system",
			msg:  &client.Message{Content: &client.MessagePhoto{}},
			want: false,
		},
		{
			name: "nil message is not system",
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

	withCallback := &client.Message{
		ReplyMarkup: &client.ReplyMarkupInlineKeyboard{
			Rows: [][]*client.InlineKeyboardButton{{
				{
					Text: "click",
					Type: &client.InlineKeyboardButtonTypeCallback{Data: []byte("data")},
				},
			}},
		},
	}
	withURLOnly := &client.Message{
		ReplyMarkup: &client.ReplyMarkupInlineKeyboard{
			Rows: [][]*client.InlineKeyboardButton{{
				{
					Text: "open",
					Type: &client.InlineKeyboardButtonTypeUrl{Url: "https://example.com"},
				},
			}},
		},
	}

	tests := []struct {
		name string
		msg  *client.Message
		want []byte
	}{
		{name: "callback button returns data", msg: withCallback, want: []byte("data")},
		{name: "url-only keyboard returns nil", msg: withURLOnly, want: nil},
		{name: "no reply markup returns nil", msg: &client.Message{}, want: nil},
		{name: "nil message returns nil", msg: nil, want: nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, svc.GetReplyMarkupData(tt.msg))
		})
	}
}

func TestBuildInputContent_Text_InvertsLinkPreview(t *testing.T) {
	t.Parallel()

	svc := New()
	msg := &client.Message{
		Content: &client.MessageText{
			Text:               &client.FormattedText{Text: "hello"},
			LinkPreviewOptions: &client.LinkPreviewOptions{IsDisabled: false},
		},
	}
	text := &client.FormattedText{Text: "hello"}

	got := svc.BuildInputContent(msg, text)

	in, ok := got.(*client.InputMessageText)
	assert.True(t, ok)
	assert.Equal(t, text, in.Text)
	assert.True(t, in.LinkPreviewOptions.IsDisabled)
}

func TestBuildInputContent_Photo(t *testing.T) {
	t.Parallel()

	svc := New()
	msg := &client.Message{
		Content: &client.MessagePhoto{
			Photo: &client.Photo{
				Sizes: []*client.PhotoSize{
					{Photo: &client.File{Id: 10}, Width: 100, Height: 60},
					{Photo: &client.File{Id: 20}, Width: 800, Height: 600},
				},
			},
		},
	}
	text := &client.FormattedText{Text: "caption"}

	got := svc.BuildInputContent(msg, text)

	in, ok := got.(*client.InputMessagePhoto)
	assert.True(t, ok)
	assert.Equal(t, text, in.Caption)
	assert.Equal(t, int32(800), in.Width)
	assert.Equal(t, int32(600), in.Height)
	photoFile, ok := in.Photo.(*client.InputFileId)
	assert.True(t, ok)
	assert.Equal(t, int32(20), photoFile.Id)
}

func TestBuildInputContent_Video(t *testing.T) {
	t.Parallel()

	svc := New()
	msg := &client.Message{
		Content: &client.MessageVideo{
			Video: &client.Video{
				Video:    &client.File{Id: 50},
				Duration: 120,
				Width:    1920,
				Height:   1080,
			},
		},
	}
	text := &client.FormattedText{Text: "video caption"}

	got := svc.BuildInputContent(msg, text)

	in, ok := got.(*client.InputMessageVideo)
	assert.True(t, ok)
	assert.Equal(t, int32(1920), in.Width)
	assert.Equal(t, int32(1080), in.Height)
	assert.Equal(t, int32(120), in.Duration)
	videoFile, ok := in.Video.(*client.InputFileId)
	assert.True(t, ok)
	assert.Equal(t, int32(50), videoFile.Id)
	assert.Equal(t, text, in.Caption)
}

func TestBuildInputContent_Document(t *testing.T) {
	t.Parallel()

	svc := New()
	msg := &client.Message{
		Content: &client.MessageDocument{
			Document: &client.Document{
				Document: &client.File{Id: 77},
			},
		},
	}
	text := &client.FormattedText{Text: "doc"}

	got := svc.BuildInputContent(msg, text)

	in, ok := got.(*client.InputMessageDocument)
	assert.True(t, ok)
	docFile, ok := in.Document.(*client.InputFileId)
	assert.True(t, ok)
	assert.Equal(t, int32(77), docFile.Id)
	assert.Equal(t, text, in.Caption)
}

func TestBuildInputContent_Audio(t *testing.T) {
	t.Parallel()

	svc := New()
	msg := &client.Message{
		Content: &client.MessageAudio{
			Audio: &client.Audio{
				Audio:     &client.File{Id: 33},
				Duration:  240,
				Title:     "track",
				Performer: "artist",
			},
		},
	}
	text := &client.FormattedText{Text: "audio"}

	got := svc.BuildInputContent(msg, text)

	in, ok := got.(*client.InputMessageAudio)
	assert.True(t, ok)
	audioFile, ok := in.Audio.(*client.InputFileId)
	assert.True(t, ok)
	assert.Equal(t, int32(33), audioFile.Id)
	assert.Equal(t, int32(240), in.Duration)
	assert.Equal(t, "track", in.Title)
	assert.Equal(t, "artist", in.Performer)
}

func TestBuildInputContent_Animation(t *testing.T) {
	t.Parallel()

	svc := New()
	msg := &client.Message{
		Content: &client.MessageAnimation{
			Animation: &client.Animation{
				Animation: &client.File{Id: 42},
				Duration:  5,
				Width:     320,
				Height:    240,
			},
		},
	}
	text := &client.FormattedText{Text: "gif"}

	got := svc.BuildInputContent(msg, text)

	in, ok := got.(*client.InputMessageAnimation)
	assert.True(t, ok)
	animFile, ok := in.Animation.(*client.InputFileId)
	assert.True(t, ok)
	assert.Equal(t, int32(42), animFile.Id)
	assert.Equal(t, int32(320), in.Width)
	assert.Equal(t, int32(240), in.Height)
	assert.Equal(t, int32(5), in.Duration)
}

func TestBuildInputContent_VoiceNote(t *testing.T) {
	t.Parallel()

	svc := New()
	msg := &client.Message{
		Content: &client.MessageVoiceNote{
			VoiceNote: &client.VoiceNote{
				Voice:    &client.File{Id: 99},
				Duration: 30,
				Waveform: []byte{1, 2, 3},
			},
		},
	}
	text := &client.FormattedText{Text: "voice"}

	got := svc.BuildInputContent(msg, text)

	in, ok := got.(*client.InputMessageVoiceNote)
	assert.True(t, ok)
	voiceFile, ok := in.VoiceNote.(*client.InputFileId)
	assert.True(t, ok)
	assert.Equal(t, int32(99), voiceFile.Id)
	assert.Equal(t, int32(30), in.Duration)
	assert.Equal(t, []byte{1, 2, 3}, in.Waveform)
}
