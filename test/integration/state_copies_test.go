package integration

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pure-golang/budva-claude/internal/config"
	"github.com/pure-golang/budva-claude/internal/repo/state"
)

func newStateRepo(t *testing.T) *state.Repo {
	t.Helper()
	repo := state.New(config.StorageConfig{DatabaseDirectory: t.TempDir()})
	require.NoError(t, repo.Start(context.Background()))
	t.Cleanup(func() { repo.Close() })
	return repo
}

func TestSetCopiedMessageID_SingleDestination(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := newStateRepo(t)

	// Act
	err := repo.SetCopiedMessageID(-100, 1, "rule1:-200:500")

	// Assert
	require.NoError(t, err)
	copies := repo.GetCopiedMessageIDs(-100, 1)
	assert.Equal(t, []string{"rule1:-200:500"}, copies)
}

func TestSetCopiedMessageID_MultipleDestinations(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := newStateRepo(t)

	// Act
	require.NoError(t, repo.SetCopiedMessageID(-100, 1, "rule1:-200:500"))
	require.NoError(t, repo.SetCopiedMessageID(-100, 1, "rule1:-300:600"))

	// Assert
	copies := repo.GetCopiedMessageIDs(-100, 1)
	assert.Len(t, copies, 2)
	assert.Contains(t, copies, "rule1:-200:500")
	assert.Contains(t, copies, "rule1:-300:600")
}

func TestSetCopiedMessageID_UpdateInPlace(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := newStateRepo(t)
	require.NoError(t, repo.SetCopiedMessageID(-100, 1, "rule1:-200:500"))

	// Act — обновляем тот же destination с новым tmpMsgID
	require.NoError(t, repo.SetCopiedMessageID(-100, 1, "rule1:-200:700"))

	// Assert — должна быть одна запись с обновлённым ID
	copies := repo.GetCopiedMessageIDs(-100, 1)
	assert.Equal(t, []string{"rule1:-200:700"}, copies)
}

func TestDeleteCopiedMessageIDs(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := newStateRepo(t)
	require.NoError(t, repo.SetCopiedMessageID(-100, 1, "rule1:-200:500"))

	// Act
	require.NoError(t, repo.DeleteCopiedMessageIDs(-100, 1))

	// Assert
	copies := repo.GetCopiedMessageIDs(-100, 1)
	assert.Nil(t, copies)
}

func TestNewMessageID_Bidirectional(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := newStateRepo(t)

	// Act
	require.NoError(t, repo.SetNewMessageID(-200, 500, 600))
	require.NoError(t, repo.SetTmpMessageID(-200, 600, 500))

	// Assert
	assert.Equal(t, int64(600), repo.GetNewMessageID(-200, 500))
	assert.Equal(t, int64(500), repo.GetTmpMessageID(-200, 600))
}

func TestIncrementCounters(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := newStateRepo(t)

	// Act
	v1, err1 := repo.IncrementViewedMessages(-200, "2026-04-14")
	v2, err2 := repo.IncrementViewedMessages(-200, "2026-04-14")
	f1, err3 := repo.IncrementForwardedMessages(-200, "2026-04-14")

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
	repo := newStateRepo(t)

	// Act
	require.NoError(t, repo.SetAnswerMessageID(-200, 500, -100, 1))

	// Assert
	answer := repo.GetAnswerMessageID(-200, 500)
	assert.Equal(t, "-100:1", answer)

	// Cleanup
	require.NoError(t, repo.DeleteAnswerMessageID(-200, 500))
	assert.Empty(t, repo.GetAnswerMessageID(-200, 500))
}
