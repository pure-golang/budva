package transform

import (
	"context"
	"log/slog"
	"slices"
	"strconv"
	"strings"

	"github.com/zelenin/go-tdlib/client"
	"go.opentelemetry.io/otel"

	"github.com/pure-golang/budva-claude/internal/domain"
)

var tracer = otel.Tracer("github.com/pure-golang/budva-claude/internal/service/transform")

// telegramRepo — частично применяемый интерфейс к infra/telegram.
// Содержит обёртки clientAdapter + то, что реально нужно transform-сервису.
type telegramRepo interface {
	TranslateText(*client.TranslateTextRequest) (*client.FormattedText, error)
	GetMessageLink(*client.GetMessageLinkRequest) (*client.MessageLink, error)
	GetMessageLinkInfo(*client.GetMessageLinkInfoRequest) (*client.MessageLinkInfo, error)
	GetCallbackQueryAnswer(*client.GetCallbackQueryAnswerRequest) (*client.CallbackQueryAnswer, error)
	GetChat(*client.GetChatRequest) (*client.Chat, error)
}

type stateRepo interface {
	GetNewMessageID(chatID int64, tmpMessageID int64) int64
	GetCopiedMessageIDs(chatID int64, messageID int64) []string
}

// Service применяет трансформации к тексту сообщения.
type Service struct {
	logger       *slog.Logger
	telegramRepo telegramRepo
	stateRepo    stateRepo
}

// New создаёт новый экземпляр сервиса трансформаций.
func New(telegramRepo telegramRepo, stateRepo stateRepo) *Service {
	return &Service{
		logger:       slog.Default().With("module", "service.transform"),
		telegramRepo: telegramRepo,
		stateRepo:    stateRepo,
	}
}

// Transform применяет все трансформации к тексту по конфигурации источника и получателя.
func (s *Service) Transform(ctx context.Context, p domain.TransformParams) (*client.FormattedText, error) {
	ctx, span := tracer.Start(ctx, "Transform")
	defer span.End()

	text := p.Text

	// 1. Перевод
	if p.Source.Translate != nil && containsChatID(p.Source.Translate.For, p.DstChatID) {
		translated, err := s.telegramRepo.TranslateText(&client.TranslateTextRequest{
			Text:           text,
			ToLanguageCode: p.Source.Translate.Lang,
		})
		if err != nil {
			s.logger.Error("Translation failed", slog.Any("err", err))
		} else {
			text = translated
		}
	}

	// 2. Auto-answer (callback query injection)
	if p.Source.AutoAnswer && len(p.ReplyMarkup) > 0 {
		text = s.addAutoAnswer(ctx, text, p.SrcChatID, p.SrcMessageID, p.ReplyMarkup)
	}

	// 3. Замена ссылок на свои сообщения
	if p.Destination != nil && p.Destination.ReplaceMyselfLinks != nil && p.Destination.ReplaceMyselfLinks.Run {
		text = s.replaceMyselfLinks(ctx, text, p.SrcChatID, p.DstChatID, p.Destination)
	}

	// 4. Замена фрагментов
	if p.Destination != nil {
		for _, fragment := range p.Destination.ReplaceFragments {
			text = replaceFragment(text, fragment)
		}
	}

	// 5. Подпись источника
	if p.WithSources && p.Source.Sign != nil && containsChatID(p.Source.Sign.For, p.DstChatID) {
		// TDLib Markdown v2: bold — одиночный `*`, двойной `**` не парсится.
		text = s.addText(ctx, text, "*"+p.Source.Sign.Title+"*")
	}

	// 6. Ссылка на источник
	if p.WithSources && p.Source.Link != nil && containsChatID(p.Source.Link.For, p.DstChatID) {
		link, err := s.telegramRepo.GetMessageLink(&client.GetMessageLinkRequest{
			ChatId:    p.SrcChatID,
			MessageId: p.SrcMessageID,
			ForAlbum:  p.ForAlbum,
		})
		if err == nil && link != nil && link.Link != "" {
			text = s.addText(ctx, text, "["+p.Source.Link.Title+"]("+link.Link+")")
		}
	}

	// 7. Ссылка на предыдущую версию
	if p.PrevMessageID != 0 && p.Source.Prev != nil && containsChatID(p.Source.Prev.For, p.DstChatID) {
		link, err := s.telegramRepo.GetMessageLink(&client.GetMessageLinkRequest{
			ChatId:    p.DstChatID,
			MessageId: p.PrevMessageID,
			ForAlbum:  p.ForAlbum,
		})
		if err == nil && link != nil && link.Link != "" {
			text = s.addText(ctx, text, "["+p.Source.Prev.Title+"]("+link.Link+")")
		}
	}

	return text, nil
}

// AddNextLink добавляет ссылку на следующую версию к существующему сообщению.
// Возвращает обновлённый текст для последующего EditMessage.
func (s *Service) AddNextLink(ctx context.Context, text *client.FormattedText, src *domain.Source, dstChatID int64, nextMessageID int64) *client.FormattedText {
	ctx, span := tracer.Start(ctx, "AddNextLink")
	defer span.End()

	if src.Next == nil || !containsChatID(src.Next.For, dstChatID) {
		return text
	}
	link, err := s.telegramRepo.GetMessageLink(&client.GetMessageLinkRequest{
		ChatId:    dstChatID,
		MessageId: nextMessageID,
		ForAlbum:  false,
	})
	if err != nil || link == nil || link.Link == "" {
		return text
	}
	return s.addText(ctx, text, "["+src.Next.Title+"]("+link.Link+")")
}

// addAutoAnswer добавляет текст ответа на callback-кнопку к сообщению.
func (s *Service) addAutoAnswer(ctx context.Context, text *client.FormattedText, srcChatID int64, srcMessageID int64, replyMarkup []byte) *client.FormattedText {
	answer, err := s.telegramRepo.GetCallbackQueryAnswer(&client.GetCallbackQueryAnswerRequest{
		ChatId:    srcChatID,
		MessageId: srcMessageID,
		Payload:   &client.CallbackQueryPayloadData{Data: replyMarkup},
	})
	if err != nil {
		s.logger.Error("Failed to get callback query answer", slog.Any("err", err))
		return text
	}
	if answer == nil || answer.Text == "" {
		return text
	}
	return s.addText(ctx, text, answer.Text)
}

// replaceMyselfLinks заменяет ссылки на сообщения в исходном чате ссылками на копии.
func (s *Service) replaceMyselfLinks(ctx context.Context, text *client.FormattedText, srcChatID, dstChatID int64, dst *domain.Destination) *client.FormattedText {
	if text == nil || len(text.Entities) == 0 {
		return text
	}

	// Проверяем тип чата — basic groups не поддерживают ссылки на сообщения.
	chat, err := s.telegramRepo.GetChat(&client.GetChatRequest{ChatId: srcChatID})
	if err != nil {
		s.logger.Error("Failed to get chat", slog.Any("err", err))
		return text
	}
	if _, isBasic := chat.Type.(*client.ChatTypeBasicGroup); isBasic {
		return text
	}

	result := deepCopyFormattedText(text)
	type replacement struct {
		offset      int32
		length      int32
		newText     string
		entityIndex int // -1 если не надо править конкретное entity
	}
	var replacements []replacement

	for i, ent := range result.Entities {
		if !isURLEntity(ent) {
			continue
		}

		url := entityURL(result.Text, ent)
		if url == "" {
			continue
		}

		linkInfo, err := s.telegramRepo.GetMessageLinkInfo(&client.GetMessageLinkInfoRequest{Url: url})
		if err != nil || linkInfo == nil {
			continue
		}

		// Проверяем, что ссылка ведёт на сообщение в исходном чате.
		if linkInfo.ChatId != srcChatID {
			if dst.ReplaceMyselfLinks.DeleteExternal {
				replacements = append(replacements, replacement{
					offset:      ent.Offset,
					length:      ent.Length,
					newText:     domain.DeletedLink,
					entityIndex: i,
				})
			}
			continue
		}

		// Находим копию сообщения в целевом чате.
		var srcMessageID int64
		if linkInfo.Message != nil {
			srcMessageID = linkInfo.Message.Id
		}
		copyLink := s.findCopyLink(ctx, linkInfo.ChatId, srcMessageID, dstChatID)
		if copyLink == "" {
			continue
		}
		if _, isTextURL := ent.Type.(*client.TextEntityTypeTextUrl); isTextURL {
			result.Entities[i].Type = &client.TextEntityTypeTextUrl{Url: copyLink}
		} else {
			replacements = append(replacements, replacement{
				offset:      ent.Offset,
				length:      ent.Length,
				newText:     copyLink,
				entityIndex: -1,
			})
		}
	}

	// Применяем замены справа налево, чтобы смещения ранее обработанных
	// entities оставались валидными.
	for i := len(replacements) - 1; i >= 0; i-- {
		r := replacements[i]
		result = applyReplacement(result, r.offset, r.length, r.newText)
		if r.entityIndex >= 0 {
			result.Entities[r.entityIndex].Type = &client.TextEntityTypeStrikethrough{}
		}
	}

	return result
}

// findCopyLink ищет ссылку на копию сообщения в целевом чате.
// Формат элемента copiedMessageIDs — "forwardRuleID:dstChatID:tmpMessageID";
// ruleID при чтении игнорируется, соответствие находим по равенству parts[1] == dstChatID.
func (s *Service) findCopyLink(ctx context.Context, srcChatID int64, srcMessageID int64, dstChatID int64) string {
	_ = ctx
	copies := s.stateRepo.GetCopiedMessageIDs(srcChatID, srcMessageID)
	for _, c := range copies {
		parts := strings.Split(c, ":")
		if len(parts) < 3 {
			continue
		}
		if parseMessageID(parts[1]) != dstChatID {
			continue
		}
		tmpID := parseMessageID(parts[2])
		newID := s.stateRepo.GetNewMessageID(dstChatID, tmpID)
		if newID == 0 {
			continue
		}
		link, err := s.telegramRepo.GetMessageLink(&client.GetMessageLinkRequest{
			ChatId:    dstChatID,
			MessageId: newID,
			ForAlbum:  false,
		})
		if err == nil && link != nil {
			return link.Link
		}
	}
	return ""
}

// addText добавляет форматированный текст (Markdown v2) в конец сообщения.
// ParseTextEntities — статическая функция go-tdlib, вызывается напрямую
// (не метод *client.Client, в clientAdapter не входит).
func (s *Service) addText(_ context.Context, text *client.FormattedText, markdown string) *client.FormattedText {
	parsed, err := client.ParseTextEntities(&client.ParseTextEntitiesRequest{
		Text:      markdown,
		ParseMode: &client.TextParseModeMarkdown{Version: 2},
	})
	if err != nil {
		// Fallback: добавляем как plain text.
		result := deepCopyFormattedText(text)
		result.Text += "\n\n" + markdown
		return result
	}

	result := deepCopyFormattedText(text)
	offset := lenUTF16(encodeUTF16(result.Text + "\n\n"))
	result.Text += "\n\n" + parsed.Text

	for _, ent := range parsed.Entities {
		ent.Offset += offset
		result.Entities = append(result.Entities, ent)
	}

	return result
}

func replaceFragment(text *client.FormattedText, fragment *domain.ReplaceFragment) *client.FormattedText {
	if text == nil || !strings.Contains(text.Text, fragment.From) {
		return text
	}
	result := deepCopyFormattedText(text)
	result.Text = strings.ReplaceAll(result.Text, fragment.From, fragment.To)
	return result
}

func containsChatID(ids []int64, target int64) bool {
	return slices.Contains(ids, target)
}

func extractSubstring(text string, offset, length int32) string {
	utf16 := encodeUTF16(text)
	if int(offset+length) > len(utf16) {
		return ""
	}
	return decodeUTF16(utf16[offset : offset+length])
}

func applyReplacement(text *client.FormattedText, offset, length int32, newText string) *client.FormattedText {
	utf16 := encodeUTF16(text.Text)
	newUTF16 := encodeUTF16(newText)
	diff := lenUTF16(newUTF16) - length

	// Заменяем в UTF-16 массиве.
	result := make([]uint16, 0, len(utf16)+int(diff))
	result = append(result, utf16[:offset]...)
	result = append(result, newUTF16...)
	result = append(result, utf16[offset+length:]...)

	text.Text = decodeUTF16(result)

	// Сдвигаем entities после замены.
	for i := range text.Entities {
		if text.Entities[i].Offset > offset {
			text.Entities[i].Offset += diff
		} else if text.Entities[i].Offset == offset {
			text.Entities[i].Length = lenUTF16(newUTF16)
		}
	}

	return text
}

func parseMessageID(s string) int64 {
	id, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0
	}
	return id
}
