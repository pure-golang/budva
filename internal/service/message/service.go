package message

import (
	"log/slog"

	"github.com/zelenin/go-tdlib/client"
)

// Service извлекает и формирует контент сообщений поверх raw TDLib-типов.
type Service struct {
	logger *slog.Logger
}

// New создаёт новый экземпляр сервиса сообщений.
func New() *Service {
	return &Service{
		logger: slog.Default().With("module", "service.message"),
	}
}

// GetFormattedText извлекает FormattedText (text или caption) из контента.
// Возвращает nil для контент-типов без текста (системные, sticker, location и т.д.).
func (s *Service) GetFormattedText(msg *client.Message) *client.FormattedText {
	if msg == nil || msg.Content == nil {
		return nil
	}
	switch c := msg.Content.(type) {
	case *client.MessageText:
		return c.Text
	case *client.MessagePhoto:
		return c.Caption
	case *client.MessageVideo:
		return c.Caption
	case *client.MessageDocument:
		return c.Caption
	case *client.MessageAudio:
		return c.Caption
	case *client.MessageAnimation:
		return c.Caption
	case *client.MessageVoiceNote:
		return c.Caption
	default:
		return nil
	}
}

// IsSystemMessage возвращает true для контент-типов, которые не имеют
// текста/подписи (системные события чата, sticker, location и пр.).
// Такие сообщения не трансформируются и пересылаются только через forward.
func (s *Service) IsSystemMessage(msg *client.Message) bool {
	if msg == nil || msg.Content == nil {
		return false
	}
	switch msg.Content.(type) {
	case *client.MessageText,
		*client.MessagePhoto,
		*client.MessageVideo,
		*client.MessageDocument,
		*client.MessageAudio,
		*client.MessageAnimation,
		*client.MessageVoiceNote:
		return false
	}
	return true
}

// GetReplyMarkupData извлекает callback-данные первой inline-кнопки.
// Используется для автоответа на callback-кнопки (transform AutoAnswer).
func (s *Service) GetReplyMarkupData(msg *client.Message) []byte {
	if msg == nil || msg.ReplyMarkup == nil {
		return nil
	}
	kb, ok := msg.ReplyMarkup.(*client.ReplyMarkupInlineKeyboard)
	if !ok {
		return nil
	}
	for _, row := range kb.Rows {
		for _, btn := range row {
			if cb, ok := btn.Type.(*client.InlineKeyboardButtonTypeCallback); ok {
				return cb.Data
			}
		}
	}
	return nil
}

// BuildInputContent формирует InputMessageContent из оригинала с подменой текста/подписи.
// Для текста инвертирует LinkPreviewOptions.IsDisabled (если был preview — отключаем в копии).
// Для медиа копирует ID файла и метаданные.
func (s *Service) BuildInputContent(msg *client.Message, text *client.FormattedText) client.InputMessageContent {
	if msg == nil || msg.Content == nil {
		return &client.InputMessageText{Text: text}
	}
	switch c := msg.Content.(type) {
	case *client.MessageText:
		// Инвертируем IsDisabled: если в оригинале preview был включён
		// (явно или по умолчанию при nil LinkPreviewOptions), в копии — выключаем.
		// Это зеркалит legacy-поведение budva43.
		disabled := false
		if lp := c.LinkPreviewOptions; lp != nil {
			disabled = lp.IsDisabled
		}
		return &client.InputMessageText{
			Text:               text,
			LinkPreviewOptions: &client.LinkPreviewOptions{IsDisabled: !disabled},
		}
	case *client.MessagePhoto:
		return &client.InputMessagePhoto{
			Photo:     fileIDInput(c.Photo),
			Thumbnail: photoThumbnail(c.Photo),
			Width:     photoWidth(c.Photo),
			Height:    photoHeight(c.Photo),
			Caption:   text,
		}
	case *client.MessageVideo:
		var file client.InputFile
		var thumb *client.InputThumbnail
		var width, height, duration int32
		if c.Video != nil {
			if c.Video.Video != nil {
				file = &client.InputFileId{Id: c.Video.Video.Id}
			}
			if c.Video.Thumbnail != nil && c.Video.Thumbnail.File != nil {
				thumb = &client.InputThumbnail{Thumbnail: &client.InputFileId{Id: c.Video.Thumbnail.File.Id}}
			}
			width = c.Video.Width
			height = c.Video.Height
			duration = c.Video.Duration
		}
		return &client.InputMessageVideo{
			Video:     file,
			Thumbnail: thumb,
			Width:     width,
			Height:    height,
			Duration:  duration,
			Caption:   text,
		}
	case *client.MessageDocument:
		var file client.InputFile
		var thumb *client.InputThumbnail
		if c.Document != nil {
			if c.Document.Document != nil {
				file = &client.InputFileId{Id: c.Document.Document.Id}
			}
			if c.Document.Thumbnail != nil && c.Document.Thumbnail.File != nil {
				thumb = &client.InputThumbnail{Thumbnail: &client.InputFileId{Id: c.Document.Thumbnail.File.Id}}
			}
		}
		return &client.InputMessageDocument{
			Document:  file,
			Thumbnail: thumb,
			Caption:   text,
		}
	case *client.MessageAudio:
		var file client.InputFile
		var thumb *client.InputThumbnail
		var duration int32
		var title, performer string
		if c.Audio != nil {
			if c.Audio.Audio != nil {
				file = &client.InputFileId{Id: c.Audio.Audio.Id}
			}
			if c.Audio.AlbumCoverThumbnail != nil && c.Audio.AlbumCoverThumbnail.File != nil {
				thumb = &client.InputThumbnail{Thumbnail: &client.InputFileId{Id: c.Audio.AlbumCoverThumbnail.File.Id}}
			}
			duration = c.Audio.Duration
			title = c.Audio.Title
			performer = c.Audio.Performer
		}
		return &client.InputMessageAudio{
			Audio:               file,
			AlbumCoverThumbnail: thumb,
			Duration:            duration,
			Title:               title,
			Performer:           performer,
			Caption:             text,
		}
	case *client.MessageAnimation:
		var file client.InputFile
		var thumb *client.InputThumbnail
		var width, height, duration int32
		if c.Animation != nil {
			if c.Animation.Animation != nil {
				file = &client.InputFileId{Id: c.Animation.Animation.Id}
			}
			if c.Animation.Thumbnail != nil && c.Animation.Thumbnail.File != nil {
				thumb = &client.InputThumbnail{Thumbnail: &client.InputFileId{Id: c.Animation.Thumbnail.File.Id}}
			}
			width = c.Animation.Width
			height = c.Animation.Height
			duration = c.Animation.Duration
		}
		return &client.InputMessageAnimation{
			Animation: file,
			Thumbnail: thumb,
			Width:     width,
			Height:    height,
			Duration:  duration,
			Caption:   text,
		}
	case *client.MessageVoiceNote:
		var file client.InputFile
		var duration int32
		var waveform []byte
		if c.VoiceNote != nil {
			if c.VoiceNote.Voice != nil {
				file = &client.InputFileId{Id: c.VoiceNote.Voice.Id}
			}
			duration = c.VoiceNote.Duration
			waveform = c.VoiceNote.Waveform
		}
		return &client.InputMessageVoiceNote{
			VoiceNote: file,
			Duration:  duration,
			Waveform:  waveform,
			Caption:   text,
		}
	default:
		// Неподдерживаемые типы возвращаются как plain text.
		return &client.InputMessageText{Text: text}
	}
}

// fileIDInput выбирает фото-файл наибольшего размера (последний в массиве Sizes)
// и оборачивает его в InputFileId для повторной отправки.
func fileIDInput(photo *client.Photo) client.InputFile {
	if photo == nil || len(photo.Sizes) == 0 {
		return nil
	}
	last := photo.Sizes[len(photo.Sizes)-1]
	if last == nil || last.Photo == nil {
		return nil
	}
	return &client.InputFileId{Id: last.Photo.Id}
}

func photoThumbnail(photo *client.Photo) *client.InputThumbnail {
	if photo == nil || len(photo.Sizes) == 0 {
		return nil
	}
	first := photo.Sizes[0]
	if first == nil || first.Photo == nil {
		return nil
	}
	return &client.InputThumbnail{Thumbnail: &client.InputFileId{Id: first.Photo.Id}}
}

func photoWidth(photo *client.Photo) int32 {
	if photo == nil || len(photo.Sizes) == 0 {
		return 0
	}
	last := photo.Sizes[len(photo.Sizes)-1]
	if last == nil {
		return 0
	}
	return last.Width
}

func photoHeight(photo *client.Photo) int32 {
	if photo == nil || len(photo.Sizes) == 0 {
		return 0
	}
	last := photo.Sizes[len(photo.Sizes)-1]
	if last == nil {
		return 0
	}
	return last.Height
}
