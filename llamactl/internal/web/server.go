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
	cfg     *config.Config
	version string
	tmpl    *template.Template
	mux     *http.ServeMux
}

// New creates and configures a new Server.
func New(cfg *config.Config, version string) (*Server, error) {
	tmpl, err := template.ParseFS(embeddedFS, "templates/*.html")
	if err != nil {
		return nil, fmt.Errorf("parse templates: %w", err)
	}

	s := &Server{cfg: cfg, version: version, tmpl: tmpl, mux: http.NewServeMux()}
	s.routes()
	return s, nil
}

// Handler returns the HTTP handler (useful for testing with httptest).
func (s *Server) Handler() http.Handler { return s.mux }

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

	// ── API: models + logs + config ───────────────────────────────────────
	s.mux.HandleFunc("/api/models", s.handleModels)
	s.mux.HandleFunc("/api/models/disabled", s.handleModelDisabledList)
	s.mux.HandleFunc("/api/models/files", s.handleModelFiles)
	s.mux.HandleFunc("/api/models/install", s.handleModelInstall)
	s.mux.HandleFunc("/api/models/manage", s.handleModelManage)
	s.mux.HandleFunc("/api/logs", s.handleLogs)
	s.mux.HandleFunc("/api/config", s.handleConfig)

	// ── API: HuggingFace discovery ────────────────────────────────────────
	s.mux.HandleFunc("/api/hf/search", s.handleHFSearch)
	s.mux.HandleFunc("/api/hf/info", s.handleHFInfo)

	// ── API: system info, unified action, analytics ───────────────────────
	s.mux.HandleFunc("/api/system", s.handleSystem)
	s.mux.HandleFunc("/api/action", s.handleAction)
	s.mux.HandleFunc("/api/analytics", s.handleAnalytics)
	s.mux.HandleFunc("/api/versions", s.handleVersions)
	s.mux.HandleFunc("/api/running", s.handleRunning)
	s.mux.HandleFunc("/api/unload", s.handleUnload)
	s.mux.HandleFunc("/api/memory", s.handleProcessMemory)
	s.mux.HandleFunc("/api/processes", s.handleProcesses)

	// ── API: config presets ───────────────────────────────────────────────
	s.mux.HandleFunc("/api/presets", s.handlePresets)
	s.mux.HandleFunc("/api/presets/save", s.handlePresetsSave)
	s.mux.HandleFunc("/api/presets/apply", s.handlePresetsApply)
	s.mux.HandleFunc("/api/presets/delete", s.handlePresetsDelete)

	// ── AI proxy: transparent forward to llama-swap with SSE keepalive ───
	// Routes all /v1/* and /upstream/* requests to llama-swap (:8080)
	// and injects `: keepalive N\n\n` SSE comments every 15s during the
	// silent prefill phase, preventing client-side timeouts.
	// Use http://<mac>:3333/v1/ instead of :8080/v1/ for timeout-free access.
	s.mux.HandleFunc("/v1/", s.handleAIProxy)
	s.mux.HandleFunc("/upstream/", s.handleAIProxy)
}
