package health

import (
	"context"
	"net/http"
	"time"
)

type pinger interface {
	Ping(ctx context.Context) error
}

// Controller реализует health-проверки.
type Controller struct {
	pingers []pinger
}

// New создаёт новый контроллер с указанными pingers.
func New(pingers ...pinger) *Controller {
	return &Controller{pingers: pingers}
}

// EnrichRoutes регистрирует health-эндпоинты на мультиплексоре.
func (c *Controller) EnrichRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/healthcheck", c.healthcheck)
	mux.HandleFunc("/health", c.healthcheck)
	mux.HandleFunc("/ready", c.ready)
	mux.HandleFunc("/live", c.live)
}

func (c *Controller) healthcheck(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()

	for _, p := range c.pingers {
		if err := p.Ping(ctx); err != nil {
			http.Error(w, err.Error(), http.StatusServiceUnavailable)
			return
		}
	}
	w.WriteHeader(http.StatusOK)
}

func (c *Controller) ready(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()

	for _, p := range c.pingers {
		if err := p.Ping(ctx); err != nil {
			http.Error(w, err.Error(), http.StatusServiceUnavailable)
			return
		}
	}
	w.WriteHeader(http.StatusOK)
}

func (c *Controller) live(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
}
