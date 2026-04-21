package state

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"testing"
	"testing/synctest"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pure-golang/budva-claude/internal/config"
)

func newTestRepo(t *testing.T) *Repo {
	t.Helper()
	cfg := config.StorageConfig{
		DatabaseDirectory: t.TempDir(),
	}
	r := New(cfg)
	err := r.Start(context.Background())
	require.NoError(t, err)
	t.Cleanup(func() {
		if err := r.Close(); err != nil {
			t.Errorf("failed to close state repo: %v", err)
		}
	})
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

func TestRepo_GetSet_fn_error(t *testing.T) {
	t.Parallel()

	// Arrange
	r := newTestRepo(t)
	wantErr := errors.New("transform error")

	// Act
	_, err := r.GetSet("key", func(_ string) (string, error) {
		return "", wantErr
	})

	// Assert
	require.ErrorIs(t, err, wantErr)
}

func TestRepo_Ping(t *testing.T) {
	t.Parallel()

	// Arrange
	r := newTestRepo(t)

	// Act
	err := r.Ping(context.Background())

	// Assert
	require.NoError(t, err)
}

func TestIsKeyNotFound_sentinel(t *testing.T) {
	t.Parallel()

	// Arrange / Act / Assert
	assert.True(t, IsKeyNotFound(ErrKeyNotFound))
	assert.False(t, IsKeyNotFound(errors.New("other error")))
	assert.False(t, IsKeyNotFound(nil))
}

func TestRepo_Close_NilStop(t *testing.T) {
	t.Parallel()

	// Arrange — repo никогда не стартовал, stop == nil
	r := New(config.StorageConfig{DatabaseDirectory: t.TempDir()})

	// Act / Assert — не паникует
	require.NoError(t, r.Close())
}

func TestBytesToUint64_short_slice(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		b    []byte
		want uint64
	}{
		{name: "nil", b: nil, want: 0},
		{name: "empty", b: []byte{}, want: 0},
		{name: "short_4_bytes", b: []byte{1, 2, 3, 4}, want: 0},
		{name: "short_7_bytes", b: []byte{0, 0, 0, 0, 0, 0, 1}, want: 0},
		{name: "exact_8_bytes", b: []byte{0, 0, 0, 0, 0, 0, 0, 1}, want: 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Act
			got := bytesToUint64(tt.b)

			// Assert
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestBadgerLogger_methods(t *testing.T) {
	t.Parallel()

	// Arrange
	l := newBadgerLogger(slog.Default())

	// Act / Assert — не паникуют
	l.Errorf("error %s %d", "msg", 1)
	l.Warningf("warn %s", "msg")
	l.Infof("info %s", "msg")
	l.Debugf("debug %s", "msg")
}

func TestRepo_Start_FileAsDirectory(t *testing.T) {
	t.Parallel()

	// Arrange — передаём путь к файлу вместо директории
	dir := t.TempDir()
	filePath := dir + "/db.txt"
	require.NoError(t, os.WriteFile(filePath, []byte("data"), 0o600))
	r := New(config.StorageConfig{DatabaseDirectory: filePath})

	// Act
	err := r.Start(context.Background())

	// Assert
	require.Error(t, err)
}

func TestRepo_runGC_ticker(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		// Arrange
		r := New(config.StorageConfig{DatabaseDirectory: t.TempDir()})
		require.NoError(t, r.Start(context.Background()))

		synctest.Wait() // runGC горутина устанавливается в select

		// Act — продвигаем фиктивное время за отметку тикера
		time.Sleep(5*time.Minute + time.Second)
		synctest.Wait() // runGC обрабатывает тик (ErrNoRewrite → break)

		// Cleanup — останавливаем GC горутину
		close(r.stop)
		r.stop = nil
		_ = r.db.Close()
		r.db = nil
		synctest.Wait()
	})
}

func TestRepo_Ping_DBError(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		// Arrange
		r := New(config.StorageConfig{DatabaseDirectory: t.TempDir()})
		require.NoError(t, r.Start(context.Background()))
		synctest.Wait()

		// Закрываем БД напрямую, чтобы Get вернул ошибку (не ErrKeyNotFound)
		require.NoError(t, r.db.Close())

		// Act
		err := r.Ping(context.Background())

		// Assert
		require.Error(t, err)

		// Cleanup
		close(r.stop)
		r.stop = nil
		r.db = nil
		synctest.Wait()
	})
}

func TestRepo_runGC_GCError(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		// Arrange
		r := New(config.StorageConfig{DatabaseDirectory: t.TempDir()})
		require.NoError(t, r.Start(context.Background()))
		synctest.Wait()

		// Закрываем БД до тика — RunValueLogGC вернёт реальную ошибку
		_ = r.db.Close()

		// Act — продвигаем время: тик срабатывает, runGC логирует ошибку
		time.Sleep(5*time.Minute + time.Second)
		synctest.Wait()

		// Cleanup
		close(r.stop)
		r.stop = nil
		r.db = nil
		synctest.Wait()
	})
}
