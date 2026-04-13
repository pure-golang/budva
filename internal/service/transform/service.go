package transform

import (
	"context"
	"log/slog"
	"strings"

	"github.com/pure-golang/budva/internal/domain"
)

type translator interface {
	TranslateText(ctx context.Context, text string, lang string) (string, error)
}

// Service применяет трансформации к тексту сообщения.
type Service struct {
	logger     *slog.Logger
	translator translator
}

// New создаёт новый экземпляр сервиса трансформаций.
func New(translator translator, logger *slog.Logger) *Service {
	return &Service{
		logger:     logger.With("module", "service.transform"),
		translator: translator,
	}
}

// Apply применяет все трансформации к тексту в соответствии с настройками источника и получателя.
func (s *Service) Apply(ctx context.Context, text string, src *domain.Source, dst *domain.Destination, dstChatID domain.ChatID) (string, error) {
	var err error

	// Перевод
	if src.Translate != nil && containsChatID(src.Translate.For, dstChatID) {
		text, err = s.translator.TranslateText(ctx, text, src.Translate.Lang)
		if err != nil {
			s.logger.Error("Translation failed", "error", err)
		}
	}

	// Замена фрагментов
	if dst != nil {
		for _, fragment := range dst.ReplaceFragments {
			text = strings.ReplaceAll(text, fragment.From, fragment.To)
		}
	}

	// Подпись
	if src.Sign != nil && containsChatID(src.Sign.For, dstChatID) {
		text = text + "\n**" + src.Sign.Title + "**"
	}

	// Ссылка на источник
	if src.Link != nil && containsChatID(src.Link.For, dstChatID) {
		text = text + "\n" + src.Link.Title
	}

	return text, nil
}

func containsChatID(ids []domain.ChatID, target domain.ChatID) bool {
	for _, id := range ids {
		if id == target {
			return true
		}
	}
	return false
}
