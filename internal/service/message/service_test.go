package message_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zelenin/go-tdlib/client"

	"github.com/pure-golang/budva-claude/internal/service/message"
)

// New переименовываем в message.New в каждом тесте — пакет external.

func TestGetFormattedText(t *testing.T) {
	t.Parallel()
	svc := message.New()

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

			// Act
			got := svc.GetFormattedText(tt.msg)

			// Assert
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestIsSystemMessage(t *testing.T) {
	t.Parallel()
	svc := message.New()

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

			// Act
			got := svc.IsSystemMessage(tt.msg)

			// Assert
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestGetReplyMarkupData(t *testing.T) {
	t.Parallel()
	svc := message.New()

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

			// Act
			got := svc.GetReplyMarkupData(tt.msg)

			// Assert
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestBuildInputContent_Text_InvertsLinkPreview(t *testing.T) {
	t.Parallel()

	svc := message.New()
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

	svc := message.New()
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

	svc := message.New()
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

	svc := message.New()
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

	svc := message.New()
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

	svc := message.New()
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

	svc := message.New()
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

// TestGetFormattedText_AllCaptionTypes закрывает ветки caption для всех
// медиа-контент-типов (video, document, audio, animation, voice note),
// которые были непокрыты в исходных тестах.
func TestGetFormattedText_AllCaptionTypes(t *testing.T) {
	t.Parallel()

	svc := message.New()
	caption := &client.FormattedText{Text: "cap"}

	tests := []struct {
		name    string
		content client.MessageContent
	}{
		{
			name:    "video_caption",
			content: &client.MessageVideo{Caption: caption},
		},
		{
			name:    "document_caption",
			content: &client.MessageDocument{Caption: caption},
		},
		{
			name:    "audio_caption",
			content: &client.MessageAudio{Caption: caption},
		},
		{
			name:    "animation_caption",
			content: &client.MessageAnimation{Caption: caption},
		},
		{
			name:    "voice_note_caption",
			content: &client.MessageVoiceNote{Caption: caption},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			msg := &client.Message{Content: tt.content}

			// Act
			got := svc.GetFormattedText(msg)

			// Assert
			assert.Equal(t, caption, got)
		})
	}
}

// TestGetFormattedText_NilContent закрывает ветку nil msg.Content.
func TestGetFormattedText_NilContent(t *testing.T) {
	t.Parallel()

	// Arrange
	svc := message.New()
	msg := &client.Message{Content: nil}

	// Act
	got := svc.GetFormattedText(msg)

	// Assert
	assert.Nil(t, got)
}

// TestIsSystemMessage_NilContent закрывает ветку msg != nil, msg.Content == nil.
func TestIsSystemMessage_NilContent(t *testing.T) {
	t.Parallel()

	// Arrange
	svc := message.New()
	msg := &client.Message{Content: nil}

	// Act
	got := svc.IsSystemMessage(msg)

	// Assert
	assert.False(t, got)
}

// TestIsSystemMessage_MediaTypes проверяет все not-system ветки type switch,
// включая те, что не были покрыты (video, document, audio, animation, voice note).
func TestIsSystemMessage_MediaTypes(t *testing.T) {
	t.Parallel()

	svc := message.New()

	tests := []struct {
		name    string
		content client.MessageContent
	}{
		{name: "video_not_system", content: &client.MessageVideo{}},
		{name: "document_not_system", content: &client.MessageDocument{}},
		{name: "audio_not_system", content: &client.MessageAudio{}},
		{name: "animation_not_system", content: &client.MessageAnimation{}},
		{name: "voice_note_not_system", content: &client.MessageVoiceNote{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			msg := &client.Message{Content: tt.content}

			// Act
			got := svc.IsSystemMessage(msg)

			// Assert
			assert.False(t, got)
		})
	}
}

// TestGetReplyMarkupData_NonInlineKeyboard закрывает ветку cast к
// *client.ReplyMarkupInlineKeyboard с неуспешной проверкой.
func TestGetReplyMarkupData_NonInlineKeyboard(t *testing.T) {
	t.Parallel()

	svc := message.New()

	tests := []struct {
		name   string
		markup client.ReplyMarkup
	}{
		{
			name:   "force_reply_returns_nil",
			markup: &client.ReplyMarkupForceReply{IsPersonal: true},
		},
		{
			name:   "remove_keyboard_returns_nil",
			markup: &client.ReplyMarkupRemoveKeyboard{},
		},
		{
			name:   "show_keyboard_returns_nil",
			markup: &client.ReplyMarkupShowKeyboard{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			msg := &client.Message{ReplyMarkup: tt.markup}

			// Act
			got := svc.GetReplyMarkupData(msg)

			// Assert
			assert.Nil(t, got)
		})
	}
}

// TestGetReplyMarkupData_CallbackAfterURL проверяет обход строк/кнопок:
// callback найден во второй строке после url-кнопки — закрывает внутренние
// итерации for-циклов и возврат первой встреченной callback-data.
func TestGetReplyMarkupData_CallbackAfterURL(t *testing.T) {
	t.Parallel()

	// Arrange
	svc := message.New()
	msg := &client.Message{
		ReplyMarkup: &client.ReplyMarkupInlineKeyboard{
			Rows: [][]*client.InlineKeyboardButton{
				{
					{Text: "url", Type: &client.InlineKeyboardButtonTypeUrl{Url: "https://example.com"}},
				},
				{
					{Text: "url2", Type: &client.InlineKeyboardButtonTypeUrl{Url: "https://example.org"}},
					{Text: "cb", Type: &client.InlineKeyboardButtonTypeCallback{Data: []byte("pick")}},
				},
			},
		},
	}

	// Act
	got := svc.GetReplyMarkupData(msg)

	// Assert
	assert.Equal(t, []byte("pick"), got)
}

// TestBuildInputContent_NilMessage закрывает ветку msg == nil →
// возвращает InputMessageText.
func TestBuildInputContent_NilMessage(t *testing.T) {
	t.Parallel()

	// Arrange
	svc := message.New()
	text := &client.FormattedText{Text: "fallback"}

	// Act
	got := svc.BuildInputContent(nil, text)

	// Assert
	in, ok := got.(*client.InputMessageText)
	require.True(t, ok)
	assert.Equal(t, text, in.Text)
	assert.Nil(t, in.LinkPreviewOptions)
}

// TestBuildInputContent_NilContent закрывает ветку msg.Content == nil.
func TestBuildInputContent_NilContent(t *testing.T) {
	t.Parallel()

	// Arrange
	svc := message.New()
	text := &client.FormattedText{Text: "fallback"}
	msg := &client.Message{Content: nil}

	// Act
	got := svc.BuildInputContent(msg, text)

	// Assert
	in, ok := got.(*client.InputMessageText)
	require.True(t, ok)
	assert.Equal(t, text, in.Text)
}

// TestBuildInputContent_UnsupportedType закрывает default-ветку
// (sticker/location/system события) — fallback в plain text.
func TestBuildInputContent_UnsupportedType(t *testing.T) {
	t.Parallel()

	svc := message.New()
	text := &client.FormattedText{Text: "fallback"}

	tests := []struct {
		name    string
		content client.MessageContent
	}{
		{name: "sticker_falls_back_to_text", content: &client.MessageSticker{}},
		{name: "chat_join_falls_back_to_text", content: &client.MessageChatJoinByLink{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			msg := &client.Message{Content: tt.content}

			// Act
			got := svc.BuildInputContent(msg, text)

			// Assert
			in, ok := got.(*client.InputMessageText)
			require.True(t, ok)
			assert.Equal(t, text, in.Text)
		})
	}
}

// TestBuildInputContent_Text_NilLinkPreview закрывает ветку с nil
// LinkPreviewOptions: disabled=false по умолчанию, в копии IsDisabled=true.
func TestBuildInputContent_Text_NilLinkPreview(t *testing.T) {
	t.Parallel()

	// Arrange
	svc := message.New()
	msg := &client.Message{
		Content: &client.MessageText{
			Text:               &client.FormattedText{Text: "no preview opts"},
			LinkPreviewOptions: nil,
		},
	}
	text := &client.FormattedText{Text: "no preview opts"}

	// Act
	got := svc.BuildInputContent(msg, text)

	// Assert
	in, ok := got.(*client.InputMessageText)
	require.True(t, ok)
	require.NotNil(t, in.LinkPreviewOptions)
	assert.True(t, in.LinkPreviewOptions.IsDisabled)
}

// TestBuildInputContent_Text_DisabledPreview закрывает ветку с LinkPreviewOptions.IsDisabled=true.
func TestBuildInputContent_Text_DisabledPreview(t *testing.T) {
	t.Parallel()

	// Arrange
	svc := message.New()
	msg := &client.Message{
		Content: &client.MessageText{
			Text:               &client.FormattedText{Text: "preview off"},
			LinkPreviewOptions: &client.LinkPreviewOptions{IsDisabled: true},
		},
	}
	text := &client.FormattedText{Text: "preview off"}

	// Act
	got := svc.BuildInputContent(msg, text)

	// Assert
	in, ok := got.(*client.InputMessageText)
	require.True(t, ok)
	require.NotNil(t, in.LinkPreviewOptions)
	assert.False(t, in.LinkPreviewOptions.IsDisabled)
}

// TestBuildInputContent_Photo_NilPhoto закрывает путь через helper-функции
// с photo == nil → width/height=0, Photo=nil.
func TestBuildInputContent_Photo_NilPhoto(t *testing.T) {
	t.Parallel()

	// Arrange
	svc := message.New()
	msg := &client.Message{Content: &client.MessagePhoto{Photo: nil}}
	text := &client.FormattedText{Text: "no photo"}

	// Act
	got := svc.BuildInputContent(msg, text)

	// Assert
	in, ok := got.(*client.InputMessagePhoto)
	require.True(t, ok)
	assert.Nil(t, in.Photo)
	assert.Nil(t, in.Thumbnail)
	assert.Equal(t, int32(0), in.Width)
	assert.Equal(t, int32(0), in.Height)
	assert.Equal(t, text, in.Caption)
}

// TestBuildInputContent_Photo_EmptySizes закрывает ветку len(Sizes)==0
// во всех helper-функциях photo*.
func TestBuildInputContent_Photo_EmptySizes(t *testing.T) {
	t.Parallel()

	// Arrange
	svc := message.New()
	msg := &client.Message{Content: &client.MessagePhoto{
		Photo: &client.Photo{Sizes: []*client.PhotoSize{}},
	}}
	text := &client.FormattedText{Text: "empty sizes"}

	// Act
	got := svc.BuildInputContent(msg, text)

	// Assert
	in, ok := got.(*client.InputMessagePhoto)
	require.True(t, ok)
	assert.Nil(t, in.Photo)
	assert.Nil(t, in.Thumbnail)
	assert.Equal(t, int32(0), in.Width)
	assert.Equal(t, int32(0), in.Height)
}

// TestBuildInputContent_Photo_NilLastSize закрывает ветку last == nil внутри
// fileIDInput/photoWidth/photoHeight, когда последний элемент Sizes — nil.
func TestBuildInputContent_Photo_NilLastSize(t *testing.T) {
	t.Parallel()

	// Arrange
	svc := message.New()
	msg := &client.Message{Content: &client.MessagePhoto{
		Photo: &client.Photo{Sizes: []*client.PhotoSize{nil}},
	}}
	text := &client.FormattedText{Text: "nil size"}

	// Act
	got := svc.BuildInputContent(msg, text)

	// Assert
	in, ok := got.(*client.InputMessagePhoto)
	require.True(t, ok)
	assert.Nil(t, in.Photo)     // fileIDInput: last == nil
	assert.Nil(t, in.Thumbnail) // photoThumbnail: first == nil
	assert.Equal(t, int32(0), in.Width)
	assert.Equal(t, int32(0), in.Height)
}

// TestBuildInputContent_Photo_NilPhotoFile закрывает ветку last.Photo == nil
// внутри fileIDInput и first.Photo == nil внутри photoThumbnail.
func TestBuildInputContent_Photo_NilPhotoFile(t *testing.T) {
	t.Parallel()

	// Arrange
	svc := message.New()
	msg := &client.Message{Content: &client.MessagePhoto{
		Photo: &client.Photo{Sizes: []*client.PhotoSize{
			{Photo: nil, Width: 10, Height: 20},
		}},
	}}
	text := &client.FormattedText{Text: "nil inner file"}

	// Act
	got := svc.BuildInputContent(msg, text)

	// Assert
	in, ok := got.(*client.InputMessagePhoto)
	require.True(t, ok)
	assert.Nil(t, in.Photo)     // last.Photo == nil
	assert.Nil(t, in.Thumbnail) // first.Photo == nil
	assert.Equal(t, int32(10), in.Width)
	assert.Equal(t, int32(20), in.Height)
}

// TestBuildInputContent_Photo_WithThumbnail закрывает полный happy-path
// photoThumbnail, когда first и first.Photo оба не nil.
func TestBuildInputContent_Photo_WithThumbnail(t *testing.T) {
	t.Parallel()

	// Arrange
	svc := message.New()
	msg := &client.Message{Content: &client.MessagePhoto{
		Photo: &client.Photo{Sizes: []*client.PhotoSize{
			{Photo: &client.File{Id: 1}, Width: 50, Height: 50},
			{Photo: &client.File{Id: 2}, Width: 200, Height: 200},
		}},
	}}
	text := &client.FormattedText{Text: "with thumb"}

	// Act
	got := svc.BuildInputContent(msg, text)

	// Assert
	in, ok := got.(*client.InputMessagePhoto)
	require.True(t, ok)
	thumb, ok := in.Thumbnail.Thumbnail.(*client.InputFileId)
	require.True(t, ok)
	assert.Equal(t, int32(1), thumb.Id)
}

// TestBuildInputContent_Video_NilVideo закрывает ветку c.Video == nil.
func TestBuildInputContent_Video_NilVideo(t *testing.T) {
	t.Parallel()

	// Arrange
	svc := message.New()
	msg := &client.Message{Content: &client.MessageVideo{Video: nil}}
	text := &client.FormattedText{Text: "no video"}

	// Act
	got := svc.BuildInputContent(msg, text)

	// Assert
	in, ok := got.(*client.InputMessageVideo)
	require.True(t, ok)
	assert.Nil(t, in.Video)
	assert.Nil(t, in.Thumbnail)
	assert.Equal(t, int32(0), in.Width)
	assert.Equal(t, int32(0), in.Height)
	assert.Equal(t, int32(0), in.Duration)
}

// TestBuildInputContent_Video_WithThumbnail закрывает ветки Thumbnail != nil
// и Thumbnail.File != nil.
func TestBuildInputContent_Video_WithThumbnail(t *testing.T) {
	t.Parallel()

	// Arrange
	svc := message.New()
	msg := &client.Message{Content: &client.MessageVideo{
		Video: &client.Video{
			Video:     &client.File{Id: 1},
			Thumbnail: &client.Thumbnail{File: &client.File{Id: 5}},
			Width:     640,
			Height:    480,
			Duration:  10,
		},
	}}
	text := &client.FormattedText{Text: "vid thumb"}

	// Act
	got := svc.BuildInputContent(msg, text)

	// Assert
	in, ok := got.(*client.InputMessageVideo)
	require.True(t, ok)
	require.NotNil(t, in.Thumbnail)
	thumb, ok := in.Thumbnail.Thumbnail.(*client.InputFileId)
	require.True(t, ok)
	assert.Equal(t, int32(5), thumb.Id)
}

// TestBuildInputContent_Video_NilInnerFile закрывает ветку c.Video.Video == nil
// и Thumbnail без File (File == nil).
func TestBuildInputContent_Video_NilInnerFile(t *testing.T) {
	t.Parallel()

	// Arrange
	svc := message.New()
	msg := &client.Message{Content: &client.MessageVideo{
		Video: &client.Video{
			Video:     nil,
			Thumbnail: &client.Thumbnail{File: nil},
			Width:     100,
		},
	}}
	text := &client.FormattedText{Text: "nil inner"}

	// Act
	got := svc.BuildInputContent(msg, text)

	// Assert
	in, ok := got.(*client.InputMessageVideo)
	require.True(t, ok)
	assert.Nil(t, in.Video)
	assert.Nil(t, in.Thumbnail)
	assert.Equal(t, int32(100), in.Width)
}

// TestBuildInputContent_Document_NilDocument закрывает ветку c.Document == nil.
func TestBuildInputContent_Document_NilDocument(t *testing.T) {
	t.Parallel()

	// Arrange
	svc := message.New()
	msg := &client.Message{Content: &client.MessageDocument{Document: nil}}
	text := &client.FormattedText{Text: "no doc"}

	// Act
	got := svc.BuildInputContent(msg, text)

	// Assert
	in, ok := got.(*client.InputMessageDocument)
	require.True(t, ok)
	assert.Nil(t, in.Document)
	assert.Nil(t, in.Thumbnail)
}

// TestBuildInputContent_Document_WithThumbnail закрывает ветку Thumbnail != nil
// и Thumbnail.File != nil для документа.
func TestBuildInputContent_Document_WithThumbnail(t *testing.T) {
	t.Parallel()

	// Arrange
	svc := message.New()
	msg := &client.Message{Content: &client.MessageDocument{
		Document: &client.Document{
			Document:  &client.File{Id: 7},
			Thumbnail: &client.Thumbnail{File: &client.File{Id: 8}},
		},
	}}
	text := &client.FormattedText{Text: "doc thumb"}

	// Act
	got := svc.BuildInputContent(msg, text)

	// Assert
	in, ok := got.(*client.InputMessageDocument)
	require.True(t, ok)
	require.NotNil(t, in.Thumbnail)
	thumb, ok := in.Thumbnail.Thumbnail.(*client.InputFileId)
	require.True(t, ok)
	assert.Equal(t, int32(8), thumb.Id)
}

// TestBuildInputContent_Document_NilInnerFile закрывает ветки c.Document.Document == nil
// и Thumbnail.File == nil.
func TestBuildInputContent_Document_NilInnerFile(t *testing.T) {
	t.Parallel()

	// Arrange
	svc := message.New()
	msg := &client.Message{Content: &client.MessageDocument{
		Document: &client.Document{
			Document:  nil,
			Thumbnail: &client.Thumbnail{File: nil},
		},
	}}
	text := &client.FormattedText{Text: "doc nil"}

	// Act
	got := svc.BuildInputContent(msg, text)

	// Assert
	in, ok := got.(*client.InputMessageDocument)
	require.True(t, ok)
	assert.Nil(t, in.Document)
	assert.Nil(t, in.Thumbnail)
}

// TestBuildInputContent_Audio_NilAudio закрывает ветку c.Audio == nil.
func TestBuildInputContent_Audio_NilAudio(t *testing.T) {
	t.Parallel()

	// Arrange
	svc := message.New()
	msg := &client.Message{Content: &client.MessageAudio{Audio: nil}}
	text := &client.FormattedText{Text: "no audio"}

	// Act
	got := svc.BuildInputContent(msg, text)

	// Assert
	in, ok := got.(*client.InputMessageAudio)
	require.True(t, ok)
	assert.Nil(t, in.Audio)
	assert.Nil(t, in.AlbumCoverThumbnail)
	assert.Equal(t, int32(0), in.Duration)
	assert.Empty(t, in.Title)
	assert.Empty(t, in.Performer)
}

// TestBuildInputContent_Audio_WithCover закрывает ветку AlbumCoverThumbnail != nil
// и AlbumCoverThumbnail.File != nil.
func TestBuildInputContent_Audio_WithCover(t *testing.T) {
	t.Parallel()

	// Arrange
	svc := message.New()
	msg := &client.Message{Content: &client.MessageAudio{
		Audio: &client.Audio{
			Audio:               &client.File{Id: 1},
			AlbumCoverThumbnail: &client.Thumbnail{File: &client.File{Id: 9}},
		},
	}}
	text := &client.FormattedText{Text: "cover"}

	// Act
	got := svc.BuildInputContent(msg, text)

	// Assert
	in, ok := got.(*client.InputMessageAudio)
	require.True(t, ok)
	require.NotNil(t, in.AlbumCoverThumbnail)
	thumb, ok := in.AlbumCoverThumbnail.Thumbnail.(*client.InputFileId)
	require.True(t, ok)
	assert.Equal(t, int32(9), thumb.Id)
}

// TestBuildInputContent_Audio_NilInnerFile закрывает ветки c.Audio.Audio == nil
// и AlbumCoverThumbnail.File == nil.
func TestBuildInputContent_Audio_NilInnerFile(t *testing.T) {
	t.Parallel()

	// Arrange
	svc := message.New()
	msg := &client.Message{Content: &client.MessageAudio{
		Audio: &client.Audio{
			Audio:               nil,
			AlbumCoverThumbnail: &client.Thumbnail{File: nil},
			Duration:            12,
		},
	}}
	text := &client.FormattedText{Text: "audio nil"}

	// Act
	got := svc.BuildInputContent(msg, text)

	// Assert
	in, ok := got.(*client.InputMessageAudio)
	require.True(t, ok)
	assert.Nil(t, in.Audio)
	assert.Nil(t, in.AlbumCoverThumbnail)
	assert.Equal(t, int32(12), in.Duration)
}

// TestBuildInputContent_Animation_NilAnimation закрывает ветку c.Animation == nil.
func TestBuildInputContent_Animation_NilAnimation(t *testing.T) {
	t.Parallel()

	// Arrange
	svc := message.New()
	msg := &client.Message{Content: &client.MessageAnimation{Animation: nil}}
	text := &client.FormattedText{Text: "no anim"}

	// Act
	got := svc.BuildInputContent(msg, text)

	// Assert
	in, ok := got.(*client.InputMessageAnimation)
	require.True(t, ok)
	assert.Nil(t, in.Animation)
	assert.Nil(t, in.Thumbnail)
	assert.Equal(t, int32(0), in.Width)
	assert.Equal(t, int32(0), in.Height)
	assert.Equal(t, int32(0), in.Duration)
}

// TestBuildInputContent_Animation_WithThumbnail закрывает ветку Thumbnail != nil
// и Thumbnail.File != nil.
func TestBuildInputContent_Animation_WithThumbnail(t *testing.T) {
	t.Parallel()

	// Arrange
	svc := message.New()
	msg := &client.Message{Content: &client.MessageAnimation{
		Animation: &client.Animation{
			Animation: &client.File{Id: 1},
			Thumbnail: &client.Thumbnail{File: &client.File{Id: 11}},
		},
	}}
	text := &client.FormattedText{Text: "anim thumb"}

	// Act
	got := svc.BuildInputContent(msg, text)

	// Assert
	in, ok := got.(*client.InputMessageAnimation)
	require.True(t, ok)
	require.NotNil(t, in.Thumbnail)
	thumb, ok := in.Thumbnail.Thumbnail.(*client.InputFileId)
	require.True(t, ok)
	assert.Equal(t, int32(11), thumb.Id)
}

// TestBuildInputContent_Animation_NilInnerFile закрывает ветки c.Animation.Animation == nil
// и Thumbnail.File == nil.
func TestBuildInputContent_Animation_NilInnerFile(t *testing.T) {
	t.Parallel()

	// Arrange
	svc := message.New()
	msg := &client.Message{Content: &client.MessageAnimation{
		Animation: &client.Animation{
			Animation: nil,
			Thumbnail: &client.Thumbnail{File: nil},
			Duration:  3,
		},
	}}
	text := &client.FormattedText{Text: "anim nil"}

	// Act
	got := svc.BuildInputContent(msg, text)

	// Assert
	in, ok := got.(*client.InputMessageAnimation)
	require.True(t, ok)
	assert.Nil(t, in.Animation)
	assert.Nil(t, in.Thumbnail)
	assert.Equal(t, int32(3), in.Duration)
}

// TestBuildInputContent_VoiceNote_NilVoiceNote закрывает ветку c.VoiceNote == nil.
func TestBuildInputContent_VoiceNote_NilVoiceNote(t *testing.T) {
	t.Parallel()

	// Arrange
	svc := message.New()
	msg := &client.Message{Content: &client.MessageVoiceNote{VoiceNote: nil}}
	text := &client.FormattedText{Text: "no voice"}

	// Act
	got := svc.BuildInputContent(msg, text)

	// Assert
	in, ok := got.(*client.InputMessageVoiceNote)
	require.True(t, ok)
	assert.Nil(t, in.VoiceNote)
	assert.Equal(t, int32(0), in.Duration)
	assert.Empty(t, in.Waveform)
}

// TestBuildInputContent_VoiceNote_NilInnerFile закрывает ветку c.VoiceNote.Voice == nil,
// но duration и waveform всё равно копируются.
func TestBuildInputContent_VoiceNote_NilInnerFile(t *testing.T) {
	t.Parallel()

	// Arrange
	svc := message.New()
	msg := &client.Message{Content: &client.MessageVoiceNote{
		VoiceNote: &client.VoiceNote{
			Voice:    nil,
			Duration: 7,
			Waveform: []byte{9, 9},
		},
	}}
	text := &client.FormattedText{Text: "voice nil"}

	// Act
	got := svc.BuildInputContent(msg, text)

	// Assert
	in, ok := got.(*client.InputMessageVoiceNote)
	require.True(t, ok)
	assert.Nil(t, in.VoiceNote)
	assert.Equal(t, int32(7), in.Duration)
	assert.Equal(t, []byte{9, 9}, in.Waveform)
}
