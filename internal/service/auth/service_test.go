package auth_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"testing/synctest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/zelenin/go-tdlib/client"

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
		synctest.Wait()

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
		synctest.Wait()

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
		synctest.Wait()

		// Отправляем input, чтобы run() продолжился
		svc.InputChan() <- "secret"
		synctest.Wait()

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
		synctest.Wait()
		assert.Equal(t, domain.AuthStateWaitPhone, svc.State())

		svc.InputChan() <- "+79261234567"
		synctest.Wait()

		// WaitCode
		states <- domain.AuthStateEvent{State: domain.AuthStateWaitCode}
		synctest.Wait()

		svc.InputChan() <- "12345"
		synctest.Wait()

		// WaitPassword
		states <- domain.AuthStateEvent{
			State: domain.AuthStateWaitPassword,
			Extra: &domain.WaitPasswordState{PasswordHint: "2FA"},
		}
		synctest.Wait()

		svc.InputChan() <- "secret"
		synctest.Wait()

		// Ready
		states <- domain.AuthStateEvent{State: domain.AuthStateReady}
		synctest.Wait()
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
		synctest.Wait()

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
		synctest.Wait()
		cancel()
		synctest.Wait()

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
		synctest.Wait()
		states <- domain.AuthStateEvent{State: domain.AuthStateReady}
		synctest.Wait()

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
		synctest.Wait()

		svc.InputChan() <- "wrong"
		synctest.Wait()

		// Сервис вернулся в внешний цикл — ждёт новый event от repo.
		// Repo повторно эмитит WaitCode (как TDLib при rejection).
		states <- domain.AuthStateEvent{State: domain.AuthStateWaitCode}
		synctest.Wait()

		svc.InputChan() <- "correct"
		synctest.Wait()

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
		synctest.Wait()
		svc.InputChan() <- "+79261234567"
		synctest.Wait()

		// WaitCode
		states <- domain.AuthStateEvent{State: domain.AuthStateWaitCode}
		synctest.Wait()
		svc.InputChan() <- "12345"
		synctest.Wait()

		// Ready — без WaitPassword
		states <- domain.AuthStateEvent{State: domain.AuthStateReady}
		synctest.Wait()

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

func TestExtra_InitiallyNil(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := mocks.NewTelegramRepo(t)
	svc := auth.New(repo)

	// Act
	extra := svc.Extra()

	// Assert
	assert.Nil(t, extra)
}

func TestExtra_ReturnsLastStateExtra(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		// Arrange
		repo := mocks.NewTelegramRepo(t)
		states := make(chan domain.AuthStateEvent, 10)
		repo.EXPECT().AuthStates().Return(states)
		svc := auth.New(repo)
		svc.Start(t.Context())

		hint := &domain.WaitPasswordState{PasswordHint: "mother's name"}

		// Act
		states <- domain.AuthStateEvent{
			State: domain.AuthStateWaitPassword,
			Extra: hint,
		}
		synctest.Wait()

		// Assert
		got := svc.Extra()
		require.NotNil(t, got)
		ws, ok := got.(*domain.WaitPasswordState)
		require.True(t, ok)
		assert.Equal(t, "mother's name", ws.PasswordHint)
	})
}

func TestLogOut_Success(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := mocks.NewTelegramRepo(t)
	repo.EXPECT().LogOut().Return(&client.Ok{}, nil)
	repo.EXPECT().CleanUp().Return()
	svc := auth.New(repo)

	// Act
	err := svc.LogOut(t.Context())

	// Assert
	require.NoError(t, err)
}

func TestLogOut_RepoError_SkipsCleanUp(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := mocks.NewTelegramRepo(t)
	logoutErr := errors.New("logout failed")
	repo.EXPECT().LogOut().Return(nil, logoutErr)
	svc := auth.New(repo)

	// Act
	err := svc.LogOut(t.Context())

	// Assert — ошибка пробрасывается, CleanUp не вызывается (AssertExpectations)
	require.ErrorIs(t, err, logoutErr)
}

func TestClosedStateIsSkipped(t *testing.T) {
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

		// Act — Closed не должен попадать в listeners
		states <- domain.AuthStateEvent{State: domain.AuthStateClosed}
		synctest.Wait()
		states <- domain.AuthStateEvent{State: domain.AuthStateReady}
		synctest.Wait()

		// Assert — только Ready
		mu.Lock()
		assert.Equal(t, []domain.AuthorizationState{domain.AuthStateReady}, received)
		mu.Unlock()
	})
}

func TestCancelBeforeEvent(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		// Arrange
		ctx, cancel := context.WithCancel(t.Context())
		repo := mocks.NewTelegramRepo(t)
		states := make(chan domain.AuthStateEvent, 10)
		repo.EXPECT().AuthStates().Return(states)
		svc := auth.New(repo)
		svc.Start(ctx)

		// Act — отменяем до отправки событий, run() должен выйти через <-ctx.Done()
		cancel()
		synctest.Wait()

		// Assert — состояние осталось zero-value
		assert.Equal(t, domain.AuthStateWaitPhone, svc.State())
	})
}

func TestSubmitPhoneError_Logged(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		// Arrange
		repo := mocks.NewTelegramRepo(t)
		states := make(chan domain.AuthStateEvent, 10)
		repo.EXPECT().AuthStates().Return(states)
		repo.EXPECT().SubmitPhone(mock.Anything, "+79261234567").Return(errors.New("invalid phone"))
		svc := auth.New(repo)
		svc.Start(t.Context())

		// Act
		states <- domain.AuthStateEvent{State: domain.AuthStateWaitPhone}
		synctest.Wait()
		svc.InputChan() <- "+79261234567"
		synctest.Wait()

		// Assert — ошибка залогирована, сервис жив; проверка через AssertExpectations
		assert.Equal(t, domain.AuthStateWaitPhone, svc.State())
	})
}

func TestSubmitPasswordError_Logged(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		// Arrange
		repo := mocks.NewTelegramRepo(t)
		states := make(chan domain.AuthStateEvent, 10)
		repo.EXPECT().AuthStates().Return(states)
		repo.EXPECT().SubmitPassword(mock.Anything, "bad").Return(errors.New("wrong password"))
		svc := auth.New(repo)
		svc.Start(t.Context())

		// Act
		states <- domain.AuthStateEvent{State: domain.AuthStateWaitPassword}
		synctest.Wait()
		svc.InputChan() <- "bad"
		synctest.Wait()

		// Assert
		assert.Equal(t, domain.AuthStateWaitPassword, svc.State())
	})
}

func TestConcurrentSubscribe(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := mocks.NewTelegramRepo(t)
	svc := auth.New(repo)

	// Act — одновременная регистрация подписчиков (без запуска run)
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			svc.Subscribe(func(_ domain.AuthorizationState, _ any) {})
		}()
	}
	wg.Wait()

	// Assert — при одновременной регистрации не должно быть race
	assert.Equal(t, domain.AuthorizationState(0), svc.State())
}

func TestConcurrentStateReadWrite(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		// Arrange — проверяем, что RLock/Lock корректно защищают state/extra
		repo := mocks.NewTelegramRepo(t)
		states := make(chan domain.AuthStateEvent, 10)
		repo.EXPECT().AuthStates().Return(states)
		svc := auth.New(repo)
		svc.Start(t.Context())

		// Act — одновременные чтения во время записи из run()
		var wg sync.WaitGroup
		for i := 0; i < 5; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				_ = svc.State()
				_ = svc.Extra()
			}()
		}
		states <- domain.AuthStateEvent{State: domain.AuthStateReady}
		wg.Wait()
		synctest.Wait()

		// Assert
		assert.Equal(t, domain.AuthStateReady, svc.State())
	})
}
