package ruleset

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

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

func TestRepo_WatchContext_FileWrite_CallsOnChange(t *testing.T) {
	t.Parallel()

	// Arrange
	path := writeRuleset(t, testRuleset)
	r := New(config.RulesetConfig{Path: path})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	called := make(chan struct{}, 1)
	err := r.WatchContext(ctx, func() {
		select {
		case called <- struct{}{}:
		default:
		}
	})
	require.NoError(t, err)
	t.Cleanup(func() { _ = r.Close() })

	// Act — записываем файл заново, чтобы fsnotify сгенерировал Write-событие
	require.NoError(t, os.WriteFile(path, []byte(testRuleset), 0o600))

	// Assert
	select {
	case <-called:
		// OK
	case <-time.After(3 * time.Second):
		t.Fatal("onChange was not called after file write")
	}
}

func TestRepo_WatchContext_ContextCancel_Stops(t *testing.T) {
	t.Parallel()

	// Arrange
	path := writeRuleset(t, testRuleset)
	r := New(config.RulesetConfig{Path: path})

	ctx, cancel := context.WithCancel(context.Background())

	err := r.WatchContext(ctx, func() {})
	require.NoError(t, err)
	t.Cleanup(func() { _ = r.Close() })

	// Act
	cancel()

	// Assert — отмена контекста не вызывает паники, Close не блокирует
}

func TestRepo_WatchContext_InvalidPath_Error(t *testing.T) {
	t.Parallel()

	// Arrange
	r := New(config.RulesetConfig{Path: "/nonexistent/path/to/file.yml"})

	// Act
	err := r.WatchContext(context.Background(), func() {})

	// Assert
	require.Error(t, err)
	assert.Contains(t, err.Error(), "watch file")
}

func TestRepo_Close_WithoutWatcher(t *testing.T) {
	t.Parallel()

	// Arrange
	r := New(config.RulesetConfig{Path: "/tmp/any.yml"})

	// Act / Assert — Close без WatchContext не паникует
	require.NoError(t, r.Close())
}

func TestRepo_Close_WithWatcher(t *testing.T) {
	t.Parallel()

	// Arrange
	path := writeRuleset(t, testRuleset)
	r := New(config.RulesetConfig{Path: path})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := r.WatchContext(ctx, func() {})
	require.NoError(t, err)

	// Act / Assert
	require.NoError(t, r.Close())
}

func TestUtf16Len_SurrogatePairs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		s    string
		want int
	}{
		{name: "ascii", s: "hello", want: 5},
		{name: "empty", s: "", want: 0},
		{name: "emoji_single", s: "😀", want: 2},    // U+1F600 — surrogate pair
		{name: "emoji_flag", s: "🇷🇺", want: 4},     // два символа-компонента флага, каждый — surrogate pair
		{name: "mixed", s: "hi😀", want: 4},         // 2 ASCII + 2 surrogate
		{name: "cjk_basic", s: "日本語", want: 3},    // U+65E5, U+672C, U+8A9E — BMP, каждый 1 unit
		{name: "linear_b", s: "\U00010000", want: 2}, // U+10000 — минимальный surrogate pair
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Act
			got := utf16Len(tt.s)

			// Assert
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestRepo_Load_NegateChatIDs_PrevNext(t *testing.T) {
	t.Parallel()

	// Arrange — источник с prev.for и next.for для проверки negateChatIDs
	path := writeRuleset(t, `
sources:
  1001000:
    prev:
      title: "Prev"
      for: [2001000]
    next:
      title: "Next"
      for: [2001000]
destinations:
  2001000: {}
forwardRules:
  rule1:
    from: 1001000
    to: [2001000]
    sendCopy: true
`)
	r := New(config.RulesetConfig{Path: path})

	// Act
	rs, err := r.Load()

	// Assert
	require.NoError(t, err)
	src := rs.Sources[-1001000]
	require.NotNil(t, src)
	require.NotNil(t, src.Prev)
	assert.Contains(t, src.Prev.For, int64(-2001000))
	require.NotNil(t, src.Next)
	assert.Contains(t, src.Next.For, int64(-2001000))
}

func TestRepo_Load_Enrich_MissingSource(t *testing.T) {
	t.Parallel()

	// Arrange — ForwardRule.From не определён в Sources, enrich создаст синтетический Source
	path := writeRuleset(t, `
sources: {}
destinations:
  2001000: {}
forwardRules:
  rule1:
    from: 1001000
    to: [2001000]
    sendCopy: true
`)
	r := New(config.RulesetConfig{Path: path})

	// Act
	rs, err := r.Load()

	// Assert — enrich добавил синтетический источник
	require.NoError(t, err)
	assert.Contains(t, rs.Sources, int64(-1001000))
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

func TestRepo_Load_InvalidYAML(t *testing.T) {
	t.Parallel()

	// Arrange — файл содержит невалидный YAML
	path := writeRuleset(t, ":\tinvalid: yaml: content\t[broken")
	r := New(config.RulesetConfig{Path: path})

	// Act
	_, err := r.Load()

	// Assert
	require.Error(t, err)
}

func TestNegateChatIDs_NilOpt(t *testing.T) {
	t.Parallel()

	// Arrange / Act / Assert — нетипизированный nil не вызывает паники
	negateChatIDs(nil)
}
