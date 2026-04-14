package telegram

import (
	"context"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pure-golang/budva-claude/internal/config"
	"github.com/pure-golang/budva-claude/internal/domain"
)

type fakeAuthDriver struct {
	mu        sync.Mutex
	states    []domain.AuthorizationState
	extras    []any
	inputChan chan string
}

func newFakeAuthDriver() *fakeAuthDriver {
	return &fakeAuthDriver{
		inputChan: make(chan string, 1),
	}
}

func (f *fakeAuthDriver) SetState(state domain.AuthorizationState, extra any) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.states = append(f.states, state)
	f.extras = append(f.extras, extra)
}

func (f *fakeAuthDriver) snapshot() ([]domain.AuthorizationState, []any) {
	f.mu.Lock()
	defer f.mu.Unlock()
	s := make([]domain.AuthorizationState, len(f.states))
	copy(s, f.states)
	e := make([]any, len(f.extras))
	copy(e, f.extras)
	return s, e
}

func (f *fakeAuthDriver) ReadChan() <-chan string {
	return f.inputChan
}

func TestRunAuthFlow_FullCycle(t *testing.T) {
	t.Parallel()

	synctest.Run(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		repo := New(config.TelegramConfig{})
		auth := newFakeAuthDriver()

		go repo.RunAuthFlow(ctx, auth)

		// WaitPhone
		time.Sleep(1 * time.Millisecond)
		states, _ := auth.snapshot()
		require.Len(t, states, 1)
		assert.Equal(t, domain.AuthStateWaitPhone, states[0])

		auth.inputChan <- "+79261234567"

		// WaitCode
		time.Sleep(1 * time.Millisecond)
		states, _ = auth.snapshot()
		require.Len(t, states, 2)
		assert.Equal(t, domain.AuthStateWaitCode, states[1])

		auth.inputChan <- "12345"

		// WaitPassword
		time.Sleep(1 * time.Millisecond)
		states, extras := auth.snapshot()
		require.Len(t, states, 3)
		assert.Equal(t, domain.AuthStateWaitPassword, states[2])
		ws, ok := extras[2].(*domain.WaitPasswordState)
		require.True(t, ok)
		assert.Equal(t, "2FA password", ws.PasswordHint)

		auth.inputChan <- "secret"

		// Ready
		time.Sleep(1 * time.Millisecond)
		states, _ = auth.snapshot()
		require.Len(t, states, 4)
		assert.Equal(t, domain.AuthStateReady, states[3])

		// ClientDone закрыт
		select {
		case <-repo.ClientDone():
			// OK
		default:
			t.Error("clientDone should be closed after Ready")
		}
	})
}

func TestRunAuthFlow_CancelDuringInput(t *testing.T) {
	t.Parallel()

	synctest.Run(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		repo := New(config.TelegramConfig{})
		auth := newFakeAuthDriver()

		done := make(chan struct{})
		go func() {
			repo.RunAuthFlow(ctx, auth)
			close(done)
		}()

		time.Sleep(1 * time.Millisecond)
		states, _ := auth.snapshot()
		require.Len(t, states, 1)

		cancel()

		select {
		case <-done:
			// OK — RunAuthFlow завершился
		case <-time.After(1 * time.Second):
			t.Error("RunAuthFlow did not exit after context cancel")
		}

		// clientDone НЕ закрыт
		select {
		case <-repo.ClientDone():
			t.Error("clientDone should NOT be closed after cancel")
		default:
			// OK
		}
	})
}
