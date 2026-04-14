package graph

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	dtogql "github.com/pure-golang/budva-claude/internal/dto/graphql"
)

type mockStatusProvider struct {
	fn func(ctx context.Context) (*dtogql.StatusResponse, error)
}

func (m *mockStatusProvider) GetStatus(ctx context.Context) (*dtogql.StatusResponse, error) {
	return m.fn(ctx)
}

func TestHandler_StatusQuery(t *testing.T) {
	t.Parallel()

	// Arrange
	provider := &mockStatusProvider{
		fn: func(_ context.Context) (*dtogql.StatusResponse, error) {
			return &dtogql.StatusResponse{TDLibVersion: "1.8.0", UserID: 12345}, nil
		},
	}
	resolver := NewResolver(provider)
	handler := resolver.Handler()

	body := strings.NewReader(`{"query":"{ status { tdlibVersion userId } }"}`)
	req := httptest.NewRequest(http.MethodPost, "/graphql", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	// Act
	handler(w, req)

	// Assert
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `"tdlibVersion":"1.8.0"`)
	assert.Contains(t, w.Body.String(), `"userId":12345`)
}

func TestHandler_StatusError(t *testing.T) {
	t.Parallel()

	// Arrange
	provider := &mockStatusProvider{
		fn: func(_ context.Context) (*dtogql.StatusResponse, error) {
			return nil, errors.New("telegram unavailable")
		},
	}
	resolver := NewResolver(provider)
	handler := resolver.Handler()

	body := strings.NewReader(`{"query":"{ status { tdlibVersion } }"}`)
	req := httptest.NewRequest(http.MethodPost, "/graphql", body)
	w := httptest.NewRecorder()

	// Act
	handler(w, req)

	// Assert
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `"errors"`)
	assert.Contains(t, w.Body.String(), "telegram unavailable")
}

func TestHandler_InvalidBody(t *testing.T) {
	t.Parallel()

	// Arrange
	resolver := NewResolver(&mockStatusProvider{})
	handler := resolver.Handler()

	body := strings.NewReader(`invalid json`)
	req := httptest.NewRequest(http.MethodPost, "/graphql", body)
	w := httptest.NewRecorder()

	// Act
	handler(w, req)

	// Assert
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestHandler_UnknownQuery(t *testing.T) {
	t.Parallel()

	// Arrange
	resolver := NewResolver(&mockStatusProvider{})
	handler := resolver.Handler()

	body := strings.NewReader(`{"query":"{ unknownField }"}`)
	req := httptest.NewRequest(http.MethodPost, "/graphql", body)
	w := httptest.NewRecorder()

	// Act
	handler(w, req)

	// Assert
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "unknown query")
}

func TestPlaygroundHandler(t *testing.T) {
	t.Parallel()

	// Arrange
	handler := PlaygroundHandler("/graphql")
	req := httptest.NewRequest(http.MethodGet, "/playground", nil)
	w := httptest.NewRecorder()

	// Act
	handler(w, req)

	// Assert
	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Header().Get("Content-Type"), "text/html")
	assert.Contains(t, w.Body.String(), "/graphql")
	assert.Contains(t, w.Body.String(), "GraphQL Playground")
}
