package queue

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestRepo_Add_and_Len(t *testing.T) {
	t.Parallel()

	// Arrange
	r := New()

	// Act
	r.Add(func() {})
	r.Add(func() {})

	// Assert
	require.Equal(t, 2, r.Len())
}

func TestRepo_ProcessQueue_executes_task(t *testing.T) {
	t.Parallel()

	// Arrange
	r := New()
	var called atomic.Bool
	r.Add(func() { called.Store(true) })

	// Act
	r.processQueue()

	// Assert
	require.True(t, called.Load())
	require.Equal(t, 0, r.Len())
}

func TestRepo_ProcessQueue_recovers_from_panic(t *testing.T) {
	t.Parallel()

	// Arrange
	r := New()
	r.Add(func() { panic("test panic") })

	// Act / Assert — не паникует
	r.processQueue()
	require.Equal(t, 0, r.Len())
}

func TestRepo_StartContext_processes_tasks(t *testing.T) {
	t.Parallel()

	// Arrange
	r := New()
	var called atomic.Bool
	r.Add(func() { called.Store(true) })

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	// Act
	err := r.StartContext(ctx)
	require.NoError(t, err)

	// Assert — дождаться выполнения
	require.Eventually(t, called.Load, 3*time.Second, 100*time.Millisecond)
}
