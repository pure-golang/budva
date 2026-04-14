package term

import (
	"context"
	"testing"
	"testing/synctest"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/pure-golang/budva-claude/internal/domain"
)

type fakeAuth struct {
	subscribeFn func(func(domain.AuthorizationState, any))
	inputChan   chan string
	state       domain.AuthorizationState
}

func (f *fakeAuth) Subscribe(listener func(domain.AuthorizationState, any)) {
	if f.subscribeFn != nil {
		f.subscribeFn(listener)
	}
}
func (f *fakeAuth) InputChan() chan<- string         { return f.inputChan }
func (f *fakeAuth) State() domain.AuthorizationState { return f.state }

type fakeTelegram struct {
	clientDone chan struct{}
}

func (f *fakeTelegram) ClientDone() <-chan struct{} { return f.clientDone }
func (f *fakeTelegram) GetOption(_ context.Context, _ string) (string, error) {
	return "1.0.0", nil
}
func (f *fakeTelegram) GetMe(_ context.Context) (int64, error) { return 123, nil }

type fakeTerm struct {
	readLineFn     func() (string, error)
	readPasswordFn func() (string, error)
}

func (f *fakeTerm) ReadLine() (string, error)     { return f.readLineFn() }
func (f *fakeTerm) ReadPassword() (string, error) { return f.readPasswordFn() }
func (f *fakeTerm) Println(_ ...any)              {}
func (f *fakeTerm) Printf(_ string, _ ...any)     {}

func TestRunInputLoop_Exit(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
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
		shutdownCalled := false
		tr.shutdown = func() {
			shutdownCalled = true
			cancel()
		}

		go tr.runInputLoop(ctx)
		time.Sleep(1 * time.Millisecond)

		assert.True(t, shutdownCalled, "shutdown should be called after exit command")
	})
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
			tr.processAuth(domain.AuthStateWaitPassword, extra)

			assert.Equal(t, test.userInput, <-inputChan)
		})
	}
}
