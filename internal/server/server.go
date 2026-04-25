package server

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/varmakarthik12/owui-proxy/internal/config"
	"github.com/varmakarthik12/owui-proxy/internal/handler"
	"github.com/varmakarthik12/owui-proxy/internal/middleware"
	"github.com/varmakarthik12/owui-proxy/internal/owuiclient"
)

// Run starts the HTTP server with all routes and middleware, handling
// graceful shutdown on SIGINT/SIGTERM.
func Run(ctx context.Context, cfg *config.Config) error {
	client := owuiclient.New(cfg)

	mux := http.NewServeMux()
	registerRoutes(mux, client, cfg)

	// Build the middleware chain.
	var h http.Handler = mux
	h = middleware.Logger(h)
	h = middleware.Limit(cfg.RateLimit)(h)
	h = middleware.Recovery(h)
	h = maxBodySize(cfg.MaxBodySize)(h)

	srv := &http.Server{
		Addr:         cfg.ListenAddress(),
		Handler:      h,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 0, // disabled for streaming
		IdleTimeout:  120 * time.Second,
	}

	// Channel for server errors.
	errCh := make(chan error, 1)

	go func() {
		if cfg.TLSCert != "" && cfg.TLSKey != "" {
			slog.Info("starting HTTPS server", "addr", srv.Addr)
			errCh <- srv.ListenAndServeTLS(cfg.TLSCert, cfg.TLSKey)
		} else {
			slog.Info("starting HTTP server", "addr", srv.Addr)
			errCh <- srv.ListenAndServe()
		}
	}()

	// Print a user-friendly startup message.
	scheme := "http"
	if cfg.TLSCert != "" {
		scheme = "https"
	}
	fmt.Fprintf(os.Stderr, "\n  owui-proxy is running at %s://%s\n\n", scheme, cfg.ListenAddress())
	if cfg.BindAll {
		slog.Warn("server is bound to 0.0.0.0 — accessible from the network. Ensure this is intentional.")
	}

	// Wait for shutdown signal or server error.
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	select {
	case sig := <-quit:
		slog.Info("received shutdown signal", "signal", sig)
	case err := <-errCh:
		if err != nil && err != http.ErrServerClosed {
			return fmt.Errorf("server error: %w", err)
		}
	case <-ctx.Done():
		slog.Info("context cancelled")
	}

	// Graceful shutdown with 15-second deadline.
	slog.Info("shutting down gracefully (15s deadline)...")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("graceful shutdown failed: %w", err)
	}

	slog.Info("server stopped")
	return nil
}

// registerRoutes sets up all Ollama-compatible API routes.
func registerRoutes(mux *http.ServeMux, client *owuiclient.Client, cfg *config.Config) {
	// Implemented endpoints.
	mux.HandleFunc("GET /api/version", handler.Version(cfg.MockVersion))
	mux.HandleFunc("GET /api/tags", handler.Tags(client))
	mux.HandleFunc("POST /api/chat", handler.Chat(client))
	mux.HandleFunc("POST /api/generate", handler.Generate(client))
	mux.HandleFunc("POST /api/show", handler.Show(client))
	mux.HandleFunc("POST /api/embeddings", handler.Embeddings(client))
	mux.HandleFunc("POST /api/embed", handler.Embed(client))
	mux.HandleFunc("GET /api/ps", handler.Ps())

	// Unimplemented endpoints — graceful 501.
	mux.HandleFunc("POST /api/pull", handler.Unimplemented())
	mux.HandleFunc("POST /api/push", handler.Unimplemented())
	mux.HandleFunc("DELETE /api/delete", handler.Unimplemented())
	mux.HandleFunc("POST /api/delete", handler.Unimplemented())
	mux.HandleFunc("POST /api/copy", handler.Unimplemented())
	mux.HandleFunc("POST /api/create", handler.Unimplemented())
	mux.HandleFunc("POST /api/blobs/", handler.Unimplemented())
	mux.HandleFunc("HEAD /api/blobs/", handler.Unimplemented())

	// Root endpoint — some Ollama clients check if the server is alive.
	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte("Ollama is running"))
	})

	// HEAD / — some clients use HEAD to check connectivity.
	mux.HandleFunc("HEAD /{$}", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
}

// maxBodySize returns middleware that limits request body size.
func maxBodySize(maxBytes int64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Body != nil {
				r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
			}
			next.ServeHTTP(w, r)
		})
	}
}
