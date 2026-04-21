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
func (w *errWriter) WriteHeader(int)           {}

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

func TestPostRejectsEmptyField(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		path string
		body string
	}{
		{name: "phone", path: "/api/auth/telegram/phone", body: `{"phone":""}`},
		{name: "code", path: "/api/auth/telegram/code", body: `{"code":""}`},
		{name: "password", path: "/api/auth/telegram/password", body: `{"password":""}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			tr, _ := newTestTransport(t)
			mux := http.NewServeMux()
			tr.EnrichRoutes(mux)
			req := httptest.NewRequest(http.MethodPost, tt.path, strings.NewReader(tt.body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			// Act
			mux.ServeHTTP(w, req)

			// Assert
			assert.Equal(t, http.StatusBadRequest, w.Code)
		})
	}
}

func TestPostRejectsInvalidJSON(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		path string
	}{
		{name: "phone", path: "/api/auth/telegram/phone"},
		{name: "code", path: "/api/auth/telegram/code"},
		{name: "password", path: "/api/auth/telegram/password"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			tr, _ := newTestTransport(t)
			mux := http.NewServeMux()
			tr.EnrichRoutes(mux)
			req := httptest.NewRequest(http.MethodPost, tt.path, strings.NewReader(`not json`))
			w := httptest.NewRecorder()

			// Act
			mux.ServeHTTP(w, req)

			// Assert
			assert.Equal(t, http.StatusBadRequest, w.Code)
		})
	}
}

func TestPostRejectsNoBody(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		path string
	}{
		{name: "phone", path: "/api/auth/telegram/phone"},
		{name: "code", path: "/api/auth/telegram/code"},
		{name: "password", path: "/api/auth/telegram/password"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			tr, _ := newTestTransport(t)
			mux := http.NewServeMux()
			tr.EnrichRoutes(mux)
			req := httptest.NewRequest(http.MethodPost, tt.path, nil)
			w := httptest.NewRecorder()

			// Act
			mux.ServeHTTP(w, req)

			// Assert
			assert.Equal(t, http.StatusBadRequest, w.Code)
		})
	}
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

func TestHandlePostEncodeError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		body   string
		invoke func(*Transport, http.ResponseWriter, *http.Request)
	}{
		{
			name: "phone",
			body: `{"phone":"+1234567890"}`,
			invoke: func(tr *Transport, w http.ResponseWriter, req *http.Request) {
				tr.handlePostPhone(w, req)
			},
		},
		{
			name: "code",
			body: `{"code":"12345"}`,
			invoke: func(tr *Transport, w http.ResponseWriter, req *http.Request) {
				tr.handlePostCode(w, req)
			},
		},
		{
			name: "password",
			body: `{"password":"secret"}`,
			invoke: func(tr *Transport, w http.ResponseWriter, req *http.Request) {
				tr.handlePostPassword(w, req)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			auth := mocks.NewAuthService(t)
			inputChan := make(chan string, 1)
			auth.EXPECT().InputChan().Return(inputChan)
			tr := New(auth, nil)
			w := &errWriter{}
			req := httptest.NewRequest(http.MethodPost, "/ignored", strings.NewReader(tt.body))
			req.Header.Set("Content-Type", "application/json")

			// Act
			tt.invoke(tr, w, req)

			// Assert
			<-inputChan
		})
	}
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
