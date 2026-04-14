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
func (f *fakeAuth) InputChan() chan<- string { return f.inputChan }
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
	readLineFn    func() (string, error)
	readPasswordFn func() (string, error)
	printed       []string
}

func (f *fakeTerm) ReadLine() (string, error) { return f.readLineFn() }
func (f *fakeTerm) ReadPassword() (string, error) { return f.readPasswordFn() }
func (f *fakeTerm) Println(a ...any) {
	// no-op в тестах
}
func (f *fakeTerm) Printf(format string, a ...any) {
	// no-op в тестах
}

func TestRunInputLoop_Exit(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	t.Cleanup(cancel)

	synctest.Run(func() {
		clientDone := make(chan struct{})
		close(clientDone)

		auth := &fakeAuth{
			subscribeFn: func(func(domain.AuthorizationState, any)) {},
			inputChan:   make(chan string, 1),
		}
		telegram := &fakeTelegram{clientDone: clientDone}
		term := &fakeTerm{
			readLineFn: func() (string, error) { return "exit", nil },
		}

		tr := New(auth, telegram, term, "")
		tr.shutdown = cancel

		go tr.runInputLoop(ctx)

		select {
		case <-ctx.Done():
			// OK
		case <-time.After(1 * time.Second):
			t.Error("Transport did not stop after exit command")
			cancel()
		}
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
