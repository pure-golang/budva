package graph

import (
	"encoding/json"
	"log/slog"
	"net/http"

	alogger "github.com/pure-golang/adapters/logger"
)

type graphqlRequest struct {
	Query string `json:"query"`
}

type graphqlResponse struct {
	Data   any              `json:"data,omitempty"`
	Errors []graphqlError   `json:"errors,omitempty"`
}

type graphqlError struct {
	Message string `json:"message"`
}

// Handler обрабатывает GraphQL-запросы.
func (r *Resolver) Handler() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		logger := alogger.FromContext(req.Context())

		var gqlReq graphqlRequest
		if err := json.NewDecoder(req.Body).Decode(&gqlReq); err != nil {
			writeJSON(w, http.StatusBadRequest, graphqlResponse{
				Errors: []graphqlError{{Message: "invalid request body"}},
			}, logger)
			return
		}

		// Минимальный роутинг по query
		if containsField(gqlReq.Query, "status") {
			r.handleStatus(w, req, logger)
			return
		}

		writeJSON(w, http.StatusOK, graphqlResponse{
			Errors: []graphqlError{{Message: "unknown query"}},
		}, logger)
	}
}

func (r *Resolver) handleStatus(w http.ResponseWriter, req *http.Request, logger *slog.Logger) {
	st, err := r.status.GetStatus(req.Context())
	if err != nil {
		writeJSON(w, http.StatusOK, graphqlResponse{
			Errors: []graphqlError{{Message: err.Error()}},
		}, logger)
		return
	}

	writeJSON(w, http.StatusOK, graphqlResponse{
		Data: map[string]any{
			"status": st,
		},
	}, logger)
}

func containsField(query, field string) bool {
	for i := range len(query) - len(field) + 1 {
		if query[i:i+len(field)] == field {
			return true
		}
	}
	return false
}

func writeJSON(w http.ResponseWriter, code int, v any, logger *slog.Logger) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		logger.Error("Failed to encode response", slog.Any("err", err))
	}
}
