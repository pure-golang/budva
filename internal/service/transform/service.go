package transform

import (
	"context"
	"fmt"
	"log/slog"
	"slices"
	"strconv"
	"strings"

	"github.com/pure-golang/budva-claude/internal/domain"
)

type telegramGateway interface {
	TranslateText(ctx context.Context, text *domain.FormattedText, lang string) (*domain.FormattedText, error)
	GetMessageLink(ctx context.Context, chatID domain.ChatID, messageID domain.MessageID) (string, error)
	GetMessageLinkInfo(ctx context.Context, url string) (*domain.MessageLinkInfo, error)
	GetCallbackQueryAnswer(ctx context.Context, chatID domain.ChatID, messageID domain.MessageID, data []byte) (string, error)
	GetChatType(ctx context.Context, chatID domain.ChatID) (string, error)
	ParseTextEntities(ctx context.Context, text string) (*domain.FormattedText, error)
}

type stateStore interface {
	GetNewMessageID(chatID domain.ChatID, tmpMessageID domain.MessageID) domain.MessageID
	GetCopiedMessageIDs(chatID domain.ChatID, messageID domain.MessageID) []string
}

// Service применяет трансформации к тексту сообщения.
type Service struct {
	logger   *slog.Logger
	telegram telegramGateway
	state    stateStore
}

// New создаёт новый экземпляр сервиса трансформаций.
func New(telegram telegramGateway, state stateStore) *Service {
	return &Service{
		logger:   slog.Default().With("module", "service.transform"),
		telegram: telegram,
		state:    state,
	}
}

// Transform применяет все трансформации к тексту по конфигурации источника и получателя.
func (s *Service) Transform(ctx context.Context, p domain.TransformParams) (*domain.FormattedText, error) {
	text := p.Text

	// 1. Перевод
	if p.Source.Translate != nil && containsChatID(p.Source.Translate.For, p.DstChatID) {
		translated, err := s.telegram.TranslateText(ctx, text, p.Source.Translate.Lang)
		if err != nil {
			s.logger.Error("Translation failed", slog.Any("error", err))
		} else {
			text = translated
		}
	}

	// 2. Замена ссылок на свои сообщения
	if p.Destination != nil && p.Destination.ReplaceMyselfLinks != nil && p.Destination.ReplaceMyselfLinks.Run {
		text = s.replaceMyselfLinks(ctx, text, p.SrcChatID, p.DstChatID, p.Destination)
	}

	// 3. Замена фрагментов
	if p.Destination != nil {
		for _, fragment := range p.Destination.ReplaceFragments {
			text = replaceFragment(text, fragment)
		}
	}

	// 4. Подпись источника
	if p.WithSources && p.Source.Sign != nil && containsChatID(p.Source.Sign.For, p.DstChatID) {
		text = s.addText(ctx, text, "**"+p.Source.Sign.Title+"**")
	}

	// 5. Ссылка на источник
	if p.WithSources && p.Source.Link != nil && containsChatID(p.Source.Link.For, p.DstChatID) {
		link, err := s.telegram.GetMessageLink(ctx, p.SrcChatID, p.SrcMessageID)
		if err == nil && link != "" {
			text = s.addText(ctx, text, "["+p.Source.Link.Title+"]("+link+")")
		}
	}

	// 6. Ссылка на предыдущую версию
	if p.PrevMessageID != 0 && p.Source.Prev != nil && containsChatID(p.Source.Prev.For, p.DstChatID) {
		link, err := s.telegram.GetMessageLink(ctx, p.DstChatID, p.PrevMessageID)
		if err == nil && link != "" {
			text = s.addText(ctx, text, "["+p.Source.Prev.Title+"]("+link+")")
		}
	}

	return text, nil
}

// AddNextLink добавляет ссылку на следующую версию к существующему сообщению.
// Возвращает обновлённый текст для последующего EditMessage.
func (s *Service) AddNextLink(ctx context.Context, text *domain.FormattedText, src *domain.Source, dstChatID domain.ChatID, nextMessageID domain.MessageID) *domain.FormattedText {
	if src.Next == nil || !containsChatID(src.Next.For, dstChatID) {
		return text
	}
	link, err := s.telegram.GetMessageLink(ctx, dstChatID, nextMessageID)
	if err != nil || link == "" {
		return text
	}
	return s.addText(ctx, text, "["+src.Next.Title+"]("+link+")")
}

// replaceMyselfLinks заменяет ссылки на сообщения в исходном чате ссылками на копии.
func (s *Service) replaceMyselfLinks(ctx context.Context, text *domain.FormattedText, srcChatID, dstChatID domain.ChatID, dst *domain.Destination) *domain.FormattedText {
	if text == nil || len(text.Entities) == 0 {
		return text
	}

	// Проверяем тип чата — basic groups не поддерживают ссылки на сообщения
	chatType, err := s.telegram.GetChatType(ctx, srcChatID)
	if err != nil {
		s.logger.Error("Failed to get chat type", slog.Any("error", err))
		return text
	}
	if chatType == "basicGroup" {
		return text
	}

	result := text.DeepCopy()
	type replacement struct {
		offset  int32
		length  int32
		newText string
		entity  *domain.TextEntity
	}
	var replacements []replacement

	for i, ent := range result.Entities {
		if ent.Type != domain.TextEntityURL && ent.Type != domain.TextEntityTextURL {
			continue
		}

		var url string
		if ent.Type == domain.TextEntityURL {
			url = extractSubstring(result.Text, ent.Offset, ent.Length)
		} else {
			url = ent.URL
		}

		linkInfo, err := s.telegram.GetMessageLinkInfo(ctx, url)
		if err != nil || linkInfo == nil {
			continue
		}

		// Проверяем, что ссылка ведёт на сообщение в исходном чате
		if linkInfo.ChatID != srcChatID {
			if dst.ReplaceMyselfLinks.DeleteExternal {
				replacements = append(replacements, replacement{
					offset:  ent.Offset,
					length:  ent.Length,
					newText: domain.DeletedLink,
					entity:  &result.Entities[i],
				})
			}
			continue
		}

		// Находим копию сообщения в целевом чате
		copyLink := s.findCopyLink(ctx, linkInfo.ChatID, linkInfo.MessageID, dstChatID)
		if copyLink != "" {
			if ent.Type == domain.TextEntityTextURL {
				result.Entities[i].URL = copyLink
			} else {
				replacements = append(replacements, replacement{
					offset:  ent.Offset,
					length:  ent.Length,
					newText: copyLink,
				})
			}
		}
	}

	// Применяем замены справа налево
	for i := len(replacements) - 1; i >= 0; i-- {
		r := replacements[i]
		result = applyReplacement(result, r.offset, r.length, r.newText)
		if r.entity != nil && r.entity.Type == domain.TextEntityTextURL {
			r.entity.Type = domain.TextEntityStrikethrough
			r.entity.URL = ""
		}
	}

	return result
}

// findCopyLink ищет ссылку на копию сообщения в целевом чате.
func (s *Service) findCopyLink(ctx context.Context, srcChatID domain.ChatID, srcMessageID domain.MessageID, dstChatID domain.ChatID) string {
	copies := s.state.GetCopiedMessageIDs(srcChatID, srcMessageID)
	dstPrefix := formatDstPrefix(dstChatID)
	for _, copy := range copies {
		if strings.HasPrefix(copy, dstPrefix) {
			parts := strings.Split(copy, ":")
			if len(parts) >= 3 {
				tmpID := parseMessageID(parts[len(parts)-1])
				newID := s.state.GetNewMessageID(dstChatID, tmpID)
				if newID != 0 {
					link, err := s.telegram.GetMessageLink(ctx, dstChatID, newID)
					if err == nil {
						return link
					}
				}
			}
		}
	}
	return ""
}

// addText добавляет форматированный текст (Markdown v2) в конец сообщения.
func (s *Service) addText(ctx context.Context, text *domain.FormattedText, markdown string) *domain.FormattedText {
	parsed, err := s.telegram.ParseTextEntities(ctx, markdown)
	if err != nil {
		// Fallback: добавляем как plain text
		result := text.DeepCopy()
		result.Text += "\n\n" + markdown
		return result
	}

	result := text.DeepCopy()
	offset := int32(len(encodeUTF16(result.Text + "\n\n"))) //nolint:gosec // UTF-16 offset всегда < 2^31
	result.Text += "\n\n" + parsed.Text

	for _, ent := range parsed.Entities {
		ent.Offset += offset
		result.Entities = append(result.Entities, ent)
	}

	return result
}

func replaceFragment(text *domain.FormattedText, fragment *domain.ReplaceFragment) *domain.FormattedText {
	if text == nil || !strings.Contains(text.Text, fragment.From) {
		return text
	}
	result := text.DeepCopy()
	result.Text = strings.ReplaceAll(result.Text, fragment.From, fragment.To)
	return result
}

func containsChatID(ids []domain.ChatID, target domain.ChatID) bool {
	return slices.Contains(ids, target)
}

func extractSubstring(text string, offset, length int32) string {
	utf16 := encodeUTF16(text)
	if int(offset+length) > len(utf16) {
		return ""
	}
	return decodeUTF16(utf16[offset : offset+length])
}

func applyReplacement(text *domain.FormattedText, offset, length int32, newText string) *domain.FormattedText {
	utf16 := encodeUTF16(text.Text)
	newUTF16 := encodeUTF16(newText)
	diff := int32(len(newUTF16)) - length //nolint:gosec // UTF-16 длина всегда < 2^31

	// Заменяем в UTF-16 массиве
	result := make([]uint16, 0, len(utf16)+int(diff))
	result = append(result, utf16[:offset]...)
	result = append(result, newUTF16...)
	result = append(result, utf16[offset+length:]...)

	text.Text = decodeUTF16(result)

	// Сдвигаем entities после замены
	for i := range text.Entities {
		if text.Entities[i].Offset > offset {
			text.Entities[i].Offset += diff
		} else if text.Entities[i].Offset == offset {
			text.Entities[i].Length = int32(len(newUTF16)) //nolint:gosec // UTF-16 длина всегда < 2^31
		}
	}

	return text
}

func formatDstPrefix(dstChatID domain.ChatID) string {
	// Формат copiedMessageID: "ruleID:dstChatID:tmpMsgID"
	s := fmt.Sprintf("%d", dstChatID)
	s = strings.ReplaceAll(s, "-", "")
	return ":" + s
}

func parseMessageID(s string) domain.MessageID {
	id, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0
	}
	return id
}

// encodeUTF16 конвертирует строку в UTF-16 массив.
//
//nolint:gosec // Конвертация rune → uint16 безопасна для BMP и surrogate pairs
func encodeUTF16(s string) []uint16 {
	var result []uint16
	for _, r := range s {
		if r >= 0x10000 {
			r -= 0x10000
			result = append(result, uint16(0xD800+(r>>10)), uint16(0xDC00+(r&0x3FF)))
		} else {
			result = append(result, uint16(r))
		}
	}
	return result
}

// decodeUTF16 конвертирует UTF-16 массив в строку.
func decodeUTF16(u []uint16) string {
	var runes []rune
	for i := 0; i < len(u); i++ {
		if u[i] >= 0xD800 && u[i] <= 0xDBFF && i+1 < len(u) && u[i+1] >= 0xDC00 && u[i+1] <= 0xDFFF {
			r := rune((int(u[i])-0xD800)<<10 + int(u[i+1]) - 0xDC00 + 0x10000)
			runes = append(runes, r)
			i++
		} else {
			runes = append(runes, rune(u[i]))
		}
	}
	return string(runes)
}
