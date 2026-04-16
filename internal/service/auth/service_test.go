package auth

import (
	"context"
	"errors"
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
	phoneErr   error
	codeErr    error
	passErr    error
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
	if f.phoneErr != nil {
		err := f.phoneErr
		f.phoneErr = nil
		return err
	}
	return nil
}

func (f *fakeTelegramRepo) SubmitCode(_ context.Context, code string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.codes = append(f.codes, code)
	if f.codeErr != nil {
		err := f.codeErr
		f.codeErr = nil
		return err
	}
	return nil
}

func (f *fakeTelegramRepo) SubmitPassword(_ context.Context, password string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.passwords = append(f.passwords, password)
	if f.passErr != nil {
		err := f.passErr
		f.passErr = nil
		return err
	}
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

		var mu sync.Mutex
		var received []domain.AuthorizationState
		svc.Subscribe(func(state domain.AuthorizationState, _ any) {
			mu.Lock()
			received = append(received, state)
			mu.Unlock()
		})

		svc.Start(t.Context())

		// Act — Ready завершает run(), поэтому отправляем его последним
		repo.authStates <- domain.AuthStateEvent{State: domain.AuthStateReady}
		time.Sleep(1 * time.Millisecond)

		// Assert
		mu.Lock()
		assert.Equal(t, []domain.AuthorizationState{domain.AuthStateReady}, received)
		mu.Unlock()
	})
}

func TestSubscribeReceivesExtra(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		// Arrange
		repo := newFakeTelegramRepo()
		svc := New(repo)

		var mu sync.Mutex
		var gotExtra any
		svc.Subscribe(func(_ domain.AuthorizationState, extra any) {
			mu.Lock()
			gotExtra = extra
			mu.Unlock()
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
		mu.Lock()
		require.NotNil(t, gotExtra)
		ws, ok := gotExtra.(*domain.WaitPasswordState)
		require.True(t, ok)
		mu.Unlock()
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
		var mu sync.Mutex
		var count1, count2 int
		svc.Subscribe(func(_ domain.AuthorizationState, _ any) {
			mu.Lock()
			count1++
			mu.Unlock()
		})
		svc.Subscribe(func(_ domain.AuthorizationState, _ any) {
			mu.Lock()
			count2++
			mu.Unlock()
		})

		svc.Start(t.Context())

		// Act
		repo.authStates <- domain.AuthStateEvent{State: domain.AuthStateReady}
		time.Sleep(1 * time.Millisecond)

		// Assert
		mu.Lock()
		assert.Equal(t, 1, count1)
		assert.Equal(t, 1, count2)
		mu.Unlock()
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

func TestClosingStateIsSkipped(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		// Arrange
		repo := newFakeTelegramRepo()
		svc := New(repo)

		var mu sync.Mutex
		var received []domain.AuthorizationState
		svc.Subscribe(func(state domain.AuthorizationState, _ any) {
			mu.Lock()
			received = append(received, state)
			mu.Unlock()
		})

		svc.Start(t.Context())

		// Act — Closing не должен попадать в listeners
		repo.authStates <- domain.AuthStateEvent{State: domain.AuthStateClosing}
		time.Sleep(1 * time.Millisecond)
		repo.authStates <- domain.AuthStateEvent{State: domain.AuthStateReady}
		time.Sleep(1 * time.Millisecond)

		// Assert — только Ready, без Closing
		mu.Lock()
		assert.Equal(t, []domain.AuthorizationState{domain.AuthStateReady}, received)
		mu.Unlock()
	})
}

func TestSubmitCodeRejection_WaitsForReEmit(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		// Arrange
		repo := newFakeTelegramRepo()
		repo.codeErr = errors.New("invalid code")
		svc := New(repo)
		svc.Start(t.Context())

		// Act — WaitCode, ввод отклонён repo
		repo.authStates <- domain.AuthStateEvent{State: domain.AuthStateWaitCode}
		time.Sleep(1 * time.Millisecond)

		svc.InputChan() <- "wrong"
		time.Sleep(1 * time.Millisecond)

		// Сервис вернулся в внешний цикл — ждёт новый event от repo.
		// Repo повторно эмитит WaitCode (как TDLib при rejection).
		repo.authStates <- domain.AuthStateEvent{State: domain.AuthStateWaitCode}
		time.Sleep(1 * time.Millisecond)

		svc.InputChan() <- "correct"
		time.Sleep(1 * time.Millisecond)

		// Assert
		repo.mu.Lock()
		require.Len(t, repo.codes, 2)
		assert.Equal(t, "wrong", repo.codes[0])
		assert.Equal(t, "correct", repo.codes[1])
		repo.mu.Unlock()
	})
}

func TestFlowWithout2FA(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		// Arrange
		repo := newFakeTelegramRepo()
		svc := New(repo)
		svc.Start(t.Context())

		// WaitPhone
		repo.authStates <- domain.AuthStateEvent{State: domain.AuthStateWaitPhone}
		time.Sleep(1 * time.Millisecond)
		svc.InputChan() <- "+79261234567"
		time.Sleep(1 * time.Millisecond)

		// WaitCode
		repo.authStates <- domain.AuthStateEvent{State: domain.AuthStateWaitCode}
		time.Sleep(1 * time.Millisecond)
		svc.InputChan() <- "12345"
		time.Sleep(1 * time.Millisecond)

		// Ready — без WaitPassword
		repo.authStates <- domain.AuthStateEvent{State: domain.AuthStateReady}
		time.Sleep(1 * time.Millisecond)

		// Assert
		assert.Equal(t, domain.AuthStateReady, svc.State())

		repo.mu.Lock()
		assert.Len(t, repo.phones, 1)
		assert.Len(t, repo.codes, 1)
		assert.Empty(t, repo.passwords)
		repo.mu.Unlock()
	})
}

func TestClose_ClosesInputChan(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := newFakeTelegramRepo()
	svc := New(repo)

	// Act
	err := svc.Close()

	// Assert
	require.NoError(t, err)
	assert.Panics(t, func() {
		svc.InputChan() <- "should panic"
	}, "writing to closed inputChan should panic")
}
