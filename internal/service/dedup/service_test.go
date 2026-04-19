package dedup_test

import (
	"sync"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pure-golang/budva-claude/internal/domain"
	"github.com/pure-golang/budva-claude/internal/service/dedup"
)

func TestNewTracker_empty_destinations(t *testing.T) {
	t.Parallel()

	// Arrange / Act
	tr := dedup.NewTracker(nil)

	// Assert — трекер создан и готов принимать произвольные чаты
	require.NotNil(t, tr)
	require.True(t, tr.TryMark(1))
}

func TestNewTracker_nil_slice(t *testing.T) {
	t.Parallel()

	// Arrange / Act
	tr := dedup.NewTracker([]domain.ChatID{})

	// Assert
	require.NotNil(t, tr)
	require.True(t, tr.TryMark(42))
	require.False(t, tr.TryMark(42))
}

func TestTracker_TryMark_first_time(t *testing.T) {
	t.Parallel()

	// Arrange
	tr := dedup.NewTracker([]domain.ChatID{100, 200})

	// Act / Assert
	require.True(t, tr.TryMark(100))
	require.True(t, tr.TryMark(200))
}

func TestTracker_TryMark_duplicate(t *testing.T) {
	t.Parallel()

	// Arrange
	tr := dedup.NewTracker([]domain.ChatID{100})

	// Act
	first := tr.TryMark(100)
	second := tr.TryMark(100)
	third := tr.TryMark(100)

	// Assert — только первый вызов возвращает true
	require.True(t, first)
	require.False(t, second)
	require.False(t, third)
}

func TestTracker_TryMark_unknown_chat(t *testing.T) {
	t.Parallel()

	// Arrange
	tr := dedup.NewTracker([]domain.ChatID{100})

	// Act / Assert — неинициализированный чат трактуется как «ещё не помечен»
	require.True(t, tr.TryMark(999))
	require.False(t, tr.TryMark(999))
}

func TestTracker_TryMark_independent_destinations(t *testing.T) {
	t.Parallel()

	// Arrange
	tr := dedup.NewTracker([]domain.ChatID{1, 2, 3})

	// Act
	a := tr.TryMark(1)
	b := tr.TryMark(2)
	c := tr.TryMark(3)
	aDup := tr.TryMark(1)

	// Assert — пометка одного чата не влияет на другие
	require.True(t, a)
	require.True(t, b)
	require.True(t, c)
	require.False(t, aDup)
}

func TestTracker_TryMark_table(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		destinations []domain.ChatID
		sequence     []domain.ChatID
		want         []bool
	}{
		{
			name:         "single_destination_marked_once",
			destinations: []domain.ChatID{100},
			sequence:     []domain.ChatID{100, 100},
			want:         []bool{true, false},
		},
		{
			name:         "multiple_destinations_interleaved",
			destinations: []domain.ChatID{1, 2},
			sequence:     []domain.ChatID{1, 2, 1, 2},
			want:         []bool{true, true, false, false},
		},
		{
			name:         "zero_chat_id",
			destinations: []domain.ChatID{0},
			sequence:     []domain.ChatID{0, 0},
			want:         []bool{true, false},
		},
		{
			name:         "negative_chat_id",
			destinations: []domain.ChatID{-1001},
			sequence:     []domain.ChatID{-1001, -1001},
			want:         []bool{true, false},
		},
		{
			name:         "unknown_chat_then_known",
			destinations: []domain.ChatID{10},
			sequence:     []domain.ChatID{999, 999, 10, 10},
			want:         []bool{true, false, true, false},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			tr := dedup.NewTracker(tt.destinations)

			// Act
			got := make([]bool, 0, len(tt.sequence))
			for _, id := range tt.sequence {
				got = append(got, tr.TryMark(id))
			}

			// Assert
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestTracker_TryMark_concurrent_same_chat(t *testing.T) {
	t.Parallel()

	// Arrange — один чат, много горутин
	tr := dedup.NewTracker([]domain.ChatID{42})
	const workers = 64
	var wg sync.WaitGroup
	wg.Add(workers)
	var successes int64

	// Act
	for i := 0; i < workers; i++ {
		go func() {
			defer wg.Done()
			if tr.TryMark(42) {
				atomic.AddInt64(&successes, 1)
			}
		}()
	}
	wg.Wait()

	// Assert — ровно одна горутина получила true
	require.Equal(t, int64(1), successes)
	require.False(t, tr.TryMark(42))
}

func TestTracker_TryMark_concurrent_different_chats(t *testing.T) {
	t.Parallel()

	// Arrange — набор чатов, каждый атакуется несколькими горутинами
	destinations := []domain.ChatID{1, 2, 3, 4, 5, 6, 7, 8}
	tr := dedup.NewTracker(destinations)

	const attemptsPerChat = 16
	var wg sync.WaitGroup
	successes := make([]int64, len(destinations))

	// Act
	for idx, chat := range destinations {
		for i := 0; i < attemptsPerChat; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				if tr.TryMark(chat) {
					atomic.AddInt64(&successes[idx], 1)
				}
			}()
		}
	}
	wg.Wait()

	// Assert — по каждому чату ровно один успех
	for idx, chat := range destinations {
		require.Equalf(t, int64(1), successes[idx], "chat %d: expected one success", chat)
	}
}

func TestTracker_independent_instances(t *testing.T) {
	t.Parallel()

	// Arrange — два трекера не разделяют состояние
	trA := dedup.NewTracker([]domain.ChatID{100})
	trB := dedup.NewTracker([]domain.ChatID{100})

	// Act
	okA := trA.TryMark(100)
	okB := trB.TryMark(100)
	dupA := trA.TryMark(100)

	// Assert
	require.True(t, okA)
	require.True(t, okB)
	require.False(t, dupA)
}
