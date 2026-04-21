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

func TestRepo_StartContext_stops_on_cancel(t *testing.T) {
	t.Parallel()

	// Arrange
	r := New()
	ctx, cancel := context.WithCancel(context.Background())

	// Act
	err := r.StartContext(ctx)
	require.NoError(t, err)
	cancel()

	// Assert — отмена контекста не вызывает паники; очередь пуста
	require.Equal(t, 0, r.Len())
}

func TestRepo_Close(t *testing.T) {
	t.Parallel()

	// Arrange
	r := New()

	// Act / Assert — всегда возвращает nil
	require.NoError(t, r.Close())
}

func TestRepo_ProcessAll(t *testing.T) {
	t.Parallel()

	// Arrange
	r := New()
	var counter atomic.Int32
	r.Add(func() { counter.Add(1) })
	r.Add(func() { counter.Add(1) })
	r.Add(func() { counter.Add(1) })

	// Act
	r.ProcessAll()

	// Assert
	require.Equal(t, int32(3), counter.Load())
	require.Equal(t, 0, r.Len())
}

func TestRepo_ProcessAll_executes_tasks_added_during_processing(t *testing.T) {
	t.Parallel()

	// Arrange
	r := New()
	var counter atomic.Int32
	r.Add(func() {
		counter.Add(1)
		if counter.Load() == 1 {
			r.Add(func() { counter.Add(10) })
		}
	})

	// Act
	r.ProcessAll()

	// Assert — задача добавленная во время выполнения тоже выполнена
	require.Equal(t, int32(11), counter.Load())
	require.Equal(t, 0, r.Len())
}

func TestRepo_ProcessBatch(t *testing.T) {
	t.Parallel()

	// Arrange
	r := New()
	var counter atomic.Int32
	r.Add(func() { counter.Add(1) })
	r.Add(func() { counter.Add(1) })

	// Act
	r.ProcessBatch()

	// Assert
	require.Equal(t, int32(2), counter.Load())
	require.Equal(t, 0, r.Len())
}

func TestRepo_ProcessBatch_ignores_tasks_added_during_processing(t *testing.T) {
	t.Parallel()

	// Arrange
	r := New()
	var counter atomic.Int32
	r.Add(func() {
		counter.Add(1)
		r.Add(func() { counter.Add(10) })
	})

	// Act
	r.ProcessBatch()

	// Assert — задача добавленная во время выполнения НЕ выполнена
	require.Equal(t, int32(1), counter.Load())
	require.Equal(t, 1, r.Len())
}

func TestRepo_ProcessQueue_empty(t *testing.T) {
	t.Parallel()

	// Arrange
	r := New()

	// Act / Assert — нет паники на пустой очереди
	r.processQueue()
	require.Equal(t, 0, r.Len())
}

func TestRepo_ProcessAll_empty_queue(t *testing.T) {
	t.Parallel()

	// Arrange
	r := New()

	// Act / Assert — нет паники на пустой очереди
	r.ProcessAll()
	require.Equal(t, 0, r.Len())
}

func TestRepo_ProcessBatch_empty_queue(t *testing.T) {
	t.Parallel()

	// Arrange
	r := New()

	// Act / Assert — нет паники на пустой очереди
	r.ProcessBatch()
	require.Equal(t, 0, r.Len())
}

func TestRepo_ProcessBatch_NilFront(t *testing.T) {
	t.Parallel()

	// Arrange — первая задача крадёт вторую; итерация 2 получит nil и выходит
	r := New()
	var counter atomic.Int32
	r.Add(func() {
		counter.Add(1)
		r.mu.Lock()
		if front := r.queue.Front(); front != nil {
			r.queue.Remove(front)
		}
		r.mu.Unlock()
	})
	r.Add(func() { counter.Add(10) })

	// Act
	r.ProcessBatch()

	// Assert — вторая задача украдена изнутри первой, guard сработал
	require.Equal(t, int32(1), counter.Load())
	require.Equal(t, 0, r.Len())
}
