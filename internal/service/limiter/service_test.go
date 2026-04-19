package limiter_test

import (
	"context"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pure-golang/budva-claude/internal/domain"
	"github.com/pure-golang/budva-claude/internal/service/limiter"
)

const interval = 3 * time.Second

func TestNew(t *testing.T) {
	t.Parallel()

	// Act
	svc := limiter.New()

	// Assert
	require.NotNil(t, svc, "Конструктор должен вернуть непустой сервис")
}

func TestWaitForForward_FirstCallNoWait(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		// Arrange
		svc := limiter.New()
		ctx := t.Context()
		chatID := domain.ChatID(123)
		start := time.Now()

		// Act
		svc.WaitForForward(ctx, chatID)

		// Assert
		assert.Equal(t, time.Duration(0), time.Since(start), "Первый вызов не должен блокироваться")
	})
}

func TestWaitForForward_SecondCallWaitsInterval(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		// Arrange
		svc := limiter.New()
		ctx := t.Context()
		chatID := domain.ChatID(123)
		start := time.Now()

		// Act
		svc.WaitForForward(ctx, chatID)
		svc.WaitForForward(ctx, chatID)

		// Assert
		assert.Equal(t, interval, time.Since(start), "Второй вызов должен ждать полный интервал")
	})
}

func TestWaitForForward_ThirdCallWaitsAnotherInterval(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		// Arrange
		svc := limiter.New()
		ctx := t.Context()
		chatID := domain.ChatID(42)
		start := time.Now()

		// Act
		svc.WaitForForward(ctx, chatID)
		svc.WaitForForward(ctx, chatID)
		svc.WaitForForward(ctx, chatID)

		// Assert
		assert.Equal(t, 2*interval, time.Since(start), "Каждый последующий вызов должен добавлять интервал")
	})
}

func TestWaitForForward_NoWaitAfterInterval(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		// Arrange
		svc := limiter.New()
		ctx := t.Context()
		chatID := domain.ChatID(7)
		svc.WaitForForward(ctx, chatID)

		// Act: ждём сверх интервала и вызываем снова
		time.Sleep(interval + time.Second)
		start := time.Now()
		svc.WaitForForward(ctx, chatID)

		// Assert
		assert.Equal(t, time.Duration(0), time.Since(start), "После истечения интервала ждать не нужно")
	})
}

func TestWaitForForward_WaitOnlyRemainingTime(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		// Arrange
		svc := limiter.New()
		ctx := t.Context()
		chatID := domain.ChatID(7)
		svc.WaitForForward(ctx, chatID)
		elapsed := time.Second
		time.Sleep(elapsed)
		start := time.Now()

		// Act
		svc.WaitForForward(ctx, chatID)

		// Assert
		assert.Equal(t, interval-elapsed, time.Since(start), "Ждать нужно только остаток интервала")
	})
}

func TestWaitForForward_DifferentChatsIndependent(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		// Arrange
		svc := limiter.New()
		ctx := t.Context()
		chatA := domain.ChatID(1)
		chatB := domain.ChatID(2)
		start := time.Now()

		// Act
		svc.WaitForForward(ctx, chatA)
		svc.WaitForForward(ctx, chatB)

		// Assert
		assert.Equal(t, time.Duration(0), time.Since(start), "Разные чаты не должны блокировать друг друга")
	})
}

func TestWaitForForward_ChatIDValues(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		chatID domain.ChatID
	}{
		{name: "zero", chatID: 0},
		{name: "positive", chatID: 1000},
		{name: "negative_channel_like", chatID: -1001234567890},
		{name: "max_int64", chatID: 1<<62 - 1},
		{name: "min_int64", chatID: -(1 << 62)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			synctest.Test(t, func(t *testing.T) {
				// Arrange
				svc := limiter.New()
				ctx := t.Context()
				start := time.Now()

				// Act
				svc.WaitForForward(ctx, tt.chatID)
				svc.WaitForForward(ctx, tt.chatID)

				// Assert
				assert.Equal(t, interval, time.Since(start), "Любой chatID должен корректно отслеживаться")
			})
		})
	}
}

func TestWaitForForward_ContextCancelledReturnsEarly(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		// Arrange
		svc := limiter.New()
		chatID := domain.ChatID(555)
		svc.WaitForForward(t.Context(), chatID)

		ctx, cancel := context.WithCancel(t.Context())
		done := make(chan struct{})
		start := time.Now()

		// Act
		go func() {
			svc.WaitForForward(ctx, chatID)
			close(done)
		}()

		// Синхронизируем планировщик перед отменой, чтобы goroutine встала в select
		time.Sleep(10 * time.Millisecond)
		cancel()
		<-done

		// Assert
		elapsed := time.Since(start)
		assert.Less(t, elapsed, interval, "При отмене контекста возврат должен быть до истечения интервала")
	})
}

func TestWaitForForward_ContextAlreadyCancelledStillProceeds(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		// Arrange: заранее отменённый контекст — первый вызов не попадает в select,
		// потому что diff == 0 и ветка ожидания не активируется
		svc := limiter.New()
		ctx, cancel := context.WithCancel(t.Context())
		cancel()
		chatID := domain.ChatID(9)
		start := time.Now()

		// Act
		svc.WaitForForward(ctx, chatID)

		// Assert
		assert.Equal(t, time.Duration(0), time.Since(start), "Отменённый контекст не должен влиять на первый вызов")
	})
}

func TestWaitForForward_ContextCancelledDoesNotUpdateTimestamp(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		// Arrange: первый вызов фиксирует timestamp; второй запускаем с отменяемым ctx
		svc := limiter.New()
		chatID := domain.ChatID(77)
		svc.WaitForForward(t.Context(), chatID)

		ctx, cancel := context.WithCancel(t.Context())
		done := make(chan struct{})

		// Act: запускаем блокирующий вызов и отменяем его
		go func() {
			svc.WaitForForward(ctx, chatID)
			close(done)
		}()
		time.Sleep(10 * time.Millisecond)
		cancel()
		<-done

		// Assert: после отмены timestamp не обновился, поэтому следующий вызов
		// должен дождаться остатка исходного интервала, а не нового полного интервала
		start := time.Now()
		svc.WaitForForward(t.Context(), chatID)
		assert.Less(t, time.Since(start), interval, "Отменённый вызов не должен обновлять timestamp")
	})
}

func TestWaitForForward_ConcurrentSameChatCompletes(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		// Arrange
		svc := limiter.New()
		ctx := t.Context()
		chatID := domain.ChatID(1)
		const n = 3
		var wg sync.WaitGroup
		wg.Add(n)
		start := time.Now()

		// Act
		for i := 0; i < n; i++ {
			go func() {
				defer wg.Done()
				svc.WaitForForward(ctx, chatID)
			}()
		}
		wg.Wait()

		// Assert: параллельные вызовы в один чат должны завершиться не раньше, чем через интервал
		// (точная сериализация не гарантируется текущей реализацией, см. service.go)
		elapsed := time.Since(start)
		assert.GreaterOrEqual(t, elapsed, interval, "Хотя бы один вызов должен дождаться интервала")
	})
}

func TestWaitForForward_SequentialSameChatSerialized(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		// Arrange
		svc := limiter.New()
		ctx := t.Context()
		chatID := domain.ChatID(1)
		const n = 4
		start := time.Now()

		// Act: последовательные вызовы серилизуются через интервал
		for i := 0; i < n; i++ {
			svc.WaitForForward(ctx, chatID)
		}

		// Assert
		assert.Equal(t, time.Duration(n-1)*interval, time.Since(start), "Последовательные вызовы должны суммировать интервалы")
	})
}

func TestWaitForForward_ConcurrentDifferentChatsParallel(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		// Arrange
		svc := limiter.New()
		ctx := t.Context()
		const n = 5
		var wg sync.WaitGroup
		wg.Add(n)
		start := time.Now()

		// Act
		for i := 0; i < n; i++ {
			chatID := domain.ChatID(i + 1)
			go func() {
				defer wg.Done()
				svc.WaitForForward(ctx, chatID)
			}()
		}
		wg.Wait()

		// Assert
		assert.Equal(t, time.Duration(0), time.Since(start), "Первые вызовы в разные чаты идут параллельно")
	})
}
