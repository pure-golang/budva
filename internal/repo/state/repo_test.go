package state

import (
	"context"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/pure-golang/budva/internal/config"
)

func newTestRepo(t *testing.T) *Repo {
	t.Helper()
	cfg := config.StorageConfig{
		DatabaseDirectory: t.TempDir(),
	}
	r := New(cfg, slog.Default())
	err := r.Start(context.Background())
	require.NoError(t, err)
	t.Cleanup(func() { _ = r.Close() })
	return r
}

func TestRepo_SetGet(t *testing.T) {
	t.Parallel()

	// Arrange
	r := newTestRepo(t)

	// Act
	err := r.Set("key1", "value1")
	require.NoError(t, err)

	val, err := r.Get("key1")

	// Assert
	require.NoError(t, err)
	require.Equal(t, "value1", val)
}

func TestRepo_Get_not_found(t *testing.T) {
	t.Parallel()

	// Arrange
	r := newTestRepo(t)

	// Act
	_, err := r.Get("missing")

	// Assert
	require.True(t, IsKeyNotFound(err))
}

func TestRepo_Delete(t *testing.T) {
	t.Parallel()

	// Arrange
	r := newTestRepo(t)
	err := r.Set("key1", "value1")
	require.NoError(t, err)

	// Act
	err = r.Delete("key1")
	require.NoError(t, err)

	// Assert
	_, err = r.Get("key1")
	require.True(t, IsKeyNotFound(err))
}

func TestRepo_GetSet_atomic(t *testing.T) {
	t.Parallel()

	// Arrange
	r := newTestRepo(t)
	err := r.Set("counter", "0")
	require.NoError(t, err)

	// Act
	val, err := r.GetSet("counter", func(val string) (string, error) {
		return val + "1", nil
	})

	// Assert
	require.NoError(t, err)
	require.Equal(t, "01", val)
}
