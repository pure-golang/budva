package term

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"testing/synctest"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zelenin/go-tdlib/client"

	"github.com/pure-golang/budva-claude/internal/domain"
)

type fakeAuth struct {
	subscribeFn func(func(domain.AuthorizationState, any))
	inputChan   chan string
	state       domain.AuthorizationState
	logoutErr   error
}

func (f *fakeAuth) Subscribe(listener func(domain.AuthorizationState, any)) {
	if f.subscribeFn != nil {
		f.subscribeFn(listener)
	}
}
func (f *fakeAuth) InputChan() chan<- string         { return f.inputChan }
func (f *fakeAuth) State() domain.AuthorizationState { return f.state }
func (f *fakeAuth) LogOut(_ context.Context) error   { return f.logoutErr }

type fakeTelegram struct {
	clientDone  chan struct{}
	getOptionFn func(*client.GetOptionRequest) (client.OptionValue, error)
	getMeFn     func() (*client.User, error)
}

func (f *fakeTelegram) ClientDone() <-chan struct{} { return f.clientDone }
func (f *fakeTelegram) GetMe() (*client.User, error) {
	if f.getMeFn != nil {
		return f.getMeFn()
	}
	return &client.User{Id: 123}, nil
}
func (f *fakeTelegram) GetOption(req *client.GetOptionRequest) (client.OptionValue, error) {
	if f.getOptionFn != nil {
		return f.getOptionFn(req)
	}
	return nil, nil
}

type fakeTerm struct {
	readLineFn     func() (string, error)
	readPasswordFn func() (string, error)
}

func (f *fakeTerm) ReadLine() (string, error)     { return f.readLineFn() }
func (f *fakeTerm) ReadPassword() (string, error) { return f.readPasswordFn() }
func (f *fakeTerm) Println(_ ...any)              {}
func (f *fakeTerm) Printf(_ string, _ ...any)     {}

type capturingTerm struct {
	readLineFn     func() (string, error)
	readPasswordFn func() (string, error)
	output         []string
}

func (c *capturingTerm) ReadLine() (string, error) {
	if c.readLineFn != nil {
		return c.readLineFn()
	}
	return "", nil
}

func (c *capturingTerm) ReadPassword() (string, error) {
	if c.readPasswordFn != nil {
		return c.readPasswordFn()
	}
	return "", nil
}

func (c *capturingTerm) Println(a ...any)          { c.output = append(c.output, fmt.Sprint(a...)) }
func (c *capturingTerm) Printf(f string, a ...any) { c.output = append(c.output, fmt.Sprintf(f, a...)) }
func (c *capturingTerm) joined() string            { return strings.Join(c.output, "\n") }

func TestRunInputLoop_Exit(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		// Arrange
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()

		clientDone := make(chan struct{})
		close(clientDone)

		auth := &fakeAuth{
			subscribeFn: func(func(domain.AuthorizationState, any)) {},
			inputChan:   make(chan string, 1),
		}
		tg := &fakeTelegram{clientDone: clientDone}
		tm := &fakeTerm{
			readLineFn: func() (string, error) { return "exit", nil },
		}

		tr := New(auth, tg, tm, "")
		shutdownCh := make(chan struct{}, 1)
		tr.shutdown = func() {
			select {
			case shutdownCh <- struct{}{}:
			default:
			}
			cancel()
		}

		// Act
		go tr.runInputLoop(ctx)

		// Assert
		select {
		case <-shutdownCh:
			// OK
		case <-time.After(1 * time.Second):
			t.Error("shutdown was not called after exit command")
		}
	})
}

func TestPrintStatus(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		getOptionFn func(*client.GetOptionRequest) (client.OptionValue, error)
		wantVersion string
	}{
		{
			name: "with_version",
			getOptionFn: func(*client.GetOptionRequest) (client.OptionValue, error) {
				return &client.OptionValueString{Value: "1.8.35"}, nil
			},
			wantVersion: "1.8.35",
		},
		{
			name: "getOption_error",
			getOptionFn: func(*client.GetOptionRequest) (client.OptionValue, error) {
				return nil, errors.New("tdlib error")
			},
			wantVersion: "",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			auth := &fakeAuth{inputChan: make(chan string, 1)}
			tg := &fakeTelegram{
				clientDone:  make(chan struct{}),
				getOptionFn: test.getOptionFn,
			}
			term := &capturingTerm{}
			tr := New(auth, tg, term, "")

			// Act
			tr.printStatus(t.Context())

			// Assert
			out := term.joined()
			if test.wantVersion != "" {
				assert.Contains(t, out, test.wantVersion)
			}
			assert.Contains(t, out, "123") // User ID from fakeTelegram.GetMe
		})
	}
}

func TestProcessAuth_WaitPhone(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		phoneNumber string
		termInput   string
		wantPhone   string
	}{
		{name: "from_config", phoneNumber: "+1234567890", wantPhone: "+1234567890"},
		{name: "manual_input", phoneNumber: "", termInput: "+9876543210", wantPhone: "+9876543210"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			inputChan := make(chan string, 1)
			auth := &fakeAuth{inputChan: inputChan}
			term := &capturingTerm{
				readPasswordFn: func() (string, error) { return test.termInput, nil },
			}
			tr := New(auth, nil, term, test.phoneNumber)

			// Act
			tr.processAuth(domain.AuthStateWaitPhone, nil)

			// Assert
			assert.Equal(t, test.wantPhone, <-inputChan)
		})
	}
}

func TestProcessAuth_WaitCode(t *testing.T) {
	t.Parallel()

	// Arrange
	inputChan := make(chan string, 1)
	auth := &fakeAuth{inputChan: inputChan}
	term := &capturingTerm{
		readPasswordFn: func() (string, error) { return "123456", nil },
	}
	tr := New(auth, nil, term, "")

	// Act
	tr.processAuth(domain.AuthStateWaitCode, nil)

	// Assert
	assert.Equal(t, "123456", <-inputChan)
}

func TestProcessAuth_Ready(t *testing.T) {
	t.Parallel()

	// Arrange
	auth := &fakeAuth{inputChan: make(chan string, 1)}
	term := &capturingTerm{}
	tr := New(auth, nil, term, "")

	// Act
	tr.processAuth(domain.AuthStateReady, nil)

	// Assert
	assert.Contains(t, term.joined(), "Authorization complete")
}

func TestProcessCommand_Help(t *testing.T) {
	t.Parallel()

	// Arrange
	auth := &fakeAuth{inputChan: make(chan string, 1)}
	term := &capturingTerm{}
	tr := New(auth, nil, term, "")

	// Act
	tr.processCommand("help")

	// Assert
	out := term.joined()
	assert.Contains(t, out, "help")
	assert.Contains(t, out, "logout")
	assert.Contains(t, out, "exit")
}

func TestProcessCommand_Unknown(t *testing.T) {
	t.Parallel()

	// Arrange
	auth := &fakeAuth{inputChan: make(chan string, 1)}
	term := &capturingTerm{}
	tr := New(auth, nil, term, "")

	// Act
	tr.processCommand("unknown_cmd")

	// Assert
	assert.Contains(t, term.joined(), "Unknown command: unknown_cmd")
}

func TestProcessCommand_Empty(t *testing.T) {
	t.Parallel()

	// Arrange
	auth := &fakeAuth{inputChan: make(chan string, 1)}
	term := &capturingTerm{}
	tr := New(auth, nil, term, "")

	// Act
	tr.processCommand("")

	// Assert — нет вывода, нет паники
	assert.Empty(t, term.output)
}

func TestHandleLogout_Success(t *testing.T) {
	t.Parallel()

	// Arrange
	auth := &fakeAuth{inputChan: make(chan string, 1)}
	term := &capturingTerm{}
	tr := New(auth, nil, term, "")
	shutdownCalled := false
	tr.shutdown = func() { shutdownCalled = true }

	// Act
	tr.handleLogout(nil)

	// Assert
	assert.True(t, shutdownCalled)
	assert.Contains(t, term.joined(), "Logged out successfully")
}

func TestHandleLogout_Error(t *testing.T) {
	t.Parallel()

	// Arrange
	auth := &fakeAuth{
		inputChan: make(chan string, 1),
		logoutErr: errors.New("network error"),
	}
	term := &capturingTerm{}
	tr := New(auth, nil, term, "")
	shutdownCalled := false
	tr.shutdown = func() { shutdownCalled = true }

	// Act
	tr.handleLogout(nil)

	// Assert
	assert.False(t, shutdownCalled)
	assert.Contains(t, term.joined(), "Logout failed")
}

func TestProcessAuth_WaitPassword(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		passwordHint string
		userInput    string
	}{
		{name: "with_hint", passwordHint: "test hint", userInput: "password123"},
		{name: "without_hint", passwordHint: "", userInput: "mypassword"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			inputChan := make(chan string, 1)
			auth := &fakeAuth{inputChan: inputChan}
			term := &fakeTerm{
				readPasswordFn: func() (string, error) { return test.userInput, nil },
			}

			tr := New(auth, nil, term, "")

			var extra any
			if test.passwordHint != "" {
				extra = &domain.WaitPasswordState{PasswordHint: test.passwordHint}
			}

			// Act
			tr.processAuth(domain.AuthStateWaitPassword, extra)

			// Assert
			assert.Equal(t, test.userInput, <-inputChan)
		})
	}
}

func TestProcessAuth_WaitPhone_ReadPasswordError(t *testing.T) {
	t.Parallel()

	// Arrange
	auth := &fakeAuth{inputChan: make(chan string, 1)}
	term := &fakeTerm{
		readPasswordFn: func() (string, error) { return "", errors.New("input error") },
	}
	tr := New(auth, nil, term, "")

	// Act — не паникует, ошибка логируется
	tr.processAuth(domain.AuthStateWaitPhone, nil)

	// Assert — ничего не отправлено в канал
	assert.Equal(t, 0, len(auth.inputChan))
}

func TestProcessAuth_WaitCode_ReadPasswordError(t *testing.T) {
	t.Parallel()

	// Arrange
	auth := &fakeAuth{inputChan: make(chan string, 1)}
	term := &fakeTerm{
		readPasswordFn: func() (string, error) { return "", errors.New("input error") },
	}
	tr := New(auth, nil, term, "")

	// Act
	tr.processAuth(domain.AuthStateWaitCode, nil)

	// Assert — ничего не отправлено
	assert.Equal(t, 0, len(auth.inputChan))
}

func TestProcessAuth_WaitPassword_ReadPasswordError(t *testing.T) {
	t.Parallel()

	// Arrange
	auth := &fakeAuth{inputChan: make(chan string, 1)}
	term := &fakeTerm{
		readPasswordFn: func() (string, error) { return "", errors.New("input error") },
	}
	tr := New(auth, nil, term, "")

	// Act
	tr.processAuth(domain.AuthStateWaitPassword, nil)

	// Assert — ничего не отправлено
	assert.Equal(t, 0, len(auth.inputChan))
}

func TestHandleExit(t *testing.T) {
	t.Parallel()

	// Arrange
	auth := &fakeAuth{inputChan: make(chan string, 1)}
	term := &capturingTerm{}
	tr := New(auth, nil, term, "")
	shutdownCalled := false
	tr.shutdown = func() { shutdownCalled = true }

	// Act
	tr.handleExit(nil)

	// Assert
	assert.True(t, shutdownCalled)
	assert.Contains(t, term.joined(), "Shutting down")
}

func TestHandleExit_NoShutdown(t *testing.T) {
	t.Parallel()

	// Arrange
	auth := &fakeAuth{inputChan: make(chan string, 1)}
	term := &capturingTerm{}
	tr := New(auth, nil, term, "")
	tr.shutdown = nil

	// Act / Assert — не паникует без shutdown
	tr.handleExit(nil)
}

func TestClose(t *testing.T) {
	t.Parallel()

	// Arrange
	auth := &fakeAuth{
		subscribeFn: func(func(domain.AuthorizationState, any)) {},
		inputChan:   make(chan string, 1),
	}
	tg := &fakeTelegram{clientDone: make(chan struct{})}
	term := &fakeTerm{
		readLineFn: func() (string, error) { return "", nil },
	}
	tr := New(auth, tg, term, "")

	// Act / Assert — не паникует
	require.NoError(t, tr.Close())
}

func TestRun(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		// Arrange
		ctx, cancel := context.WithCancel(t.Context())

		auth := &fakeAuth{
			subscribeFn: func(func(domain.AuthorizationState, any)) {},
			inputChan:   make(chan string, 1),
		}
		tg := &fakeTelegram{clientDone: make(chan struct{})}
		term := &fakeTerm{
			readLineFn: func() (string, error) { return "", nil },
		}
		tr := New(auth, tg, term, "")

		// Act — Run блокирует, запускаем в горутине
		done := make(chan error, 1)
		go func() {
			done <- tr.Run(ctx, cancel)
		}()

		// Отменяем контекст, чтобы runInputLoop вышел
		cancel()

		// Assert
		select {
		case err := <-done:
			require.NoError(t, err)
		case <-time.After(1 * time.Second):
			t.Error("Run did not return after context cancellation")
		}
	})
}

func TestRunInputLoop_ContextDone(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		// Arrange
		ctx, cancel := context.WithCancel(t.Context())

		auth := &fakeAuth{
			subscribeFn: func(func(domain.AuthorizationState, any)) {},
			inputChan:   make(chan string, 1),
		}
		tg := &fakeTelegram{clientDone: make(chan struct{})}
		term := &fakeTerm{
			readLineFn: func() (string, error) { return "", nil },
		}
		tr := New(auth, tg, term, "")

		done := make(chan struct{})
		go func() {
			tr.runInputLoop(ctx)
			close(done)
		}()

		// Act
		cancel()

		// Assert
		select {
		case <-done:
			// OK — вышел по ctx.Done()
		case <-time.After(1 * time.Second):
			t.Error("runInputLoop did not exit after context cancellation")
		}
	})
}

func TestRunInputLoop_AuthStateChannel(t *testing.T) {
	t.Parallel()

	// Arrange
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	inputChan := make(chan string, 1)
	subscribed := make(chan struct{})
	var capturedListener func(domain.AuthorizationState, any)
	auth := &fakeAuth{
		subscribeFn: func(listener func(domain.AuthorizationState, any)) {
			capturedListener = listener
			close(subscribed)
		},
		inputChan: inputChan,
	}
	tg := &fakeTelegram{clientDone: make(chan struct{})}
	term := &capturingTerm{
		readPasswordFn: func() (string, error) { return "+1234567890", nil },
	}
	tr := New(auth, tg, term, "")

	go tr.Run(ctx, cancel) //nolint:errcheck // результат горутины не отслеживается в тесте

	// Ждём вызова Subscribe
	select {
	case <-subscribed:
	case <-time.After(time.Second):
		t.Fatal("Subscribe was not called")
	}

	// Act — симулируем переход состояния через подписчика
	capturedListener(domain.AuthStateWaitPhone, nil)

	// Assert — WaitPhone отправляет номер в канал
	select {
	case phone := <-inputChan:
		assert.Equal(t, "+1234567890", phone)
	case <-time.After(time.Second):
		t.Error("phone was not sent to inputChan")
	}
}

func TestRunInputLoop_ReadLineError(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		// Arrange
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()

		clientDone := make(chan struct{})
		close(clientDone)

		auth := &fakeAuth{
			subscribeFn: func(func(domain.AuthorizationState, any)) {},
			inputChan:   make(chan string, 1),
		}
		tg := &fakeTelegram{clientDone: clientDone}
		exited := make(chan struct{})
		tm := &fakeTerm{
			readLineFn: func() (string, error) {
				return "", errors.New("terminal closed")
			},
		}
		tr := New(auth, tg, tm, "")

		// Act
		go func() {
			tr.runInputLoop(ctx)
			close(exited)
		}()

		// Assert — runInputLoop завершается при ошибке ReadLine
		select {
		case <-exited:
			// OK
		case <-time.After(1 * time.Second):
			t.Error("runInputLoop did not exit after ReadLine error")
		}
	})
}

func TestPrintStatus_GetMeError(t *testing.T) {
	t.Parallel()

	// Arrange
	auth := &fakeAuth{inputChan: make(chan string, 1)}
	tg := &fakeTelegram{
		clientDone: make(chan struct{}),
		getMeFn: func() (*client.User, error) {
			return nil, errors.New("get me error")
		},
	}
	term := &capturingTerm{}
	tr := New(auth, tg, term, "")

	// Act
	tr.printStatus(t.Context())

	// Assert — User ID равен 0 при ошибке GetMe
	assert.Contains(t, term.joined(), "User ID: 0")
}

func TestProcessCommand_WithArgs(t *testing.T) {
	t.Parallel()

	// Arrange
	auth := &fakeAuth{inputChan: make(chan string, 1)}
	term := &capturingTerm{}
	tr := New(auth, nil, term, "")
	shutdownCalled := false
	tr.shutdown = func() { shutdownCalled = true }

	// Act — команда с аргументами (exit игнорирует аргументы)
	tr.processCommand("exit extra arg")

	// Assert
	assert.True(t, shutdownCalled)
}
