package dedup

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTracker_TryMark_first_time(t *testing.T) {
	t.Parallel()

	// Arrange
	tr := NewTracker([]int64{100, 200})

	// Act / Assert
	require.True(t, tr.TryMark(100))
	require.True(t, tr.TryMark(200))
}

func TestTracker_TryMark_duplicate(t *testing.T) {
	t.Parallel()

	// Arrange
	tr := NewTracker([]int64{100})

	// Act
	tr.TryMark(100)

	// Assert
	require.False(t, tr.TryMark(100))
}

func TestTracker_TryMark_unknown_chat(t *testing.T) {
	t.Parallel()

	// Arrange
	tr := NewTracker([]int64{100})

	// Act / Assert — неинициализированный чат возвращает true
	require.True(t, tr.TryMark(999))
}
