package filters_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/pure-golang/budva-claude/internal/domain"
	"github.com/pure-golang/budva-claude/internal/service/filters"
)

func TestNew_returns_non_nil_service(t *testing.T) {
	t.Parallel()

	// Arrange + Act
	svc := filters.New()

	// Assert
	require.NotNil(t, svc)
}

func TestEvaluate(t *testing.T) {
	t.Parallel()

	// Arrange общие данные — сервис без состояния, создаётся один раз
	svc := filters.New()

	// Таблица сценариев покрывает ветки Evaluate:
	// пустой текст, exclude/include, submatch, их сочетания и edge cases
	tests := []struct {
		name string
		text string
		rule *domain.ForwardRule
		want domain.FiltersMode
	}{
		{
			// Arrange — пустое правило без фильтров
			name: "no_filters_any_text",
			text: "any text",
			rule: &domain.ForwardRule{},
			want: domain.FiltersOK,
		},
		{
			// Arrange — пустой текст без include-правил пропускается
			name: "empty_text_no_includes",
			text: "",
			rule: &domain.ForwardRule{Exclude: "EXCLUDE"},
			want: domain.FiltersOK,
		},
		{
			// Arrange — пустой текст при наличии Include отправляется в other
			name: "empty_text_with_include",
			text: "",
			rule: &domain.ForwardRule{Include: "INCLUDE"},
			want: domain.FiltersOther,
		},
		{
			// Arrange — пустой текст при наличии submatch отправляется в other
			name: "empty_text_with_submatch",
			text: "",
			rule: &domain.ForwardRule{
				IncludeSubmatch: []*domain.SubmatchRule{
					{Regexp: `\$(\w+)`, Group: 1, Match: []string{"TSLA"}},
				},
			},
			want: domain.FiltersOther,
		},
		{
			// Arrange — пустой текст, submatch с пустым Regexp не считается include
			name: "empty_text_with_empty_submatch_only",
			text: "",
			rule: &domain.ForwardRule{
				IncludeSubmatch: []*domain.SubmatchRule{
					{Regexp: "", Group: 0, Match: []string{"x"}},
				},
			},
			want: domain.FiltersOK,
		},
		{
			// Arrange — exclude срабатывает и возвращает FiltersCheck
			name: "exclude_matches",
			text: "EXCLUDE other",
			rule: &domain.ForwardRule{Exclude: "EXCLUDE"},
			want: domain.FiltersCheck,
		},
		{
			// Arrange — exclude case-insensitive (компилируется с (?i))
			name: "exclude_matches_case_insensitive",
			text: "exclude other",
			rule: &domain.ForwardRule{Exclude: "EXCLUDE"},
			want: domain.FiltersCheck,
		},
		{
			// Arrange — exclude не матчится, нет include — FiltersOK
			name: "exclude_no_match",
			text: "normal text",
			rule: &domain.ForwardRule{Exclude: "EXCLUDE"},
			want: domain.FiltersOK,
		},
		{
			// Arrange — exclude имеет приоритет над include
			name: "exclude_wins_over_include",
			text: "EXCLUDE and INCLUDE",
			rule: &domain.ForwardRule{Exclude: "EXCLUDE", Include: "INCLUDE"},
			want: domain.FiltersCheck,
		},
		{
			// Arrange — include матчится, FiltersOK
			name: "include_matches",
			text: "INCLUDE other",
			rule: &domain.ForwardRule{Include: "INCLUDE"},
			want: domain.FiltersOK,
		},
		{
			// Arrange — include case-insensitive
			name: "include_matches_case_insensitive",
			text: "include other",
			rule: &domain.ForwardRule{Include: "INCLUDE"},
			want: domain.FiltersOK,
		},
		{
			// Arrange — include не матчится, FiltersOther
			name: "include_no_match",
			text: "normal text",
			rule: &domain.ForwardRule{Include: "INCLUDE"},
			want: domain.FiltersOther,
		},
		{
			// Arrange — exclude не матчится, include матчится
			name: "exclude_not_match_include_match",
			text: "INCLUDE only",
			rule: &domain.ForwardRule{Exclude: "XYZ", Include: "INCLUDE"},
			want: domain.FiltersOK,
		},
		{
			// Arrange — submatch по группе совпадает
			name: "submatch_matches",
			text: "$TSLA is up",
			rule: &domain.ForwardRule{
				IncludeSubmatch: []*domain.SubmatchRule{
					{Regexp: `\$(\w+)`, Group: 1, Match: []string{"TSLA"}},
				},
			},
			want: domain.FiltersOK,
		},
		{
			// Arrange — submatch по группе не совпадает с Match
			name: "submatch_no_match_in_list",
			text: "$AAPL is up",
			rule: &domain.ForwardRule{
				IncludeSubmatch: []*domain.SubmatchRule{
					{Regexp: `\$(\w+)`, Group: 1, Match: []string{"TSLA"}},
				},
			},
			want: domain.FiltersOther,
		},
		{
			// Arrange — submatch regex не находит совпадений вообще
			name: "submatch_regex_no_matches",
			text: "no tickers here",
			rule: &domain.ForwardRule{
				IncludeSubmatch: []*domain.SubmatchRule{
					{Regexp: `\$(\w+)`, Group: 1, Match: []string{"TSLA"}},
				},
			},
			want: domain.FiltersOther,
		},
		{
			// Arrange — пустой Regexp в submatch пропускается (continue)
			name: "submatch_empty_regexp_skipped",
			text: "any text",
			rule: &domain.ForwardRule{
				IncludeSubmatch: []*domain.SubmatchRule{
					{Regexp: "", Group: 0, Match: []string{"x"}},
				},
			},
			want: domain.FiltersOK,
		},
		{
			// Arrange — первый submatch с пустым Regexp пропускается, второй матчится
			name: "submatch_skip_empty_then_match",
			text: "$TSLA up",
			rule: &domain.ForwardRule{
				IncludeSubmatch: []*domain.SubmatchRule{
					{Regexp: "", Group: 0, Match: []string{"x"}},
					{Regexp: `\$(\w+)`, Group: 1, Match: []string{"TSLA"}},
				},
			},
			want: domain.FiltersOK,
		},
		{
			// Arrange — Group вне диапазона, FiltersOther
			name: "submatch_group_out_of_range",
			text: "$TSLA up",
			rule: &domain.ForwardRule{
				IncludeSubmatch: []*domain.SubmatchRule{
					{Regexp: `\$(\w+)`, Group: 5, Match: []string{"TSLA"}},
				},
			},
			want: domain.FiltersOther,
		},
		{
			// Arrange — Group=0 совпадает с полным матчем regex
			name: "submatch_group_zero_full_match",
			text: "$TSLA up",
			rule: &domain.ForwardRule{
				IncludeSubmatch: []*domain.SubmatchRule{
					{Regexp: `\$(\w+)`, Group: 0, Match: []string{"$TSLA"}},
				},
			},
			want: domain.FiltersOK,
		},
		{
			// Arrange — два вхождения, второе соответствует Match
			name: "submatch_multiple_matches_second_hits",
			text: "$AAPL and $TSLA",
			rule: &domain.ForwardRule{
				IncludeSubmatch: []*domain.SubmatchRule{
					{Regexp: `\$(\w+)`, Group: 1, Match: []string{"TSLA"}},
				},
			},
			want: domain.FiltersOK,
		},
		{
			// Arrange — Match nil означает, что ни одна подстрока не подойдёт
			name: "submatch_nil_match_list",
			text: "$TSLA up",
			rule: &domain.ForwardRule{
				IncludeSubmatch: []*domain.SubmatchRule{
					{Regexp: `\$(\w+)`, Group: 1, Match: nil},
				},
			},
			want: domain.FiltersOther,
		},
		{
			// Arrange — submatch case-insensitive
			name: "submatch_case_insensitive",
			text: "$tsla up",
			rule: &domain.ForwardRule{
				IncludeSubmatch: []*domain.SubmatchRule{
					{Regexp: `\$(\w+)`, Group: 1, Match: []string{"tsla"}},
				},
			},
			want: domain.FiltersOK,
		},
		{
			// Arrange — Include не матчится, но submatch матчится — FiltersOK
			name: "include_no_match_submatch_match",
			text: "$TSLA",
			rule: &domain.ForwardRule{
				Include: "KEYWORD",
				IncludeSubmatch: []*domain.SubmatchRule{
					{Regexp: `\$(\w+)`, Group: 1, Match: []string{"TSLA"}},
				},
			},
			want: domain.FiltersOK,
		},
		{
			// Arrange — Include и submatch оба не матчатся — FiltersOther
			name: "include_and_submatch_no_match",
			text: "nothing here",
			rule: &domain.ForwardRule{
				Include: "KEYWORD",
				IncludeSubmatch: []*domain.SubmatchRule{
					{Regexp: `\$(\w+)`, Group: 1, Match: []string{"TSLA"}},
				},
			},
			want: domain.FiltersOther,
		},
		{
			// Arrange — unicode в тексте и include
			name: "include_unicode",
			text: "Привет мир",
			rule: &domain.ForwardRule{Include: "Привет"},
			want: domain.FiltersOK,
		},
		{
			// Arrange — unicode case-insensitive
			name: "include_unicode_case_insensitive",
			text: "ПРИВЕТ мир",
			rule: &domain.ForwardRule{Include: "привет"},
			want: domain.FiltersOK,
		},
		{
			// Arrange — unicode в submatch (используем \p{L} для юникод-букв)
			name: "submatch_unicode",
			text: "тикер:ГАЗП растёт",
			rule: &domain.ForwardRule{
				IncludeSubmatch: []*domain.SubmatchRule{
					{Regexp: `тикер:(\p{L}+)`, Group: 1, Match: []string{"ГАЗП"}},
				},
			},
			want: domain.FiltersOK,
		},
		{
			// Arrange — emoji в тексте
			name: "include_emoji",
			text: "price up 📈 now",
			rule: &domain.ForwardRule{Include: "📈"},
			want: domain.FiltersOK,
		},
		{
			// Arrange — альтернация regex
			name: "include_alternation",
			text: "foo happens",
			rule: &domain.ForwardRule{Include: "foo|bar|baz"},
			want: domain.FiltersOK,
		},
		{
			// Arrange — якоря ^/$ работают корректно
			name: "include_anchors",
			text: "start middle end",
			rule: &domain.ForwardRule{Include: "^start"},
			want: domain.FiltersOK,
		},
		{
			// Arrange — метасимвол . без экранирования
			name: "include_dot_metachar",
			text: "a1b",
			rule: &domain.ForwardRule{Include: "a.b"},
			want: domain.FiltersOK,
		},
	}

	for _, tt := range tests {

		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Act
			got := svc.Evaluate(tt.text, tt.rule)

			// Assert
			require.Equal(t, tt.want, got)
		})
	}
}

func TestEvaluate_large_text(t *testing.T) {
	t.Parallel()

	// Arrange — большой текст с include-словом в конце
	svc := filters.New()
	var b strings.Builder
	b.Grow(200_000)
	for range 10_000 {
		b.WriteString("padding ")
	}
	b.WriteString("NEEDLE")
	text := b.String()
	rule := &domain.ForwardRule{Include: "NEEDLE"}

	// Act
	got := svc.Evaluate(text, rule)

	// Assert
	require.Equal(t, domain.FiltersOK, got)
}

func TestEvaluate_large_text_no_match(t *testing.T) {
	t.Parallel()

	// Arrange — большой текст без искомого слова
	svc := filters.New()
	text := strings.Repeat("a ", 50_000)
	rule := &domain.ForwardRule{Include: "NEEDLE"}

	// Act
	got := svc.Evaluate(text, rule)

	// Assert
	require.Equal(t, domain.FiltersOther, got)
}

func TestEvaluate_invalid_exclude_regexp_panics(t *testing.T) {
	t.Parallel()

	// Arrange — невалидный regex в Exclude вызывает panic через MustCompile
	svc := filters.New()
	rule := &domain.ForwardRule{Exclude: "["}

	// Act + Assert — ожидаем panic
	require.Panics(t, func() {
		svc.Evaluate("any", rule)
	})
}

func TestEvaluate_invalid_include_regexp_panics(t *testing.T) {
	t.Parallel()

	// Arrange — невалидный regex в Include
	svc := filters.New()
	rule := &domain.ForwardRule{Include: "(unclosed"}

	// Act + Assert
	require.Panics(t, func() {
		svc.Evaluate("any", rule)
	})
}

func TestEvaluate_invalid_submatch_regexp_panics(t *testing.T) {
	t.Parallel()

	// Arrange — невалидный regex в submatch
	svc := filters.New()
	rule := &domain.ForwardRule{
		IncludeSubmatch: []*domain.SubmatchRule{
			{Regexp: "[", Group: 0, Match: []string{"x"}},
		},
	}

	// Act + Assert
	require.Panics(t, func() {
		svc.Evaluate("any", rule)
	})
}

func TestEvaluate_concurrent_safe(t *testing.T) {
	t.Parallel()

	// Arrange — пакет заявлен потокобезопасным, проверяем при -race
	svc := filters.New()
	rule := &domain.ForwardRule{
		Exclude: "BAD",
		Include: "GOOD",
		IncludeSubmatch: []*domain.SubmatchRule{
			{Regexp: `\$(\w+)`, Group: 1, Match: []string{"TSLA"}},
		},
	}

	// Act — вызываем параллельно
	done := make(chan struct{}, 32)
	for range 32 {
		go func() {
			_ = svc.Evaluate("GOOD $TSLA text", rule)
			done <- struct{}{}
		}()
	}
	for range 32 {
		<-done
	}

	// Assert — детектор гонок не сработал, сам вызов возвращает FiltersOK
	require.Equal(t, domain.FiltersOK, svc.Evaluate("GOOD $TSLA text", rule))
}
