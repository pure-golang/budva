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
	clientDone chan struct{}
}

func (f *fakeTelegram) ClientDone() <-chan struct{} { return f.clientDone }
func (f *fakeTelegram) GetMe() (*client.User, error) {
	return &client.User{Id: 123}, nil
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

func (c *capturingTerm) Println(a ...any)              { c.output = append(c.output, fmt.Sprint(a...)) }
func (c *capturingTerm) Printf(f string, a ...any)     { c.output = append(c.output, fmt.Sprintf(f, a...)) }
func (c *capturingTerm) joined() string                { return strings.Join(c.output, "\n") }

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
