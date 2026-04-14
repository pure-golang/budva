package state

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSetCopiedMessageID_SingleDestination(t *testing.T) {
	t.Parallel()

	// Arrange
	r := newTestRepo(t)

	// Act
	err := r.SetCopiedMessageID(-100, 1, "rule1:-200:500")

	// Assert
	require.NoError(t, err)
	assert.Equal(t, []string{"rule1:-200:500"}, r.GetCopiedMessageIDs(-100, 1))
}

func TestSetCopiedMessageID_MultipleDestinations(t *testing.T) {
	t.Parallel()

	// Arrange
	r := newTestRepo(t)

	// Act
	require.NoError(t, r.SetCopiedMessageID(-100, 1, "rule1:-200:500"))
	require.NoError(t, r.SetCopiedMessageID(-100, 1, "rule1:-300:600"))

	// Assert
	copies := r.GetCopiedMessageIDs(-100, 1)
	assert.Len(t, copies, 2)
	assert.Contains(t, copies, "rule1:-200:500")
	assert.Contains(t, copies, "rule1:-300:600")
}

func TestSetCopiedMessageID_UpdateInPlace(t *testing.T) {
	t.Parallel()

	// Arrange
	r := newTestRepo(t)
	require.NoError(t, r.SetCopiedMessageID(-100, 1, "rule1:-200:500"))

	// Act
	require.NoError(t, r.SetCopiedMessageID(-100, 1, "rule1:-200:700"))

	// Assert
	assert.Equal(t, []string{"rule1:-200:700"}, r.GetCopiedMessageIDs(-100, 1))
}

func TestDeleteCopiedMessageIDs(t *testing.T) {
	t.Parallel()

	// Arrange
	r := newTestRepo(t)
	require.NoError(t, r.SetCopiedMessageID(-100, 1, "rule1:-200:500"))

	// Act
	require.NoError(t, r.DeleteCopiedMessageIDs(-100, 1))

	// Assert
	assert.Nil(t, r.GetCopiedMessageIDs(-100, 1))
}

func TestNewMessageID_Bidirectional(t *testing.T) {
	t.Parallel()

	// Arrange
	r := newTestRepo(t)

	// Act
	require.NoError(t, r.SetNewMessageID(-200, 500, 600))
	require.NoError(t, r.SetTmpMessageID(-200, 600, 500))

	// Assert
	assert.Equal(t, int64(600), r.GetNewMessageID(-200, 500))
	assert.Equal(t, int64(500), r.GetTmpMessageID(-200, 600))
}

func TestIncrementCounters(t *testing.T) {
	t.Parallel()

	// Arrange
	r := newTestRepo(t)

	// Act
	v1, err1 := r.IncrementViewedMessages(-200, "2026-04-14")
	v2, err2 := r.IncrementViewedMessages(-200, "2026-04-14")
	f1, err3 := r.IncrementForwardedMessages(-200, "2026-04-14")

	// Assert
	require.NoError(t, err1)
	require.NoError(t, err2)
	require.NoError(t, err3)
	assert.Equal(t, uint64(1), v1)
	assert.Equal(t, uint64(2), v2)
	assert.Equal(t, uint64(1), f1)
}

func TestAnswerMessageID(t *testing.T) {
	t.Parallel()

	// Arrange
	r := newTestRepo(t)

	// Act
	require.NoError(t, r.SetAnswerMessageID(-200, 500, -100, 1))

	// Assert
	assert.Equal(t, "-100:1", r.GetAnswerMessageID(-200, 500))

	// Cleanup
	require.NoError(t, r.DeleteAnswerMessageID(-200, 500))
	assert.Empty(t, r.GetAnswerMessageID(-200, 500))
}
