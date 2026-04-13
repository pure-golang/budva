package ruleset

import (
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/pure-golang/budva/internal/config"
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
	// Arrange
	dir := t.TempDir()
	path := filepath.Join(dir, "ruleset.yml")
	err := os.WriteFile(path, []byte(testRuleset), 0o644)
	require.NoError(t, err)

	r := New(config.RulesetConfig{Path: path}, slog.Default())

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
	// Arrange
	dir := t.TempDir()
	path := filepath.Join(dir, "ruleset.yml")
	err := os.WriteFile(path, []byte("{}"), 0o644)
	require.NoError(t, err)

	r := New(config.RulesetConfig{Path: path}, slog.Default())

	// Act
	_, err = r.Load()

	// Assert
	require.ErrorIs(t, err, ErrEmptyConfig)
}

func TestRepo_Load_file_not_found(t *testing.T) {
	// Arrange
	r := New(config.RulesetConfig{Path: "/nonexistent/ruleset.yml"}, slog.Default())

	// Act
	_, err := r.Load()

	// Assert
	require.Error(t, err)
}
