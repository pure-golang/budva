package http

import (
	"encoding/json"
	"log/slog"
	"net/http"

	alogger "github.com/pure-golang/adapters/logger"

	"github.com/pure-golang/budva-claude/internal/domain"
	"github.com/pure-golang/budva-claude/internal/transport/http/graph"
)

type authService interface {
	Subscribe(listener func(state domain.AuthorizationState, extra any))
	InputChan() chan<- string
	State() domain.AuthorizationState
}

// Transport реализует HTTP-транспорт с REST-эндпоинтами для авторизации.
type Transport struct {
	auth     authService
	resolver *graph.Resolver
}

// New создаёт новый экземпляр HTTP-транспорта.
func New(auth authService, resolver *graph.Resolver) *Transport {
	return &Transport{
		auth:     auth,
		resolver: resolver,
	}
}

// EnrichRoutes регистрирует маршруты транспорта в mux.
func (t *Transport) EnrichRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/auth/telegram/state", t.handleGetState)
	mux.HandleFunc("POST /api/auth/telegram/phone", t.handlePostPhone)
	mux.HandleFunc("POST /api/auth/telegram/code", t.handlePostCode)
	mux.HandleFunc("POST /api/auth/telegram/password", t.handlePostPassword)
	if t.resolver != nil {
		mux.HandleFunc("POST /graphql", t.resolver.Handler())
		mux.HandleFunc("GET /playground", graph.PlaygroundHandler("/graphql"))
	}
}

type stateResponse struct {
	StateType    string `json:"state_type"`
	PasswordHint string `json:"password_hint,omitempty"`
}

type inputRequest struct {
	Phone    string `json:"phone,omitempty"`
	Code     string `json:"code,omitempty"`
	Password string `json:"password,omitempty"`
}

type statusResponse struct {
	Status string `json:"status"`
}

func (t *Transport) handleGetState(w http.ResponseWriter, r *http.Request) {
	state := t.auth.State()
	resp := stateResponse{StateType: state.String()}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		alogger.FromContext(r.Context()).Error("Failed to encode state response", slog.Any("err", err))
	}
}

func (t *Transport) handlePostPhone(w http.ResponseWriter, r *http.Request) {
	var req inputRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}
	if req.Phone == "" {
		http.Error(w, `{"error":"phone is required"}`, http.StatusBadRequest)
		return
	}

	t.auth.InputChan() <- req.Phone

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	if err := json.NewEncoder(w).Encode(statusResponse{Status: "accepted"}); err != nil {
		alogger.FromContext(r.Context()).Error("Failed to encode response", slog.Any("err", err))
	}
}

func (t *Transport) handlePostCode(w http.ResponseWriter, r *http.Request) {
	var req inputRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}
	if req.Code == "" {
		http.Error(w, `{"error":"code is required"}`, http.StatusBadRequest)
		return
	}

	t.auth.InputChan() <- req.Code

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	if err := json.NewEncoder(w).Encode(statusResponse{Status: "accepted"}); err != nil {
		alogger.FromContext(r.Context()).Error("Failed to encode response", slog.Any("err", err))
	}
}

func (t *Transport) handlePostPassword(w http.ResponseWriter, r *http.Request) {
	var req inputRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}
	if req.Password == "" {
		http.Error(w, `{"error":"password is required"}`, http.StatusBadRequest)
		return
	}

	t.auth.InputChan() <- req.Password

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	if err := json.NewEncoder(w).Encode(statusResponse{Status: "accepted"}); err != nil {
		alogger.FromContext(r.Context()).Error("Failed to encode response", slog.Any("err", err))
	}
}
