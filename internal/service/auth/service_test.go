package auth

import (
	"context"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pure-golang/budva-claude/internal/domain"
)

type fakeTelegramRepo struct {
	mu         sync.Mutex
	authStates chan domain.AuthStateEvent
	phones     []string
	codes      []string
	passwords  []string
}

func newFakeTelegramRepo() *fakeTelegramRepo {
	return &fakeTelegramRepo{
		authStates: make(chan domain.AuthStateEvent, 10),
	}
}

func (f *fakeTelegramRepo) AuthStates() <-chan domain.AuthStateEvent {
	return f.authStates
}

func (f *fakeTelegramRepo) SubmitPhone(_ context.Context, phone string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.phones = append(f.phones, phone)
	return nil
}

func (f *fakeTelegramRepo) SubmitCode(_ context.Context, code string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.codes = append(f.codes, code)
	return nil
}

func (f *fakeTelegramRepo) SubmitPassword(_ context.Context, password string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.passwords = append(f.passwords, password)
	return nil
}

func TestNew(t *testing.T) {
	t.Parallel()

	// Act
	repo := newFakeTelegramRepo()
	svc := New(repo)

	// Assert
	assert.Equal(t, domain.AuthorizationState(0), svc.State())
	assert.NotNil(t, svc.InputChan())
}

func TestStateUpdatedFromEvent(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		// Arrange
		repo := newFakeTelegramRepo()
		svc := New(repo)
		svc.Start(t.Context())

		// Act
		repo.authStates <- domain.AuthStateEvent{State: domain.AuthStateReady}
		time.Sleep(1 * time.Millisecond)

		// Assert
		assert.Equal(t, domain.AuthStateReady, svc.State())
	})
}

func TestSubscribeReceivesStateChanges(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		// Arrange
		repo := newFakeTelegramRepo()
		svc := New(repo)

		var received []domain.AuthorizationState
		svc.Subscribe(func(state domain.AuthorizationState, _ any) {
			received = append(received, state)
		})

		svc.Start(t.Context())

		// Act — Ready завершает run(), поэтому отправляем его последним
		repo.authStates <- domain.AuthStateEvent{State: domain.AuthStateReady}
		time.Sleep(1 * time.Millisecond)

		// Assert
		assert.Equal(t, []domain.AuthorizationState{domain.AuthStateReady}, received)
	})
}

func TestSubscribeReceivesExtra(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		// Arrange
		repo := newFakeTelegramRepo()
		svc := New(repo)

		var gotExtra any
		svc.Subscribe(func(_ domain.AuthorizationState, extra any) {
			gotExtra = extra
		})

		hint := &domain.WaitPasswordState{PasswordHint: "pet name"}

		svc.Start(t.Context())

		// Act
		repo.authStates <- domain.AuthStateEvent{
			State: domain.AuthStateWaitPassword,
			Extra: hint,
		}
		time.Sleep(1 * time.Millisecond)

		// Отправляем input, чтобы run() продолжился
		svc.InputChan() <- "secret"
		time.Sleep(1 * time.Millisecond)

		// Assert
		require.NotNil(t, gotExtra)
		ws, ok := gotExtra.(*domain.WaitPasswordState)
		require.True(t, ok)
		assert.Equal(t, "pet name", ws.PasswordHint)
		assert.Equal(t, domain.AuthStateWaitPassword, svc.State())
	})
}

func TestFullAuthFlow(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		// Arrange
		repo := newFakeTelegramRepo()
		svc := New(repo)
		svc.Start(t.Context())

		// WaitPhone
		repo.authStates <- domain.AuthStateEvent{State: domain.AuthStateWaitPhone}
		time.Sleep(1 * time.Millisecond)
		assert.Equal(t, domain.AuthStateWaitPhone, svc.State())

		svc.InputChan() <- "+79261234567"
		time.Sleep(1 * time.Millisecond)

		repo.mu.Lock()
		require.Len(t, repo.phones, 1)
		assert.Equal(t, "+79261234567", repo.phones[0])
		repo.mu.Unlock()

		// WaitCode
		repo.authStates <- domain.AuthStateEvent{State: domain.AuthStateWaitCode}
		time.Sleep(1 * time.Millisecond)

		svc.InputChan() <- "12345"
		time.Sleep(1 * time.Millisecond)

		repo.mu.Lock()
		require.Len(t, repo.codes, 1)
		assert.Equal(t, "12345", repo.codes[0])
		repo.mu.Unlock()

		// WaitPassword
		repo.authStates <- domain.AuthStateEvent{
			State: domain.AuthStateWaitPassword,
			Extra: &domain.WaitPasswordState{PasswordHint: "2FA"},
		}
		time.Sleep(1 * time.Millisecond)

		svc.InputChan() <- "secret"
		time.Sleep(1 * time.Millisecond)

		repo.mu.Lock()
		require.Len(t, repo.passwords, 1)
		assert.Equal(t, "secret", repo.passwords[0])
		repo.mu.Unlock()

		// Ready
		repo.authStates <- domain.AuthStateEvent{State: domain.AuthStateReady}
		time.Sleep(1 * time.Millisecond)
		assert.Equal(t, domain.AuthStateReady, svc.State())
	})
}

func TestMultipleSubscribers(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		// Arrange
		repo := newFakeTelegramRepo()
		svc := New(repo)
		var count1, count2 int
		svc.Subscribe(func(_ domain.AuthorizationState, _ any) { count1++ })
		svc.Subscribe(func(_ domain.AuthorizationState, _ any) { count2++ })

		svc.Start(t.Context())

		// Act
		repo.authStates <- domain.AuthStateEvent{State: domain.AuthStateReady}
		time.Sleep(1 * time.Millisecond)

		// Assert
		assert.Equal(t, 1, count1)
		assert.Equal(t, 1, count2)
	})
}

func TestCancelDuringWait(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		// Arrange
		ctx, cancel := context.WithCancel(t.Context())
		repo := newFakeTelegramRepo()
		svc := New(repo)
		svc.Start(ctx)

		// Act — отправляем состояние, но не даём input
		repo.authStates <- domain.AuthStateEvent{State: domain.AuthStateWaitPhone}
		time.Sleep(1 * time.Millisecond)
		cancel()
		time.Sleep(1 * time.Millisecond)

		// Assert — сервис остановился, состояние зафиксировано
		assert.Equal(t, domain.AuthStateWaitPhone, svc.State())
	})
}
