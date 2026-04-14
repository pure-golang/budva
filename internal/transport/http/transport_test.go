package http

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/pure-golang/budva-claude/internal/domain"
	"github.com/pure-golang/budva-claude/internal/transport/http/mocks"
)

func newTestTransport(t *testing.T, state domain.AuthorizationState) (*Transport, *mocks.AuthService) {
	t.Helper()
	auth := mocks.NewAuthService(t)
	auth.EXPECT().State().Return(state).Maybe()
	return New(auth, nil), auth
}

func TestGetState_WaitPhone(t *testing.T) {
	t.Parallel()

	// Arrange
	tr, auth := newTestTransport(t, domain.AuthStateWaitPhone)
	auth.EXPECT().Extra().Return(nil).Maybe()
	mux := http.NewServeMux()
	tr.EnrichRoutes(mux)
	req := httptest.NewRequest(http.MethodGet, "/api/auth/telegram/state", nil)
	w := httptest.NewRecorder()

	// Act
	mux.ServeHTTP(w, req)

	// Assert
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `"state_type":"waitPhone"`)
}

func TestGetState_Ready(t *testing.T) {
	t.Parallel()

	// Arrange
	tr, auth := newTestTransport(t, domain.AuthStateReady)
	auth.EXPECT().Extra().Return(nil).Maybe()
	mux := http.NewServeMux()
	tr.EnrichRoutes(mux)
	req := httptest.NewRequest(http.MethodGet, "/api/auth/telegram/state", nil)
	w := httptest.NewRecorder()

	// Act
	mux.ServeHTTP(w, req)

	// Assert
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `"state_type":"ready"`)
}

func TestPostPhone_Success(t *testing.T) {
	t.Parallel()

	// Arrange
	tr, auth := newTestTransport(t, domain.AuthStateWaitPhone)
	inputChan := make(chan string, 1)
	auth.EXPECT().InputChan().Return(inputChan)
	mux := http.NewServeMux()
	tr.EnrichRoutes(mux)
	body := strings.NewReader(`{"phone":"+1234567890"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/auth/telegram/phone", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	// Act
	mux.ServeHTTP(w, req)

	// Assert
	assert.Equal(t, http.StatusAccepted, w.Code)
	assert.Contains(t, w.Body.String(), `"status":"accepted"`)
	got := <-inputChan
	assert.Equal(t, "+1234567890", got)
}

func TestPostPhone_EmptyPhone(t *testing.T) {
	t.Parallel()

	// Arrange
	tr, _ := newTestTransport(t, domain.AuthStateWaitPhone)
	mux := http.NewServeMux()
	tr.EnrichRoutes(mux)
	body := strings.NewReader(`{"phone":""}`)
	req := httptest.NewRequest(http.MethodPost, "/api/auth/telegram/phone", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	// Act
	mux.ServeHTTP(w, req)

	// Assert
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestPostPhone_InvalidJSON(t *testing.T) {
	t.Parallel()

	// Arrange
	tr, _ := newTestTransport(t, domain.AuthStateWaitPhone)
	mux := http.NewServeMux()
	tr.EnrichRoutes(mux)
	body := strings.NewReader(`invalid`)
	req := httptest.NewRequest(http.MethodPost, "/api/auth/telegram/phone", body)
	w := httptest.NewRecorder()

	// Act
	mux.ServeHTTP(w, req)

	// Assert
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestPostCode_Success(t *testing.T) {
	t.Parallel()

	// Arrange
	tr, auth := newTestTransport(t, domain.AuthStateWaitCode)
	inputChan := make(chan string, 1)
	auth.EXPECT().InputChan().Return(inputChan)
	mux := http.NewServeMux()
	tr.EnrichRoutes(mux)
	body := strings.NewReader(`{"code":"12345"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/auth/telegram/code", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	// Act
	mux.ServeHTTP(w, req)

	// Assert
	assert.Equal(t, http.StatusAccepted, w.Code)
	got := <-inputChan
	assert.Equal(t, "12345", got)
}

func TestPostCode_EmptyCode(t *testing.T) {
	t.Parallel()

	// Arrange
	tr, _ := newTestTransport(t, domain.AuthStateWaitCode)
	mux := http.NewServeMux()
	tr.EnrichRoutes(mux)
	body := strings.NewReader(`{"code":""}`)
	req := httptest.NewRequest(http.MethodPost, "/api/auth/telegram/code", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	// Act
	mux.ServeHTTP(w, req)

	// Assert
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestPostPassword_Success(t *testing.T) {
	t.Parallel()

	// Arrange
	tr, auth := newTestTransport(t, domain.AuthStateWaitPassword)
	inputChan := make(chan string, 1)
	auth.EXPECT().InputChan().Return(inputChan)
	mux := http.NewServeMux()
	tr.EnrichRoutes(mux)
	body := strings.NewReader(`{"password":"secret123"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/auth/telegram/password", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	// Act
	mux.ServeHTTP(w, req)

	// Assert
	assert.Equal(t, http.StatusAccepted, w.Code)
	got := <-inputChan
	assert.Equal(t, "secret123", got)
}

func TestPostPassword_EmptyPassword(t *testing.T) {
	t.Parallel()

	// Arrange
	tr, _ := newTestTransport(t, domain.AuthStateWaitPassword)
	mux := http.NewServeMux()
	tr.EnrichRoutes(mux)
	body := strings.NewReader(`{"password":""}`)
	req := httptest.NewRequest(http.MethodPost, "/api/auth/telegram/password", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	// Act
	mux.ServeHTTP(w, req)

	// Assert
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestResponseContentType(t *testing.T) {
	t.Parallel()

	// Arrange
	tr, auth := newTestTransport(t, domain.AuthStateReady)
	auth.EXPECT().Extra().Return(nil).Maybe()
	mux := http.NewServeMux()
	tr.EnrichRoutes(mux)
	req := httptest.NewRequest(http.MethodGet, "/api/auth/telegram/state", nil)
	w := httptest.NewRecorder()

	// Act
	mux.ServeHTTP(w, req)

	// Assert
	ct := w.Header().Get("Content-Type")
	assert.Equal(t, "application/json", ct)
}

func TestGetState_PasswordHint(t *testing.T) {
	t.Parallel()

	// Arrange
	auth := mocks.NewAuthService(t)
	auth.EXPECT().State().Return(domain.AuthStateWaitPassword)
	auth.EXPECT().Extra().Return(&domain.WaitPasswordState{PasswordHint: "pet name"})
	tr := New(auth, nil)
	mux := http.NewServeMux()
	tr.EnrichRoutes(mux)
	req := httptest.NewRequest(http.MethodGet, "/api/auth/telegram/state", nil)
	w := httptest.NewRecorder()

	// Act
	mux.ServeHTTP(w, req)

	// Assert
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `"state_type":"waitPassword"`)
	assert.Contains(t, w.Body.String(), `"password_hint":"pet name"`)
}

func TestPostPhone_NoBody(t *testing.T) {
	t.Parallel()

	// Arrange
	tr, _ := newTestTransport(t, domain.AuthStateWaitPhone)
	mux := http.NewServeMux()
	tr.EnrichRoutes(mux)
	req := httptest.NewRequest(http.MethodPost, "/api/auth/telegram/phone", nil)
	w := httptest.NewRecorder()

	// Act
	mux.ServeHTTP(w, req)

	// Assert
	assert.Equal(t, http.StatusBadRequest, w.Code)
}
