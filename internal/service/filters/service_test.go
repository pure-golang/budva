package filters

import (
	"log/slog"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/pure-golang/budva/internal/domain"
)

func TestEvaluate_no_filters(t *testing.T) {
	t.Parallel()

	// Arrange
	svc := New(slog.Default())
	rule := &domain.ForwardRule{}

	// Act
	mode := svc.Evaluate("any text", rule)

	// Assert
	require.Equal(t, domain.FiltersOK, mode)
}

func TestEvaluate_exclude_matches(t *testing.T) {
	t.Parallel()

	// Arrange
	svc := New(slog.Default())
	rule := &domain.ForwardRule{Exclude: "EXCLUDE"}

	// Act
	mode := svc.Evaluate("EXCLUDE other", rule)

	// Assert
	require.Equal(t, domain.FiltersCheck, mode)
}

func TestEvaluate_exclude_no_match(t *testing.T) {
	t.Parallel()

	// Arrange
	svc := New(slog.Default())
	rule := &domain.ForwardRule{Exclude: "EXCLUDE"}

	// Act
	mode := svc.Evaluate("normal text", rule)

	// Assert
	require.Equal(t, domain.FiltersOK, mode)
}

func TestEvaluate_include_matches(t *testing.T) {
	t.Parallel()

	// Arrange
	svc := New(slog.Default())
	rule := &domain.ForwardRule{Include: "INCLUDE"}

	// Act
	mode := svc.Evaluate("INCLUDE other", rule)

	// Assert
	require.Equal(t, domain.FiltersOK, mode)
}

func TestEvaluate_include_no_match(t *testing.T) {
	t.Parallel()

	// Arrange
	svc := New(slog.Default())
	rule := &domain.ForwardRule{Include: "INCLUDE"}

	// Act
	mode := svc.Evaluate("normal text", rule)

	// Assert
	require.Equal(t, domain.FiltersOther, mode)
}

func TestEvaluate_empty_text_with_include(t *testing.T) {
	t.Parallel()

	// Arrange
	svc := New(slog.Default())
	rule := &domain.ForwardRule{Include: "INCLUDE"}

	// Act
	mode := svc.Evaluate("", rule)

	// Assert
	require.Equal(t, domain.FiltersOther, mode)
}

func TestEvaluate_submatch(t *testing.T) {
	t.Parallel()

	// Arrange
	svc := New(slog.Default())
	rule := &domain.ForwardRule{
		IncludeSubmatch: []*domain.SubmatchRule{
			{Regexp: `\$(\w+)`, Group: 1, Match: []string{"TSLA"}},
		},
	}

	// Act
	mode := svc.Evaluate("$TSLA is up", rule)

	// Assert
	require.Equal(t, domain.FiltersOK, mode)
}

func TestEvaluate_submatch_no_match(t *testing.T) {
	t.Parallel()

	// Arrange
	svc := New(slog.Default())
	rule := &domain.ForwardRule{
		IncludeSubmatch: []*domain.SubmatchRule{
			{Regexp: `\$(\w+)`, Group: 1, Match: []string{"TSLA"}},
		},
	}

	// Act
	mode := svc.Evaluate("$AAPL is up", rule)

	// Assert
	require.Equal(t, domain.FiltersOther, mode)
}
