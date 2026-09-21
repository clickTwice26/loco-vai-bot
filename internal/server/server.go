package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"
)

// Server provides a lightweight HTTP server for platform health checks.
type Server struct {
	httpServer *http.Server
	logger     *slog.Logger
}

// New creates a new HTTP health server listening on 0.0.0.0:<port>.
func New(port string, logger *slog.Logger) *Server {
	if port == "" {
		port = "8080"
	}

	mux := http.NewServeMux()

	s := &Server{
		httpServer: &http.Server{
			Addr:              fmt.Sprintf("0.0.0.0:%s", port),
			Handler:           mux,
			ReadHeaderTimeout: 5 * time.Second,
			ReadTimeout:       10 * time.Second,
			WriteTimeout:      10 * time.Second,
			IdleTimeout:       30 * time.Second,
		},
		logger: logger.With("module", "http-server"),
	}

	mux.HandleFunc("/healthz", s.handleHealthz)
	mux.HandleFunc("/", s.handleRoot)

	return s
}

func (s *Server) handleHealthz(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"status": "ok",
	})
}

func (s *Server) handleRoot(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"name":   "localoy-bot",
		"status": "running",
	})
}

// Start launches the HTTP server in a background goroutine.
func (s *Server) Start() {
	go func() {
		s.logger.Info("starting HTTP health check server", "addr", s.httpServer.Addr)
		if err := s.httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			s.logger.Error("HTTP health server encountered an error", "error", err)
		}
	}()
}

// Shutdown gracefully stops the HTTP server.
func (s *Server) Shutdown(ctx context.Context) error {
	s.logger.Info("shutting down HTTP health server...")
	return s.httpServer.Shutdown(ctx)
}
