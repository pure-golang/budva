package term

import (
	"context"
	"log/slog"
	"strings"

	"github.com/pure-golang/budva-claude/internal/domain"
)

type authService interface {
	Subscribe(listener func(state domain.AuthorizationState, extra any))
	InputChan() chan<- string
	State() domain.AuthorizationState
}

type telegramGateway interface {
	ClientDone() <-chan struct{}
	GetOption(ctx context.Context, name string) (string, error)
	GetMe(ctx context.Context) (int64, error)
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
	auth          authService
	telegram      telegramGateway
	term          termIO
	authStateChan chan domain.AuthorizationState
	authExtra     chan any
	commands      []command
	commandMap    map[string]*command
	shutdown      func()
	phoneNumber   string
}

// New создаёт новый экземпляр терминального транспорта.
func New(auth authService, telegram telegramGateway, term termIO, phoneNumber string) *Transport {
	t := &Transport{
		logger:        slog.Default().With("module", "transport.term"),
		auth:          auth,
		telegram:      telegram,
		term:          term,
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

	t.auth.Subscribe(func(state domain.AuthorizationState, extra any) {
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
		case <-t.telegram.ClientDone():
			if !isAuth {
				t.printStatus(ctx)
				isAuth = true
			}
			t.term.Println(">")
			input, err := t.term.ReadLine()
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

func (t *Transport) printStatus(ctx context.Context) {
	version, err := t.telegram.GetOption(ctx, "version")
	if err != nil {
		t.logger.Error("Failed to get TDLib version", slog.Any("err", err))
	}
	userID, err := t.telegram.GetMe(ctx)
	if err != nil {
		t.logger.Error("Failed to get user ID", slog.Any("err", err))
	}
	t.term.Printf("TDLib version: %s, User ID: %d\n", version, userID)
}

func (t *Transport) registerCommands() {
	t.commands = []command{
		{name: "help", description: "Show available commands", handler: t.handleHelp},
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
		t.term.Printf("Unknown command: %s. Type 'help' for available commands.\n", parts[0])
		return
	}
	var args []string
	if len(parts) > 1 {
		args = parts[1:]
	}
	cmd.handler(args)
}

func (t *Transport) handleHelp(_ []string) {
	t.term.Println("Available commands:")
	for _, cmd := range t.commands {
		t.term.Printf("  %-15s - %s\n", cmd.name, cmd.description)
	}
}

func (t *Transport) handleExit(_ []string) {
	t.term.Println("Shutting down...")
	if t.shutdown != nil {
		t.shutdown()
	}
}

func (t *Transport) processAuth(state domain.AuthorizationState, extra any) {
	switch state {
	case domain.AuthStateWaitPhone:
		phone := t.phoneNumber
		if phone == "" {
			t.term.Println("Enter phone number:")
			var err error
			phone, err = t.term.ReadPassword()
			if err != nil {
				t.logger.Error("Failed to read phone", slog.Any("err", err))
				return
			}
		} else {
			t.term.Printf("Phone: %s\n", domain.MaskPhoneNumber(phone))
		}
		t.auth.InputChan() <- phone

	case domain.AuthStateWaitCode:
		t.term.Println("Enter confirmation code:")
		code, err := t.term.ReadPassword()
		if err != nil {
			t.logger.Error("Failed to read code", slog.Any("err", err))
			return
		}
		t.auth.InputChan() <- code

	case domain.AuthStateWaitPassword:
		hint := ""
		if ws, ok := extra.(*domain.WaitPasswordState); ok && ws != nil {
			hint = ws.PasswordHint
		}
		if hint != "" {
			t.term.Printf("Enter password (hint: %s):\n", hint)
		} else {
			t.term.Println("Enter password:")
		}
		password, err := t.term.ReadPassword()
		if err != nil {
			t.logger.Error("Failed to read password", slog.Any("err", err))
			return
		}
		t.auth.InputChan() <- password

	case domain.AuthStateReady:
		t.term.Println("Authorization complete.")
	}
}
