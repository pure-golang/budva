package filters

import (
	"log/slog"
	"regexp"
	"slices"

	"github.com/pure-golang/budva-claude/internal/domain"
)

// Service определяет режим фильтрации для сообщений.
type Service struct {
	logger *slog.Logger
}

// New создаёт новый экземпляр сервиса фильтрации.
func New() *Service {
	return &Service{
		logger: slog.Default().With("module", "service.filters"),
	}
}

// Evaluate определяет режим фильтрации для текста по правилу пересылки.
func (s *Service) Evaluate(text string, rule *domain.ForwardRule) domain.FiltersMode {
	if text == "" {
		if hasIncludeRules(rule) {
			return domain.FiltersOther
		}
		return domain.FiltersOK
	}

	if rule.Exclude != "" {
		re := regexp.MustCompile("(?i)" + rule.Exclude)
		if re.MatchString(text) {
			return domain.FiltersCheck
		}
	}

	hasInclude := false

	if rule.Include != "" {
		hasInclude = true
		re := regexp.MustCompile("(?i)" + rule.Include)
		if re.MatchString(text) {
			return domain.FiltersOK
		}
	}

	for _, submatch := range rule.IncludeSubmatch {
		if submatch.Regexp == "" {
			continue
		}
		hasInclude = true
		re := regexp.MustCompile("(?i)" + submatch.Regexp)
		matches := re.FindAllStringSubmatch(text, -1)
		for _, match := range matches {
			if submatch.Group < len(match) && slices.Contains(submatch.Match, match[submatch.Group]) {
				return domain.FiltersOK
			}
		}
	}

	if hasInclude {
		return domain.FiltersOther
	}

	return domain.FiltersOK
}

func hasIncludeRules(rule *domain.ForwardRule) bool {
	if rule.Include != "" {
		return true
	}
	for _, submatch := range rule.IncludeSubmatch {
		if submatch.Regexp != "" {
			return true
		}
	}
	return false
}
