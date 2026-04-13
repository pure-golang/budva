package controller

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

type mockPinger struct {
	err error
}

func (m *mockPinger) Ping(_ context.Context) error { return m.err }

func TestLive_always_200(t *testing.T) {
	t.Parallel()

	// Arrange
	ctrl := New()
	mux := http.NewServeMux()
	ctrl.EnrichRoutes(mux)

	// Act
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/live", nil))

	// Assert
	require.Equal(t, http.StatusOK, rec.Code)
}

func TestHealthcheck_all_healthy(t *testing.T) {
	t.Parallel()

	// Arrange
	ctrl := New(&mockPinger{})
	mux := http.NewServeMux()
	ctrl.EnrichRoutes(mux)

	// Act
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/healthcheck", nil))

	// Assert
	require.Equal(t, http.StatusOK, rec.Code)
}

func TestHealthcheck_unhealthy(t *testing.T) {
	t.Parallel()

	// Arrange
	ctrl := New(&mockPinger{err: errors.New("db is down")})
	mux := http.NewServeMux()
	ctrl.EnrichRoutes(mux)

	// Act
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/healthcheck", nil))

	// Assert
	require.Equal(t, http.StatusServiceUnavailable, rec.Code)
}
