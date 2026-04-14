package integration

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pure-golang/budva-claude/internal/config"
	"github.com/pure-golang/budva-claude/internal/repo/ruleset"
)

func writeRuleset(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "ruleset.yml")
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
	return path
}

func TestRuleset_FullPipeline(t *testing.T) {
	t.Parallel()

	// Arrange
	path := writeRuleset(t, `
sources:
  1001000:
    sign:
      title: "Test Source"
      for: [2001000, 3001000]
destinations:
  2001000:
    replaceFragments:
      - from: "hello"
        to: "world"
  3001000: {}
forwardRules:
  rule1:
    from: 1001000
    to: [2001000, 3001000]
    sendCopy: true
`)

	// Act
	repo := ruleset.New(config.RulesetConfig{Path: path})
	rs, err := repo.Load()

	// Assert
	require.NoError(t, err)

	// Chat IDs negated
	assert.Contains(t, rs.UniqueSources, int64(-1001000))
	assert.Contains(t, rs.UniqueDestinations, int64(-2001000))
	assert.Contains(t, rs.UniqueDestinations, int64(-3001000))

	// Source enriched
	src := rs.Sources[-1001000]
	require.NotNil(t, src)
	assert.Equal(t, int64(-1001000), src.ChatID)
	assert.Equal(t, "Test Source", src.Sign.Title)
	assert.Contains(t, src.Sign.For, int64(-2001000))

	// Destination enriched
	dst := rs.Destinations[-2001000]
	require.NotNil(t, dst)
	require.Len(t, dst.ReplaceFragments, 1)
	assert.Equal(t, "hello", dst.ReplaceFragments[0].From)

	// Forward rule enriched
	rule := rs.ForwardRules["rule1"]
	require.NotNil(t, rule)
	assert.Equal(t, int64(-1001000), rule.From)
	assert.Equal(t, []int64{int64(-2001000), int64(-3001000)}, rule.To)
	assert.True(t, rule.SendCopy)
}

func TestRuleset_EmptyConfig(t *testing.T) {
	t.Parallel()

	// Arrange
	path := writeRuleset(t, `
sources: {}
destinations: {}
forwardRules: {}
`)

	// Act
	repo := ruleset.New(config.RulesetConfig{Path: path})
	_, err := repo.Load()

	// Assert
	require.ErrorIs(t, err, ruleset.ErrEmptyConfig)
}

func TestRuleset_FragmentUTF16Validation(t *testing.T) {
	t.Parallel()

	// Arrange — From и To разной UTF-16 длины
	path := writeRuleset(t, `
sources:
  1001000: {}
destinations:
  2001000:
    replaceFragments:
      - from: "ab"
        to: "abc"
forwardRules:
  rule1:
    from: 1001000
    to: [2001000]
    sendCopy: true
`)

	// Act
	repo := ruleset.New(config.RulesetConfig{Path: path})
	_, err := repo.Load()

	// Assert
	require.Error(t, err)
	assert.Contains(t, err.Error(), "UTF-16 lengths differ")
}

func TestRuleset_NegationOfForListIDs(t *testing.T) {
	t.Parallel()

	// Arrange
	path := writeRuleset(t, `
sources:
  1001000:
    translate:
      lang: "ru"
      for: [2001000]
destinations:
  2001000: {}
forwardRules:
  rule1:
    from: 1001000
    to: [2001000]
    sendCopy: true
`)

	// Act
	repo := ruleset.New(config.RulesetConfig{Path: path})
	rs, err := repo.Load()

	// Assert
	require.NoError(t, err)
	src := rs.Sources[-1001000]
	require.NotNil(t, src)
	require.NotNil(t, src.Translate)
	assert.Contains(t, src.Translate.For, int64(-2001000))
}
