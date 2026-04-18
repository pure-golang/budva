package auth_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/pure-golang/budva-claude/internal/domain"
	"github.com/pure-golang/budva-claude/internal/service/auth"
	"github.com/pure-golang/budva-claude/internal/service/auth/mocks"
)

func TestNew(t *testing.T) {
	t.Parallel()

	// Act
	repo := mocks.NewTelegramRepo(t)
	svc := auth.New(repo)

	// Assert
	assert.Equal(t, domain.AuthorizationState(0), svc.State())
	assert.NotNil(t, svc.InputChan())
}

func TestStateUpdatedFromEvent(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		// Arrange
		repo := mocks.NewTelegramRepo(t)
		states := make(chan domain.AuthStateEvent, 10)
		repo.EXPECT().AuthStates().Return(states)
		svc := auth.New(repo)
		svc.Start(t.Context())

		// Act
		states <- domain.AuthStateEvent{State: domain.AuthStateReady}
		time.Sleep(1 * time.Millisecond)

		// Assert
		assert.Equal(t, domain.AuthStateReady, svc.State())
	})
}

func TestSubscribeReceivesStateChanges(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		// Arrange
		repo := mocks.NewTelegramRepo(t)
		states := make(chan domain.AuthStateEvent, 10)
		repo.EXPECT().AuthStates().Return(states)
		svc := auth.New(repo)

		var mu sync.Mutex
		var received []domain.AuthorizationState
		svc.Subscribe(func(state domain.AuthorizationState, _ any) {
			mu.Lock()
			received = append(received, state)
			mu.Unlock()
		})

		svc.Start(t.Context())

		// Act — Ready завершает run(), поэтому отправляем его последним
		states <- domain.AuthStateEvent{State: domain.AuthStateReady}
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
		repo := mocks.NewTelegramRepo(t)
		states := make(chan domain.AuthStateEvent, 10)
		repo.EXPECT().AuthStates().Return(states)
		repo.EXPECT().SubmitPassword(mock.Anything, "secret").Return(nil)
		svc := auth.New(repo)

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
		states <- domain.AuthStateEvent{
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
		repo := mocks.NewTelegramRepo(t)
		states := make(chan domain.AuthStateEvent, 10)
		repo.EXPECT().AuthStates().Return(states)
		repo.EXPECT().SubmitPhone(mock.Anything, "+79261234567").Return(nil)
		repo.EXPECT().SubmitCode(mock.Anything, "12345").Return(nil)
		repo.EXPECT().SubmitPassword(mock.Anything, "secret").Return(nil)
		svc := auth.New(repo)
		svc.Start(t.Context())

		// WaitPhone
		states <- domain.AuthStateEvent{State: domain.AuthStateWaitPhone}
		time.Sleep(1 * time.Millisecond)
		assert.Equal(t, domain.AuthStateWaitPhone, svc.State())

		svc.InputChan() <- "+79261234567"
		time.Sleep(1 * time.Millisecond)

		// WaitCode
		states <- domain.AuthStateEvent{State: domain.AuthStateWaitCode}
		time.Sleep(1 * time.Millisecond)

		svc.InputChan() <- "12345"
		time.Sleep(1 * time.Millisecond)

		// WaitPassword
		states <- domain.AuthStateEvent{
			State: domain.AuthStateWaitPassword,
			Extra: &domain.WaitPasswordState{PasswordHint: "2FA"},
		}
		time.Sleep(1 * time.Millisecond)

		svc.InputChan() <- "secret"
		time.Sleep(1 * time.Millisecond)

		// Ready
		states <- domain.AuthStateEvent{State: domain.AuthStateReady}
		time.Sleep(1 * time.Millisecond)
		assert.Equal(t, domain.AuthStateReady, svc.State())
	})
}

func TestMultipleSubscribers(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		// Arrange
		repo := mocks.NewTelegramRepo(t)
		states := make(chan domain.AuthStateEvent, 10)
		repo.EXPECT().AuthStates().Return(states)
		svc := auth.New(repo)
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
		states <- domain.AuthStateEvent{State: domain.AuthStateReady}
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
		repo := mocks.NewTelegramRepo(t)
		states := make(chan domain.AuthStateEvent, 10)
		repo.EXPECT().AuthStates().Return(states)
		svc := auth.New(repo)
		svc.Start(ctx)

		// Act — отправляем состояние, но не даём input
		states <- domain.AuthStateEvent{State: domain.AuthStateWaitPhone}
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
		repo := mocks.NewTelegramRepo(t)
		states := make(chan domain.AuthStateEvent, 10)
		repo.EXPECT().AuthStates().Return(states)
		svc := auth.New(repo)

		var mu sync.Mutex
		var received []domain.AuthorizationState
		svc.Subscribe(func(state domain.AuthorizationState, _ any) {
			mu.Lock()
			received = append(received, state)
			mu.Unlock()
		})

		svc.Start(t.Context())

		// Act — Closing не должен попадать в listeners
		states <- domain.AuthStateEvent{State: domain.AuthStateClosing}
		time.Sleep(1 * time.Millisecond)
		states <- domain.AuthStateEvent{State: domain.AuthStateReady}
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
		repo := mocks.NewTelegramRepo(t)
		states := make(chan domain.AuthStateEvent, 10)
		repo.EXPECT().AuthStates().Return(states)
		repo.EXPECT().SubmitCode(mock.Anything, "wrong").Return(errors.New("invalid code"))
		repo.EXPECT().SubmitCode(mock.Anything, "correct").Return(nil)
		svc := auth.New(repo)
		svc.Start(t.Context())

		// Act — WaitCode, ввод отклонён repo
		states <- domain.AuthStateEvent{State: domain.AuthStateWaitCode}
		time.Sleep(1 * time.Millisecond)

		svc.InputChan() <- "wrong"
		time.Sleep(1 * time.Millisecond)

		// Сервис вернулся в внешний цикл — ждёт новый event от repo.
		// Repo повторно эмитит WaitCode (как TDLib при rejection).
		states <- domain.AuthStateEvent{State: domain.AuthStateWaitCode}
		time.Sleep(1 * time.Millisecond)

		svc.InputChan() <- "correct"
		time.Sleep(1 * time.Millisecond)

		// Assert — оба вызова SubmitCode проверяются через AssertExpectations
	})
}

func TestFlowWithout2FA(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		// Arrange
		repo := mocks.NewTelegramRepo(t)
		states := make(chan domain.AuthStateEvent, 10)
		repo.EXPECT().AuthStates().Return(states)
		repo.EXPECT().SubmitPhone(mock.Anything, "+79261234567").Return(nil)
		repo.EXPECT().SubmitCode(mock.Anything, "12345").Return(nil)
		svc := auth.New(repo)
		svc.Start(t.Context())

		// WaitPhone
		states <- domain.AuthStateEvent{State: domain.AuthStateWaitPhone}
		time.Sleep(1 * time.Millisecond)
		svc.InputChan() <- "+79261234567"
		time.Sleep(1 * time.Millisecond)

		// WaitCode
		states <- domain.AuthStateEvent{State: domain.AuthStateWaitCode}
		time.Sleep(1 * time.Millisecond)
		svc.InputChan() <- "12345"
		time.Sleep(1 * time.Millisecond)

		// Ready — без WaitPassword
		states <- domain.AuthStateEvent{State: domain.AuthStateReady}
		time.Sleep(1 * time.Millisecond)

		// Assert
		assert.Equal(t, domain.AuthStateReady, svc.State())
	})
}

func TestClose_ClosesInputChan(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := mocks.NewTelegramRepo(t)
	svc := auth.New(repo)

	// Act
	err := svc.Close()

	// Assert
	require.NoError(t, err)
	assert.Panics(t, func() {
		svc.InputChan() <- "should panic"
	}, "writing to closed inputChan should panic")
}
