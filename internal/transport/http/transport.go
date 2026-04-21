package http

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/99designs/gqlgen/graphql/playground"
	alogger "github.com/pure-golang/adapters/logger"

	"github.com/pure-golang/budva-claude/internal/domain"
	transportgraphql "github.com/pure-golang/budva-claude/internal/transport/http/graphql"
	"github.com/pure-golang/budva-claude/internal/transport/http/resolvers"
)

type authService interface {
	Subscribe(listener func(state domain.AuthorizationState, extra any))
	InputChan() chan<- string
	State() domain.AuthorizationState
	Extra() any
}

// Transport реализует HTTP-транспорт с REST-эндпоинтами для авторизации.
type Transport struct {
	authService authService
	resolver    *resolvers.Resolver
}

// New создаёт новый экземпляр HTTP-транспорта.
func New(authService authService, resolver *resolvers.Resolver) *Transport {
	return &Transport{
		authService: authService,
		resolver:    resolver,
	}
}

// EnrichRoutes регистрирует маршруты транспорта в mux.
func (t *Transport) EnrichRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/auth/telegram/state", t.handleGetState)
	mux.HandleFunc("POST /api/auth/telegram/phone", t.handlePostPhone)
	mux.HandleFunc("POST /api/auth/telegram/code", t.handlePostCode)
	mux.HandleFunc("POST /api/auth/telegram/password", t.handlePostPassword)
	if t.resolver != nil {
		srv := handler.NewDefaultServer(
			transportgraphql.NewExecutableSchema(transportgraphql.Config{Resolvers: t.resolver}),
		)
		mux.Handle("POST /graphql", srv)
		mux.Handle("GET /playground", playground.Handler("GraphQL playground", "/graphql"))
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
	state := t.authService.State()
	resp := stateResponse{StateType: state.String()}

	if state == domain.AuthStateWaitPassword {
		if ws, ok := t.authService.Extra().(*domain.WaitPasswordState); ok && ws != nil {
			resp.PasswordHint = ws.PasswordHint
		}
	}

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

	t.authService.InputChan() <- req.Phone

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

	t.authService.InputChan() <- req.Code

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

	t.authService.InputChan() <- req.Password

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	if err := json.NewEncoder(w).Encode(statusResponse{Status: "accepted"}); err != nil {
		alogger.FromContext(r.Context()).Error("Failed to encode response", slog.Any("err", err))
	}
}
