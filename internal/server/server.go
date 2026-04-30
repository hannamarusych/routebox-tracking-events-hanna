package server

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/312school/routebox-tracking-events/internal/config"
	"github.com/312school/routebox-tracking-events/internal/db"
)

// New builds an *http.Server wired with routes + middleware.
func New(cfg *config.Config, repo *db.EventRepo, logger *slog.Logger) *http.Server {
	h := NewHandlers(cfg, repo, logger)

	r := chi.NewRouter()
	r.Use(Middleware(logger))

	r.Get("/healthz", h.Healthz)
	r.Get("/readyz", h.Readyz)
	r.Post("/v1/webhooks/{carrier}", h.Webhook)

	return &http.Server{
		Addr:              fmt.Sprintf(":%d", cfg.Port),
		Handler:           r,
		ReadTimeout:       15 * time.Second,
		ReadHeaderTimeout: 5 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       120 * time.Second,
		MaxHeaderBytes:    1 << 14, // 16KB
	}
}

// Shutdown gracefully stops the server, with a hard ceiling.
func Shutdown(ctx context.Context, srv *http.Server) error {
	shutdownCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	return srv.Shutdown(shutdownCtx)
}
