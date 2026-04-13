package config

import (
	agrpc "github.com/pure-golang/adapters/grpc/std"
	ahttp "github.com/pure-golang/adapters/httpserver/std"
	"github.com/pure-golang/platform/monitoring"
)

// Config описывает конфигурацию приложения из переменных окружения.
type Config struct {
	Environment string `envconfig:"ENVIRONMENT" default:"development"`
	Monitoring  monitoring.Config
	HTTPServer  ahttp.Config
	GRPCServer  agrpc.Config
	Telegram    TelegramConfig
	Storage     StorageConfig
	Ruleset     RulesetConfig
}

// TelegramConfig описывает параметры подключения к Telegram API.
type TelegramConfig struct {
	APIID              int32  `envconfig:"TELEGRAM_API_ID" required:"true"`
	APIHash            string `envconfig:"TELEGRAM_API_HASH" required:"true"`
	Phone              string `envconfig:"TELEGRAM_PHONE" required:"true"`
	DatabaseDirectory  string `envconfig:"TELEGRAM_DATABASE_DIR" default:".data/tdlib"`
	FilesDirectory     string `envconfig:"TELEGRAM_FILES_DIR" default:".data/tdlib-files"`
	SystemLanguageCode string `envconfig:"TELEGRAM_SYSTEM_LANG" default:"en"`
	DeviceModel        string `envconfig:"TELEGRAM_DEVICE_MODEL" default:"Server"`
	UseTestDC          bool   `envconfig:"TELEGRAM_USE_TEST_DC" default:"false"`
	LogVerbosityLevel  int32  `envconfig:"TELEGRAM_LOG_VERBOSITY" default:"0"`
}

// StorageConfig описывает параметры KV-хранилища BadgerDB.
type StorageConfig struct {
	DatabaseDirectory string `envconfig:"STORAGE_PATH" default:".data/badger"`
}

// RulesetConfig описывает параметры загрузки правил пересылки.
type RulesetConfig struct {
	Path string `envconfig:"RULESET_PATH" default:"ruleset.yml"`
}
