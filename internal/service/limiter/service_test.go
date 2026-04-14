package limiter

import (
	"testing"
	"testing/synctest"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestWaitForForward(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		ctx := t.Context()
		limiter := New()
		chatID := int64(123)

		start := time.Now()

		limiter.WaitForForward(ctx, chatID)
		elapsed := time.Since(start)
		assert.Equal(t, 0*time.Second, elapsed, "Первый вызов не должен ждать")

		limiter.WaitForForward(ctx, chatID)
		elapsed = time.Since(start)
		assert.Equal(t, 3*time.Second, elapsed, "Второй вызов должен ждать 3 секунды")
	})
}
