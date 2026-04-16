package telegram

import (
	"github.com/zelenin/go-tdlib/client"

	"github.com/pure-golang/budva-claude/internal/domain"
)

// mapMessage конвертирует client.Message → domain.Message.
func mapMessage(msg *client.Message) *domain.Message {
	if msg == nil {
		return nil
	}
	dm := &domain.Message{
		ChatID:       msg.ChatId,
		ID:           msg.Id,
		MediaAlbumID: int64(msg.MediaAlbumId),
		CanBeSaved:   msg.CanBeSaved,
	}

	dm.Content = mapMessageContent(msg.Content)
	dm.ForwardInfo = mapForwardInfo(msg.ForwardInfo)
	dm.ReplyTo = mapReplyTo(msg.ReplyTo)
	dm.ReplyMarkup = mapReplyMarkup(msg.ReplyMarkup)

	return dm
}

// mapMessageContent конвертирует client.MessageContent → domain.MessageContent.
func mapMessageContent(content client.MessageContent) domain.MessageContent {
	if content == nil {
		return domain.MessageContent{Type: domain.ContentUnknown}
	}

	switch c := content.(type) {
	case *client.MessageText:
		mc := domain.MessageContent{
			Type: domain.ContentText,
			Text: mapFormattedText(c.Text),
		}
		if c.LinkPreviewOptions != nil {
			mc.DisableLinkPreview = c.LinkPreviewOptions.IsDisabled
		}
		return mc

	case *client.MessagePhoto:
		mc := domain.MessageContent{
			Type: domain.ContentPhoto,
			Text: mapFormattedText(c.Caption),
		}
		if c.Photo != nil && len(c.Photo.Sizes) > 0 {
			best := c.Photo.Sizes[len(c.Photo.Sizes)-1]
			mc.Width = best.Width
			mc.Height = best.Height
			if best.Photo != nil && best.Photo.Remote != nil {
				mc.FileID = best.Photo.Remote.Id
			}
		}
		return mc

	case *client.MessageVideo:
		mc := domain.MessageContent{
			Type:     domain.ContentVideo,
			Text:     mapFormattedText(c.Caption),
			Duration: c.Video.Duration,
		}
		if c.Video != nil {
			mc.Width = c.Video.Width
			mc.Height = c.Video.Height
			mc.FileName = c.Video.FileName
			mc.MimeType = c.Video.MimeType
			if c.Video.Video != nil && c.Video.Video.Remote != nil {
				mc.FileID = c.Video.Video.Remote.Id
			}
			if c.Video.Thumbnail != nil && c.Video.Thumbnail.File != nil && c.Video.Thumbnail.File.Remote != nil {
				mc.ThumbnailFileID = c.Video.Thumbnail.File.Remote.Id
			}
		}
		return mc

	case *client.MessageDocument:
		mc := domain.MessageContent{
			Type: domain.ContentDocument,
			Text: mapFormattedText(c.Caption),
		}
		if c.Document != nil {
			mc.FileName = c.Document.FileName
			mc.MimeType = c.Document.MimeType
			if c.Document.Document != nil && c.Document.Document.Remote != nil {
				mc.FileID = c.Document.Document.Remote.Id
			}
			if c.Document.Thumbnail != nil && c.Document.Thumbnail.File != nil && c.Document.Thumbnail.File.Remote != nil {
				mc.ThumbnailFileID = c.Document.Thumbnail.File.Remote.Id
			}
		}
		return mc

	case *client.MessageAudio:
		mc := domain.MessageContent{
			Type: domain.ContentAudio,
			Text: mapFormattedText(c.Caption),
		}
		if c.Audio != nil {
			mc.Duration = c.Audio.Duration
			mc.FileName = c.Audio.FileName
			mc.MimeType = c.Audio.MimeType
			if c.Audio.Audio != nil && c.Audio.Audio.Remote != nil {
				mc.FileID = c.Audio.Audio.Remote.Id
			}
		}
		return mc

	case *client.MessageAnimation:
		mc := domain.MessageContent{
			Type: domain.ContentAnimation,
			Text: mapFormattedText(c.Caption),
		}
		if c.Animation != nil {
			mc.Duration = c.Animation.Duration
			mc.Width = c.Animation.Width
			mc.Height = c.Animation.Height
			mc.FileName = c.Animation.FileName
			mc.MimeType = c.Animation.MimeType
			if c.Animation.Animation != nil && c.Animation.Animation.Remote != nil {
				mc.FileID = c.Animation.Animation.Remote.Id
			}
			if c.Animation.Thumbnail != nil && c.Animation.Thumbnail.File != nil && c.Animation.Thumbnail.File.Remote != nil {
				mc.ThumbnailFileID = c.Animation.Thumbnail.File.Remote.Id
			}
		}
		return mc

	case *client.MessageVoiceNote:
		mc := domain.MessageContent{
			Type: domain.ContentVoiceNote,
			Text: mapFormattedText(c.Caption),
		}
		if c.VoiceNote != nil {
			mc.Duration = c.VoiceNote.Duration
			mc.MimeType = c.VoiceNote.MimeType
			if c.VoiceNote.Voice != nil && c.VoiceNote.Voice.Remote != nil {
				mc.FileID = c.VoiceNote.Voice.Remote.Id
			}
		}
		return mc

	default:
		return domain.MessageContent{Type: domain.ContentUnknown}
	}
}

// mapForwardInfo конвертирует client.MessageForwardInfo → domain.MessageForwardInfo.
func mapForwardInfo(info *client.MessageForwardInfo) *domain.MessageForwardInfo {
	if info == nil || info.Origin == nil {
		return nil
	}
	ch, ok := info.Origin.(*client.MessageOriginChannel)
	if !ok {
		return nil
	}
	return &domain.MessageForwardInfo{
		OriginChatID:    ch.ChatId,
		OriginMessageID: ch.MessageId,
	}
}

// mapReplyTo конвертирует client.MessageReplyTo → domain.MessageReplyTo.
func mapReplyTo(rt client.MessageReplyTo) *domain.MessageReplyTo {
	if rt == nil {
		return nil
	}
	rm, ok := rt.(*client.MessageReplyToMessage)
	if !ok {
		return nil
	}
	return &domain.MessageReplyTo{
		ChatID:    rm.ChatId,
		MessageID: rm.MessageId,
	}
}

// mapReplyMarkup конвертирует client.ReplyMarkup → domain.ReplyMarkup (первая callback-кнопка).
func mapReplyMarkup(rm client.ReplyMarkup) *domain.ReplyMarkup {
	if rm == nil {
		return nil
	}
	inline, ok := rm.(*client.ReplyMarkupInlineKeyboard)
	if !ok {
		return nil
	}
	for _, row := range inline.Rows {
		for _, btn := range row {
			if cb, isCb := btn.Type.(*client.InlineKeyboardButtonTypeCallback); isCb {
				return &domain.ReplyMarkup{CallbackData: cb.Data}
			}
		}
	}
	return nil
}

// mapFormattedText конвертирует client.FormattedText → domain.FormattedText.
func mapFormattedText(ft *client.FormattedText) *domain.FormattedText {
	if ft == nil {
		return nil
	}
	dft := &domain.FormattedText{Text: ft.Text}
	if len(ft.Entities) > 0 {
		dft.Entities = make([]domain.TextEntity, len(ft.Entities))
		for i, e := range ft.Entities {
			dft.Entities[i] = domain.TextEntity{
				Offset: e.Offset,
				Length: e.Length,
				Type:   mapTextEntityType(e.Type),
				URL:    extractEntityURL(e.Type),
			}
		}
	}
	return dft
}

// toTDLibFormattedText конвертирует domain.FormattedText → client.FormattedText.
func toTDLibFormattedText(ft *domain.FormattedText) *client.FormattedText {
	if ft == nil {
		return nil
	}
	cft := &client.FormattedText{Text: ft.Text}
	if len(ft.Entities) > 0 {
		cft.Entities = make([]*client.TextEntity, len(ft.Entities))
		for i, e := range ft.Entities {
			cft.Entities[i] = &client.TextEntity{
				Offset: e.Offset,
				Length: e.Length,
				Type:   toTDLibTextEntityType(e),
			}
		}
	}
	return cft
}

func mapTextEntityType(t client.TextEntityType) domain.TextEntityType {
	if t == nil {
		return domain.TextEntityPlain
	}
	switch t.(type) {
	case *client.TextEntityTypeUrl:
		return domain.TextEntityURL
	case *client.TextEntityTypeTextUrl:
		return domain.TextEntityTextURL
	case *client.TextEntityTypeBold:
		return domain.TextEntityBold
	case *client.TextEntityTypeItalic:
		return domain.TextEntityItalic
	case *client.TextEntityTypeStrikethrough:
		return domain.TextEntityStrikethrough
	case *client.TextEntityTypeCode, *client.TextEntityTypePre:
		return domain.TextEntityCode
	default:
		return domain.TextEntityPlain
	}
}

func extractEntityURL(t client.TextEntityType) string {
	if tu, ok := t.(*client.TextEntityTypeTextUrl); ok {
		return tu.Url
	}
	return ""
}

func toTDLibTextEntityType(e domain.TextEntity) client.TextEntityType {
	switch e.Type {
	case domain.TextEntityURL:
		return &client.TextEntityTypeUrl{}
	case domain.TextEntityTextURL:
		return &client.TextEntityTypeTextUrl{Url: e.URL}
	case domain.TextEntityBold:
		return &client.TextEntityTypeBold{}
	case domain.TextEntityItalic:
		return &client.TextEntityTypeItalic{}
	case domain.TextEntityStrikethrough:
		return &client.TextEntityTypeStrikethrough{}
	case domain.TextEntityCode:
		return &client.TextEntityTypeCode{}
	default:
		return &client.TextEntityTypeBold{}
	}
}

// toTDLibInputMessageContent конвертирует domain.InputMessageContent → client.InputMessageContent.
func toTDLibInputMessageContent(content domain.InputMessageContent) client.InputMessageContent {
	switch content.Type {
	case domain.ContentText:
		imc := &client.InputMessageText{
			Text: toTDLibFormattedText(content.Text),
		}
		if content.DisableLinkPreview {
			imc.LinkPreviewOptions = &client.LinkPreviewOptions{IsDisabled: true}
		}
		return imc

	case domain.ContentPhoto:
		return &client.InputMessagePhoto{
			Photo:   toTDLibInputFile(content),
			Width:   content.Width,
			Height:  content.Height,
			Caption: toTDLibFormattedText(content.Text),
		}

	case domain.ContentVideo:
		return &client.InputMessageVideo{
			Video:    toTDLibInputFile(content),
			Duration: content.Duration,
			Width:    content.Width,
			Height:   content.Height,
			Caption:  toTDLibFormattedText(content.Text),
		}

	case domain.ContentDocument:
		return &client.InputMessageDocument{
			Document: toTDLibInputFile(content),
			Caption:  toTDLibFormattedText(content.Text),
		}

	case domain.ContentAudio:
		return &client.InputMessageAudio{
			Audio:    toTDLibInputFile(content),
			Duration: content.Duration,
			Caption:  toTDLibFormattedText(content.Text),
		}

	case domain.ContentAnimation:
		return &client.InputMessageAnimation{
			Animation: toTDLibInputFile(content),
			Duration:  content.Duration,
			Width:     content.Width,
			Height:    content.Height,
			Caption:   toTDLibFormattedText(content.Text),
		}

	case domain.ContentVoiceNote:
		return &client.InputMessageVoiceNote{
			VoiceNote: toTDLibInputFile(content),
			Duration:  content.Duration,
			Caption:   toTDLibFormattedText(content.Text),
		}

	default:
		return &client.InputMessageText{
			Text: toTDLibFormattedText(content.Text),
		}
	}
}

func toTDLibInputFile(content domain.InputMessageContent) client.InputFile {
	if content.FilePath != "" {
		return &client.InputFileLocal{Path: content.FilePath}
	}
	if content.FileID != "" {
		return &client.InputFileRemote{Id: content.FileID}
	}
	return &client.InputFileRemote{Id: ""}
}

// toTDLibInputMessageReplyTo конвертирует replyToMessageID → client.InputMessageReplyTo.
func toTDLibInputMessageReplyTo(replyToMessageID domain.MessageID) client.InputMessageReplyTo {
	if replyToMessageID == 0 {
		return nil
	}
	return &client.InputMessageReplyToMessage{
		MessageId: replyToMessageID,
	}
}
