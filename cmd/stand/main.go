package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	aenv "github.com/pure-golang/adapters/env"
	"github.com/pure-golang/platform/monitoring"

	"github.com/pure-golang/budva-claude/internal/config"
	"github.com/pure-golang/budva-claude/internal/repo/telegram"
	"github.com/pure-golang/budva-claude/internal/repo/term"
	"github.com/pure-golang/budva-claude/internal/service/auth"
	termtransport "github.com/pure-golang/budva-claude/internal/transport/term"
	"github.com/pure-golang/budva-claude/test/support"
)

const fixturesPath = ".config/stand.json"

// chatSpec описывает чат, который нужно создать.
type chatSpec struct {
	name           string // ключ для BDD Examples: "публичный канал"
	title          string // Telegram title: "SRC PUB CHL"
	usernamePrefix string // CamelCase для username: "SrcPubChl"
	isChannel      bool
	isBasic        bool
	isPublic       bool
}

// Все типы чатов из BDD Examples.
// Теги по образцу легаси: SRC PUB CHL, SRC PRV GRP и т.д.
var specs = []chatSpec{
	{name: "публичный канал", title: "SRC PUB CHL", usernamePrefix: "SrcPubChl", isChannel: true, isPublic: true},
	{name: "приватный канал", title: "SRC PRV CHL", usernamePrefix: "SrcPrvChl", isChannel: true},
	{name: "публичная группа", title: "SRC PUB GRP", usernamePrefix: "SrcPubGrp", isPublic: true},
	{name: "приватная группа", title: "SRC PRV GRP", usernamePrefix: "SrcPrvGrp", isBasic: true},
}

func main() {
	up := flag.Bool("up", false, "Create test chats and save fixtures")
	down := flag.Bool("down", false, "Delete test chats and remove fixtures")
	flag.Parse()

	if !*up && !*down {
		flag.Usage()
		os.Exit(1)
	}
	if *up && *down {
		fmt.Fprintln(os.Stderr, "specify either --up or --down, not both")
		os.Exit(1)
	}

	if err := run(*up); err != nil {
		log.Fatal(err)
	}
}

func run(up bool) error {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	var cfg config.Config
	if err := aenv.InitConfig(&cfg); err != nil {
		return fmt.Errorf("config: %w", err)
	}

	closeMonitoring := monitoring.InitDefault(cfg.Monitoring)
	logger := slog.Default().With("module", "stand")
	defer func() {
		if err := closeMonitoring(); err != nil {
			logger.Error("Failed to close monitoring", slog.Any("err", err))
		}
	}()

	telegramRepo := telegram.New(cfg.Telegram)
	if err := telegramRepo.Start(ctx); err != nil {
		return fmt.Errorf("telegram repo: %w", err)
	}
	defer func() {
		if err := telegramRepo.Close(); err != nil {
			logger.Warn("Failed to close telegram repo", slog.Any("err", err))
		}
	}()

	// Авторизация
	authService := auth.New(telegramRepo)
	authService.Start(ctx)

	termRepo := term.New(os.Stdin, os.Stdout, int(os.Stdin.Fd())) //nolint:gosec // fd всегда 0 для stdin
	termTransport := termtransport.New(authService, telegramRepo, termRepo, cfg.Telegram.Phone)
	go func() {
		if err := termTransport.Run(ctx, cancel); err != nil {
			logger.Error("Terminal transport error", slog.Any("err", err))
		}
	}()

	// Ждём авторизации
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-telegramRepo.ClientDone():
	}

	logger.Info("Authorization complete, running stand")

	if up {
		return standUp(ctx, logger, telegramRepo)
	}
	return standDown(ctx, logger, telegramRepo)
}

func standUp(ctx context.Context, logger *slog.Logger, repo *telegram.Repo) error {
	if _, err := os.Stat(fixturesPath); err == nil {
		return fmt.Errorf("fixtures already exist at %s, run --down first", fixturesPath)
	}

	fixtures := &support.Fixtures{}

	for _, spec := range specs {
		fix, err := createChat(ctx, repo, spec)
		if err != nil {
			return fmt.Errorf("create chat %q: %w", spec.name, err)
		}
		fixtures.Chats = append(fixtures.Chats, fix)

		logger.Info("Chat created",
			slog.String("name", fix.Name),
			slog.Int64("chat_id", fix.ChatID),
			slog.String("type", fix.ChatType),
			slog.Bool("is_channel", fix.IsChannel),
		)
	}

	if err := os.MkdirAll(filepath.Dir(fixturesPath), 0o755); err != nil {
		return fmt.Errorf("mkdir: %w", err)
	}
	if err := support.SaveFixtures(fixturesPath, fixtures); err != nil {
		return fmt.Errorf("save fixtures: %w", err)
	}

	logger.Info("Fixtures saved", slog.String("path", fixturesPath))
	return nil
}

func createChat(ctx context.Context, repo *telegram.Repo, spec chatSpec) (support.ChatFixture, error) {
	fix := support.ChatFixture{
		Name:      spec.name,
		IsChannel: spec.isChannel,
	}

	if spec.isBasic {
		chatID, err := repo.CreateNewBasicGroupChat(ctx, spec.title, nil)
		if err != nil {
			return fix, err
		}
		fix.ChatID = chatID
		fix.ChatType = "basicGroup"
		return fix, nil
	}

	// В TDLib каналы и супергруппы — оба ChatTypeSupergroup
	chatID, supergroupID, err := repo.CreateNewSupergroupChat(ctx, spec.title, spec.isChannel, "")
	if err != nil {
		return fix, err
	}
	fix.ChatID = chatID
	fix.SupergroupID = supergroupID
	fix.ChatType = "supergroup"

	// Username: SrcPubChl_<supergroupID>
	if spec.isPublic && supergroupID != 0 {
		username := fmt.Sprintf("%s_%d", spec.usernamePrefix, supergroupID)
		if err := repo.SetSupergroupUsername(ctx, supergroupID, username); err != nil {
			return fix, fmt.Errorf("set username: %w", err)
		}
		fix.Username = username
	}

	return fix, nil
}

func standDown(ctx context.Context, logger *slog.Logger, repo *telegram.Repo) error {
	fixtures, err := support.LoadFixtures(fixturesPath)
	if err != nil {
		return fmt.Errorf("load fixtures: %w", err)
	}

	for _, chat := range fixtures.Chats {
		if err := repo.DeleteChat(ctx, chat.ChatID); err != nil {
			logger.Warn("Failed to delete chat",
				slog.String("name", chat.Name),
				slog.Int64("chat_id", chat.ChatID),
				slog.Any("err", err),
			)
			continue
		}
		logger.Info("Chat deleted",
			slog.String("name", chat.Name),
			slog.Int64("chat_id", chat.ChatID),
		)
	}

	if err := os.Remove(fixturesPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove fixtures file: %w", err)
	}

	logger.Info("Fixtures removed", slog.String("path", fixturesPath))
	return nil
}
