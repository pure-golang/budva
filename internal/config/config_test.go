package config

import (
	"testing"

	"github.com/kelseyhightower/envconfig"
	"github.com/stretchr/testify/require"
)

// CFG-001: TelegramConfig требует обязательные поля.
func TestTelegramConfig_RequiredFields(t *testing.T) {
	// Arrange
	t.Setenv("TELEGRAM_API_ID", "12345")
	t.Setenv("TELEGRAM_API_HASH", "abc123")
	t.Setenv("TELEGRAM_PHONE", "+79001234567")

	// Act
	var cfg TelegramConfig
	err := envconfig.Process("", &cfg)

	// Assert
	require.NoError(t, err)
	require.Equal(t, int32(12345), cfg.APIID)
	require.Equal(t, "abc123", cfg.APIHash)
	require.Equal(t, "+79001234567", cfg.Phone)
}

// CFG-002: TelegramConfig имеет разумные значения по умолчанию.
func TestTelegramConfig_Defaults(t *testing.T) {
	// Arrange
	t.Setenv("TELEGRAM_API_ID", "1")
	t.Setenv("TELEGRAM_API_HASH", "x")
	t.Setenv("TELEGRAM_PHONE", "+1")

	// Act
	var cfg TelegramConfig
	err := envconfig.Process("", &cfg)

	// Assert
	require.NoError(t, err)
	require.Equal(t, ".data/tdlib", cfg.DatabaseDirectory)
	require.Equal(t, ".data/tdlib-files", cfg.FilesDirectory)
	require.Equal(t, "en", cfg.SystemLanguageCode)
	require.Equal(t, "Server", cfg.DeviceModel)
	require.Equal(t, int32(0), cfg.LogVerbosityLevel)
	require.True(t, cfg.UseFileDatabase)
	require.True(t, cfg.UseChatInfoDatabase)
	require.True(t, cfg.UseMessageDatabase)
	require.False(t, cfg.UseSecretChats)
	require.Equal(t, ".data/tdlib-logs", cfg.LogDirectory)
	require.Equal(t, int64(10), cfg.LogMaxFileSize)
}

// CFG-003: StorageConfig имеет разумные значения по умолчанию.
func TestStorageConfig_Defaults(t *testing.T) {
	t.Parallel()

	// Arrange / Act
	var cfg StorageConfig
	err := envconfig.Process("", &cfg)

	// Assert
	require.NoError(t, err)
	require.Equal(t, ".data/badger", cfg.DatabaseDirectory)
}

// CFG-004: RulesetConfig имеет путь к файлу по умолчанию.
func TestRulesetConfig_Defaults(t *testing.T) {
	t.Parallel()

	// Arrange / Act
	var cfg RulesetConfig
	err := envconfig.Process("", &cfg)

	// Assert
	require.NoError(t, err)
	require.Equal(t, "ruleset.yml", cfg.Path)
}
