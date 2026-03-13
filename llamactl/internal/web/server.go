// Package web provides the embedded HTTP server for the llamactl Web UI.
//
// Architecture:
//   - server.go  — HTTP server setup, embed.FS, routing
//   - handlers.go — request handlers that call internal/service
//   - templates/  — HTML templates served as embedded files
//
// The server is intentionally dependency-free (stdlib net/http + html/template).
// All state mutations go through internal/service, keeping the handler layer thin.
package web

import (
	"embed"
	"fmt"
	"html/template"
	"log"
	"net/http"

	"github.com/andermurias/llamactl/internal/config"
)

//go:embed templates static
var embeddedFS embed.FS

// Server holds the HTTP server and its dependencies.
type Server struct {
	cfg  *config.Config
	tmpl *template.Template
	mux  *http.ServeMux
}

// New creates and configures a new Server.
func New(cfg *config.Config) (*Server, error) {
	tmpl, err := template.ParseFS(embeddedFS, "templates/*.html")
	if err != nil {
		return nil, fmt.Errorf("parse templates: %w", err)
	}

	s := &Server{cfg: cfg, tmpl: tmpl, mux: http.NewServeMux()}
	s.routes()
	return s, nil
}

// Start listens on 0.0.0.0:<port> and blocks until the process is killed.
func (s *Server) Start(port string) error {
	addr := "0.0.0.0:" + port
	log.Printf("llamactl web UI listening on http://%s", addr)
	return http.ListenAndServe(addr, s.mux)
}

// routes wires all HTTP handlers.
func (s *Server) routes() {
	// Static assets (CSS, JS)
	s.mux.Handle("/static/", http.FileServer(http.FS(embeddedFS)))

	// Dashboard
	s.mux.HandleFunc("/", s.handleIndex)

	// ── API: llama-swap ───────────────────────────────────────────────────
	s.mux.HandleFunc("/api/status", s.handleStatus)
	s.mux.HandleFunc("/api/llamaswap/start", s.handleLlamaSwapStart)
	s.mux.HandleFunc("/api/llamaswap/stop", s.handleLlamaSwapStop)
	s.mux.HandleFunc("/api/llamaswap/restart", s.handleLlamaSwapRestart)

	// ── API: ComfyUI ──────────────────────────────────────────────────────
	s.mux.HandleFunc("/api/comfyui/start", s.handleComfyUIStart)
	s.mux.HandleFunc("/api/comfyui/stop", s.handleComfyUIStop)

	// ── API: models + logs ────────────────────────────────────────────────
	s.mux.HandleFunc("/api/models", s.handleModels)
	s.mux.HandleFunc("/api/logs", s.handleLogs)
}
