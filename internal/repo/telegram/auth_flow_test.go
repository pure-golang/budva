package telegram

import (
	"testing"
	"testing/synctest"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pure-golang/budva-claude/internal/config"
	"github.com/pure-golang/budva-claude/internal/domain"
)

func TestStart_EmitsWaitPhone(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		// Arrange
		repo := New(config.TelegramConfig{})

		// Act
		require.NoError(t, repo.Start(t.Context()))

		// Assert
		time.Sleep(1 * time.Millisecond)
		select {
		case event := <-repo.AuthStates():
			assert.Equal(t, domain.AuthStateWaitPhone, event.State)
		default:
			t.Error("expected WaitPhone event")
		}
	})
}

func TestSubmitPhone_EmitsWaitCode(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		// Arrange
		repo := New(config.TelegramConfig{})
		require.NoError(t, repo.Start(t.Context()))
		<-repo.AuthStates() // drain WaitPhone

		// Act
		require.NoError(t, repo.SubmitPhone(t.Context(), "+79261234567"))

		// Assert
		time.Sleep(1 * time.Millisecond)
		select {
		case event := <-repo.AuthStates():
			assert.Equal(t, domain.AuthStateWaitCode, event.State)
		default:
			t.Error("expected WaitCode event")
		}
	})
}

func TestSubmitCode_EmitsWaitPassword(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		// Arrange
		repo := New(config.TelegramConfig{})
		require.NoError(t, repo.Start(t.Context()))
		<-repo.AuthStates() // drain WaitPhone

		// Act
		require.NoError(t, repo.SubmitCode(t.Context(), "12345"))

		// Assert
		time.Sleep(1 * time.Millisecond)
		select {
		case event := <-repo.AuthStates():
			assert.Equal(t, domain.AuthStateWaitPassword, event.State)
			ws, ok := event.Extra.(*domain.WaitPasswordState)
			require.True(t, ok)
			assert.Equal(t, "2FA password", ws.PasswordHint)
		default:
			t.Error("expected WaitPassword event")
		}
	})
}

func TestSubmitPassword_EmitsReadyAndClosesClientDone(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		// Arrange
		repo := New(config.TelegramConfig{})
		require.NoError(t, repo.Start(t.Context()))
		<-repo.AuthStates() // drain WaitPhone

		// Act
		require.NoError(t, repo.SubmitPassword(t.Context(), "secret"))

		// Assert
		time.Sleep(1 * time.Millisecond)
		select {
		case event := <-repo.AuthStates():
			assert.Equal(t, domain.AuthStateReady, event.State)
		default:
			t.Error("expected Ready event")
		}

		select {
		case <-repo.ClientDone():
			// OK
		default:
			t.Error("clientDone should be closed after Ready")
		}
	})
}
