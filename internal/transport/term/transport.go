package term

import (
	"context"
	"log/slog"
	"strings"

	"github.com/zelenin/go-tdlib/client"

	"github.com/pure-golang/budva-claude/internal/domain"
)

type authService interface {
	Subscribe(listener func(state domain.AuthorizationState, extra any))
	InputChan() chan<- string
	State() domain.AuthorizationState
	LogOut(ctx context.Context) error
}

// telegramRepo — частично применяемый интерфейс к repo/telegram.
// ClientDone — собственный метод Repo (канал lifecycle); GetMe — обёртка clientAdapter.
// GetOption — метод *Repo, обёртка над пакетной функцией client.GetOption.
type telegramRepo interface {
	ClientDone() <-chan struct{}
	GetMe() (*client.User, error)
	GetOption(*client.GetOptionRequest) (client.OptionValue, error)
}

type termIO interface {
	ReadLine() (string, error)
	ReadPassword() (string, error)
	Println(a ...any)
	Printf(format string, a ...any)
}

type command struct {
	name        string
	description string
	handler     func(args []string)
}

// Transport реализует терминальный интерфейс для авторизации и CLI-команд.
type Transport struct {
	logger        *slog.Logger
	authService   authService
	telegramRepo  telegramRepo
	termIO        termIO
	authStateChan chan domain.AuthorizationState
	authExtra     chan any
	commands      []command
	commandMap    map[string]*command
	shutdown      func()
	phoneNumber   string
}

// New создаёт новый экземпляр терминального транспорта.
func New(authService authService, telegramRepo telegramRepo, termIO termIO, phoneNumber string) *Transport {
	t := &Transport{
		logger:        slog.Default().With("module", "transport.term"),
		authService:   authService,
		telegramRepo:  telegramRepo,
		termIO:        termIO,
		authStateChan: make(chan domain.AuthorizationState, 10),
		authExtra:     make(chan any, 10),
		phoneNumber:   phoneNumber,
	}
	t.registerCommands()
	return t
}

// Run запускает терминальный интерфейс.
func (t *Transport) Run(ctx context.Context, shutdown func()) error {
	t.shutdown = shutdown

	t.authService.Subscribe(func(state domain.AuthorizationState, extra any) {
		select {
		case t.authStateChan <- state:
			select {
			case t.authExtra <- extra:
			default:
			}
		default:
		}
	})

	t.runInputLoop(ctx)
	return nil
}

// Close закрывает транспорт.
func (t *Transport) Close() error {
	close(t.authStateChan)
	return nil
}

func (t *Transport) runInputLoop(ctx context.Context) {
	isAuth := false
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.telegramRepo.ClientDone():
			if !isAuth {
				t.printStatus(ctx)
				isAuth = true
			}
			t.termIO.Println(">")
			input, err := t.termIO.ReadLine()
			if err != nil {
				return
			}
			t.processCommand(input)
		case state := <-t.authStateChan:
			var extra any
			select {
			case extra = <-t.authExtra:
			default:
			}
			t.processAuth(state, extra)
		}
	}
}

func (t *Transport) printStatus(_ context.Context) {
	var version string
	opt, err := t.telegramRepo.GetOption(&client.GetOptionRequest{Name: "version"})
	if err != nil {
		t.logger.Error("Failed to get TDLib version", slog.Any("err", err))
	} else if v, ok := opt.(*client.OptionValueString); ok {
		version = v.Value
	}
	var userID int64
	user, err := t.telegramRepo.GetMe()
	if err != nil {
		t.logger.Error("Failed to get user ID", slog.Any("err", err))
	} else if user != nil {
		userID = user.Id
	}
	t.termIO.Printf("TDLib version: %s, User ID: %d\n", version, userID)
}

func (t *Transport) registerCommands() {
	t.commands = []command{
		{name: "help", description: "Show available commands", handler: t.handleHelp},
		{name: "logout", description: "Log out from Telegram and exit", handler: t.handleLogout},
		{name: "exit", description: "Exit the program", handler: t.handleExit},
	}
	t.commandMap = make(map[string]*command, len(t.commands))
	for i := range t.commands {
		t.commandMap[t.commands[i].name] = &t.commands[i]
	}
}

func (t *Transport) processCommand(input string) {
	parts := strings.Fields(input)
	if len(parts) == 0 {
		return
	}
	cmd, ok := t.commandMap[parts[0]]
	if !ok {
		t.termIO.Printf("Unknown command: %s. Type 'help' for available commands.\n", parts[0])
		return
	}
	var args []string
	if len(parts) > 1 {
		args = parts[1:]
	}
	cmd.handler(args)
}

func (t *Transport) handleHelp(_ []string) {
	t.termIO.Println("Available commands:")
	for _, cmd := range t.commands {
		t.termIO.Printf("  %-15s - %s\n", cmd.name, cmd.description)
	}
}

func (t *Transport) handleLogout(_ []string) {
	t.termIO.Println("Logging out from Telegram...")
	if err := t.authService.LogOut(context.Background()); err != nil {
		t.termIO.Printf("Logout failed: %v\n", err)
		return
	}
	t.termIO.Println("Logged out successfully")
	if t.shutdown != nil {
		t.shutdown()
	}
}

func (t *Transport) handleExit(_ []string) {
	t.termIO.Println("Shutting down...")
	if t.shutdown != nil {
		t.shutdown()
	}
}

func (t *Transport) processAuth(state domain.AuthorizationState, extra any) {
	switch state {
	case domain.AuthStateWaitPhone:
		phone := t.phoneNumber
		if phone == "" {
			t.termIO.Println("Enter phone number:")
			var err error
			phone, err = t.termIO.ReadPassword()
			if err != nil {
				t.logger.Error("Failed to read phone", slog.Any("err", err))
				return
			}
		} else {
			t.termIO.Printf("Phone: %s\n", domain.MaskPhoneNumber(phone))
		}
		t.authService.InputChan() <- phone

	case domain.AuthStateWaitCode:
		t.termIO.Println("Enter confirmation code:")
		code, err := t.termIO.ReadPassword()
		if err != nil {
			t.logger.Error("Failed to read code", slog.Any("err", err))
			return
		}
		t.authService.InputChan() <- code

	case domain.AuthStateWaitPassword:
		hint := ""
		if ws, ok := extra.(*domain.WaitPasswordState); ok && ws != nil {
			hint = ws.PasswordHint
		}
		if hint != "" {
			t.termIO.Printf("Enter password (hint: %s):\n", hint)
		} else {
			t.termIO.Println("Enter password:")
		}
		password, err := t.termIO.ReadPassword()
		if err != nil {
			t.logger.Error("Failed to read password", slog.Any("err", err))
			return
		}
		t.authService.InputChan() <- password

	case domain.AuthStateReady:
		t.termIO.Println("Authorization complete.")
	}
}
