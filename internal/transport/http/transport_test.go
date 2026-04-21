package http

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/pure-golang/budva-claude/internal/domain"
	"github.com/pure-golang/budva-claude/internal/transport/http/mocks"
	"github.com/pure-golang/budva-claude/internal/transport/http/resolvers"
	resmocks "github.com/pure-golang/budva-claude/internal/transport/http/resolvers/mocks"
)

type errWriter struct {
	header http.Header
}

func (w *errWriter) Header() http.Header {
	if w.header == nil {
		w.header = make(http.Header)
	}
	return w.header
}
func (w *errWriter) Write([]byte) (int, error) { return 0, errors.New("write error") }
func (w *errWriter) WriteHeader(int)            {}

func newTestTransport(t *testing.T) (*Transport, *mocks.AuthService) {
	t.Helper()
	auth := mocks.NewAuthService(t)
	return New(auth, nil), auth
}

func TestGetState_WaitPhone(t *testing.T) {
	t.Parallel()

	// Arrange
	tr, auth := newTestTransport(t)
	auth.EXPECT().State().Return(domain.AuthStateWaitPhone)
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
	tr, auth := newTestTransport(t)
	auth.EXPECT().State().Return(domain.AuthStateReady)
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
	tr, auth := newTestTransport(t)
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
	tr, _ := newTestTransport(t)
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
	tr, _ := newTestTransport(t)
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
	tr, auth := newTestTransport(t)
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
	tr, _ := newTestTransport(t)
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
	tr, auth := newTestTransport(t)
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
	tr, _ := newTestTransport(t)
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
	tr, auth := newTestTransport(t)
	auth.EXPECT().State().Return(domain.AuthStateReady)
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
	tr, _ := newTestTransport(t)
	mux := http.NewServeMux()
	tr.EnrichRoutes(mux)
	req := httptest.NewRequest(http.MethodPost, "/api/auth/telegram/phone", nil)
	w := httptest.NewRecorder()

	// Act
	mux.ServeHTTP(w, req)

	// Assert
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestPostCode_InvalidJSON(t *testing.T) {
	t.Parallel()

	// Arrange
	tr, _ := newTestTransport(t)
	mux := http.NewServeMux()
	tr.EnrichRoutes(mux)
	body := strings.NewReader(`not json`)
	req := httptest.NewRequest(http.MethodPost, "/api/auth/telegram/code", body)
	w := httptest.NewRecorder()

	// Act
	mux.ServeHTTP(w, req)

	// Assert
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestPostCode_NoBody(t *testing.T) {
	t.Parallel()

	// Arrange
	tr, _ := newTestTransport(t)
	mux := http.NewServeMux()
	tr.EnrichRoutes(mux)
	req := httptest.NewRequest(http.MethodPost, "/api/auth/telegram/code", nil)
	w := httptest.NewRecorder()

	// Act
	mux.ServeHTTP(w, req)

	// Assert
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestPostPassword_InvalidJSON(t *testing.T) {
	t.Parallel()

	// Arrange
	tr, _ := newTestTransport(t)
	mux := http.NewServeMux()
	tr.EnrichRoutes(mux)
	body := strings.NewReader(`not json`)
	req := httptest.NewRequest(http.MethodPost, "/api/auth/telegram/password", body)
	w := httptest.NewRecorder()

	// Act
	mux.ServeHTTP(w, req)

	// Assert
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestPostPassword_NoBody(t *testing.T) {
	t.Parallel()

	// Arrange
	tr, _ := newTestTransport(t)
	mux := http.NewServeMux()
	tr.EnrichRoutes(mux)
	req := httptest.NewRequest(http.MethodPost, "/api/auth/telegram/password", nil)
	w := httptest.NewRecorder()

	// Act
	mux.ServeHTTP(w, req)

	// Assert
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestEnrichRoutes_WithResolver(t *testing.T) {
	t.Parallel()

	// Arrange
	auth := mocks.NewAuthService(t)
	statusSvc := resmocks.NewStatusService(t)
	resolver := resolvers.New(statusSvc)
	tr := New(auth, resolver)
	mux := http.NewServeMux()

	// Act — EnrichRoutes с не-nil resolver регистрирует GraphQL-маршруты
	tr.EnrichRoutes(mux)

	// Assert — GraphQL endpoint зарегистрирован (playground отвечает)
	req := httptest.NewRequest(http.MethodGet, "/playground", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestHandleGetState_EncodeError(t *testing.T) {
	t.Parallel()

	// Arrange
	auth := mocks.NewAuthService(t)
	auth.EXPECT().State().Return(domain.AuthStateReady)
	tr := New(auth, nil)
	w := &errWriter{}
	req := httptest.NewRequest(http.MethodGet, "/api/auth/telegram/state", nil)

	// Act / Assert — не паникует при ошибке записи
	tr.handleGetState(w, req)
}

func TestHandlePostPhone_EncodeError(t *testing.T) {
	t.Parallel()

	// Arrange
	auth := mocks.NewAuthService(t)
	inputChan := make(chan string, 1)
	auth.EXPECT().InputChan().Return(inputChan)
	tr := New(auth, nil)
	w := &errWriter{}
	body := strings.NewReader(`{"phone":"+1234567890"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/auth/telegram/phone", body)
	req.Header.Set("Content-Type", "application/json")

	// Act / Assert — не паникует при ошибке записи
	tr.handlePostPhone(w, req)
	<-inputChan // consume the sent value
}

func TestHandlePostCode_EncodeError(t *testing.T) {
	t.Parallel()

	// Arrange
	auth := mocks.NewAuthService(t)
	inputChan := make(chan string, 1)
	auth.EXPECT().InputChan().Return(inputChan)
	tr := New(auth, nil)
	w := &errWriter{}
	body := strings.NewReader(`{"code":"12345"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/auth/telegram/code", body)
	req.Header.Set("Content-Type", "application/json")

	// Act / Assert — не паникует при ошибке записи
	tr.handlePostCode(w, req)
	<-inputChan
}

func TestHandlePostPassword_EncodeError(t *testing.T) {
	t.Parallel()

	// Arrange
	auth := mocks.NewAuthService(t)
	inputChan := make(chan string, 1)
	auth.EXPECT().InputChan().Return(inputChan)
	tr := New(auth, nil)
	w := &errWriter{}
	body := strings.NewReader(`{"password":"secret"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/auth/telegram/password", body)
	req.Header.Set("Content-Type", "application/json")

	// Act / Assert — не паникует при ошибке записи
	tr.handlePostPassword(w, req)
	<-inputChan
}

func TestGetState_WaitPassword_ExtraNotWaitPasswordState(t *testing.T) {
	t.Parallel()

	// Arrange
	auth := mocks.NewAuthService(t)
	auth.EXPECT().State().Return(domain.AuthStateWaitPassword)
	auth.EXPECT().Extra().Return("not a WaitPasswordState")
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
	assert.NotContains(t, w.Body.String(), `"password_hint"`)
}
