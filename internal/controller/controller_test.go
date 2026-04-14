package controller

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/pure-golang/budva-claude/internal/controller/mocks"
)

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
	p := mocks.NewPinger(t)
	p.EXPECT().Ping(mock.Anything).Return(nil)
	ctrl := New(p)
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
	p := mocks.NewPinger(t)
	p.EXPECT().Ping(mock.Anything).Return(errors.New("db is down"))
	ctrl := New(p)
	mux := http.NewServeMux()
	ctrl.EnrichRoutes(mux)

	// Act
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/healthcheck", nil))

	// Assert
	require.Equal(t, http.StatusServiceUnavailable, rec.Code)
}
