package ruleset

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pure-golang/budva-claude/internal/config"
)

const testRuleset = `
sources:
  1002641439846:
    sign:
      title: "Sign"
      for: [1002667730628]
    link:
      title: "🔗Link"
      for: [1002667730628]

destinations:
  1002667730628:
    replaceMyselfLinks:
      run: true
      deleteExternal: true
    replaceFragments:
      - from: "hello"
        to: "12345"

forwardRules:
  CopyRule:
    from: 1002641439846
    to: [1002667730628]
    sendCopy: true
    copyOnce: true
    indelible: true
    exclude: "EXCLUDE"
`

func TestRepo_Load(t *testing.T) {
	t.Parallel()

	// Arrange
	dir := t.TempDir()
	path := filepath.Join(dir, "ruleset.yml")
	err := os.WriteFile(path, []byte(testRuleset), 0o600)
	require.NoError(t, err)

	r := New(config.RulesetConfig{Path: path})

	// Act
	rs, err := r.Load()

	// Assert
	require.NoError(t, err)
	require.NotNil(t, rs)
	require.Len(t, rs.ForwardRules, 1)
	require.Contains(t, rs.ForwardRules, "CopyRule")

	rule := rs.ForwardRules["CopyRule"]
	require.Equal(t, int64(-1002641439846), rule.From)
	require.Equal(t, []int64{-1002667730628}, rule.To)
	require.True(t, rule.SendCopy)
	require.True(t, rule.CopyOnce)
	require.True(t, rule.Indelible)
	require.Equal(t, "EXCLUDE", rule.Exclude)
}

func TestRepo_Load_empty_config(t *testing.T) {
	t.Parallel()

	// Arrange
	dir := t.TempDir()
	path := filepath.Join(dir, "ruleset.yml")
	err := os.WriteFile(path, []byte("{}"), 0o600)
	require.NoError(t, err)

	r := New(config.RulesetConfig{Path: path})

	// Act
	_, err = r.Load()

	// Assert
	require.ErrorIs(t, err, ErrEmptyConfig)
}

func TestRepo_Load_file_not_found(t *testing.T) {
	t.Parallel()

	// Arrange
	r := New(config.RulesetConfig{Path: "/nonexistent/ruleset.yml"})

	// Act
	_, err := r.Load()

	// Assert
	require.Error(t, err)
}

func writeRuleset(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "ruleset.yml")
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
	return path
}

func TestRepo_Load_Negation(t *testing.T) {
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
	r := New(config.RulesetConfig{Path: path})
	rs, err := r.Load()

	// Assert
	require.NoError(t, err)

	// Chat IDs negated
	assert.Contains(t, rs.UniqueSources, int64(-1001000))
	assert.Contains(t, rs.UniqueDestinations, int64(-2001000))

	// For list IDs negated
	src := rs.Sources[-1001000]
	require.NotNil(t, src)
	require.NotNil(t, src.Translate)
	assert.Contains(t, src.Translate.For, int64(-2001000))
}

func TestRepo_Load_FragmentUTF16Validation(t *testing.T) {
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
	r := New(config.RulesetConfig{Path: path})
	_, err := r.Load()

	// Assert
	require.Error(t, err)
	assert.Contains(t, err.Error(), "UTF-16 lengths differ")
}

func TestRepo_Load_FullPipeline(t *testing.T) {
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
	r := New(config.RulesetConfig{Path: path})
	rs, err := r.Load()

	// Assert
	require.NoError(t, err)

	// Sources enriched
	src := rs.Sources[-1001000]
	require.NotNil(t, src)
	assert.Equal(t, int64(-1001000), src.ChatID)
	assert.Equal(t, "Test Source", src.Sign.Title)
	assert.Contains(t, src.Sign.For, int64(-2001000))

	// Destinations enriched
	dst := rs.Destinations[-2001000]
	require.NotNil(t, dst)
	require.Len(t, dst.ReplaceFragments, 1)
	assert.Equal(t, "hello", dst.ReplaceFragments[0].From)

	// Rule enriched
	rule := rs.ForwardRules["rule1"]
	require.NotNil(t, rule)
	assert.Equal(t, int64(-1001000), rule.From)
	assert.Equal(t, []int64{int64(-2001000), int64(-3001000)}, rule.To)
	assert.True(t, rule.SendCopy)
}

func TestRepo_Load_InvalidRuleID_Colon(t *testing.T) {
	t.Parallel()

	// Arrange
	path := writeRuleset(t, `
sources:
  1001000: {}
destinations:
  2001000: {}
forwardRules:
  "rule:bad":
    from: 1001000
    to: [2001000]
    sendCopy: true
`)
	r := New(config.RulesetConfig{Path: path})

	// Act
	_, err := r.Load()

	// Assert
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid characters")
}

func TestRepo_Load_InvalidRuleID_Comma(t *testing.T) {
	t.Parallel()

	// Arrange
	path := writeRuleset(t, `
sources:
  1001000: {}
destinations:
  2001000: {}
forwardRules:
  "rule,bad":
    from: 1001000
    to: [2001000]
    sendCopy: true
`)
	r := New(config.RulesetConfig{Path: path})

	// Act
	_, err := r.Load()

	// Assert
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid characters")
}

func TestRepo_Load_NegativeFrom(t *testing.T) {
	t.Parallel()

	// Arrange
	path := writeRuleset(t, `
sources:
  1001000: {}
destinations:
  2001000: {}
forwardRules:
  rule1:
    from: -1001000
    to: [2001000]
    sendCopy: true
`)
	r := New(config.RulesetConfig{Path: path})

	// Act
	_, err := r.Load()

	// Assert
	require.Error(t, err)
	assert.Contains(t, err.Error(), "From must be positive")
}

func TestRepo_Load_NegativeTo(t *testing.T) {
	t.Parallel()

	// Arrange
	path := writeRuleset(t, `
sources:
  1001000: {}
destinations:
  2001000: {}
forwardRules:
  rule1:
    from: 1001000
    to: [-2001000]
    sendCopy: true
`)
	r := New(config.RulesetConfig{Path: path})

	// Act
	_, err := r.Load()

	// Assert
	require.Error(t, err)
	assert.Contains(t, err.Error(), "To[0] must be positive")
}

func TestRepo_Load_FromEqualsTo(t *testing.T) {
	t.Parallel()

	// Arrange
	path := writeRuleset(t, `
sources:
  1001000: {}
destinations:
  1001000: {}
forwardRules:
  rule1:
    from: 1001000
    to: [1001000]
    sendCopy: true
`)
	r := New(config.RulesetConfig{Path: path})

	// Act
	_, err := r.Load()

	// Assert
	require.Error(t, err)
	assert.Contains(t, err.Error(), "must differ from From")
}
