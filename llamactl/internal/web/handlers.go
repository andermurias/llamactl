package web

import (
	"bufio"
	"encoding/json"
	"net/http"
	"os"
	"strconv"

	"github.com/andermurias/llamactl/internal/service"
)

// ── helpers ──────────────────────────────────────────────────────────────────

func jsonOK(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

func jsonErr(w http.ResponseWriter, code int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

func postOnly(w http.ResponseWriter, r *http.Request) bool {
	if r.Method != http.MethodPost {
		http.Error(w, "POST only", http.StatusMethodNotAllowed)
		return false
	}
	return true
}

// ── Dashboard ─────────────────────────────────────────────────────────────────

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	lsStatus := service.GetStatus(s.cfg)
	cuStatus := service.GetComfyUIStatus(s.cfg)
	models := service.GetModelsInfo(s.cfg)

	data := map[string]any{
		"LlamaSwap":  lsStatus,
		"ComfyUI":    cuStatus,
		"Models":     models,
		"ComfyPort":  s.cfg.ComfyUIPort,
		"ConfigFile": s.cfg.ConfigFile,
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := s.tmpl.ExecuteTemplate(w, "index.html", data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// ── Status API ────────────────────────────────────────────────────────────────

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	jsonOK(w, map[string]any{
		"llamaswap": service.GetStatus(s.cfg),
		"comfyui":   service.GetComfyUIStatus(s.cfg),
	})
}

// ── llama-swap actions ────────────────────────────────────────────────────────

func (s *Server) handleLlamaSwapStart(w http.ResponseWriter, r *http.Request) {
	if !postOnly(w, r) {
		return
	}
	pid, err := service.Start(s.cfg)
	if err != nil {
		jsonErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	jsonOK(w, map[string]any{"ok": true, "pid": pid})
}

func (s *Server) handleLlamaSwapStop(w http.ResponseWriter, r *http.Request) {
	if !postOnly(w, r) {
		return
	}
	if err := service.Stop(s.cfg); err != nil {
		jsonErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	jsonOK(w, map[string]bool{"ok": true})
}

func (s *Server) handleLlamaSwapRestart(w http.ResponseWriter, r *http.Request) {
	if !postOnly(w, r) {
		return
	}
	_ = service.Stop(s.cfg)
	pid, err := service.Start(s.cfg)
	if err != nil {
		jsonErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	jsonOK(w, map[string]any{"ok": true, "pid": pid})
}

// ── ComfyUI actions ───────────────────────────────────────────────────────────

func (s *Server) handleComfyUIStart(w http.ResponseWriter, r *http.Request) {
	if !postOnly(w, r) {
		return
	}
	pid, err := service.StartComfyUI(s.cfg)
	if err != nil {
		jsonErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	jsonOK(w, map[string]any{"ok": true, "pid": pid})
}

func (s *Server) handleComfyUIStop(w http.ResponseWriter, r *http.Request) {
	if !postOnly(w, r) {
		return
	}
	if err := service.StopComfyUI(s.cfg); err != nil {
		jsonErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	jsonOK(w, map[string]bool{"ok": true})
}

// ── Models ────────────────────────────────────────────────────────────────────

func (s *Server) handleModels(w http.ResponseWriter, r *http.Request) {
	jsonOK(w, service.GetModelsInfo(s.cfg))
}

// ── Logs ──────────────────────────────────────────────────────────────────────

// handleLogs returns the last N lines of a log file.
// Query params: service=llamaswap|comfyui (default: llamaswap), lines=N (default: 100)
func (s *Server) handleLogs(w http.ResponseWriter, r *http.Request) {
	svcName := r.URL.Query().Get("service")
	if svcName == "" {
		svcName = "llamaswap"
	}
	lines := 100
	if n, err := strconv.Atoi(r.URL.Query().Get("lines")); err == nil && n > 0 {
		lines = n
	}

	logPath := s.cfg.LogFile
	if svcName == "comfyui" {
		logPath = s.cfg.ComfyUILog
	}

	tail, err := tailFile(logPath, lines)
	if err != nil {
		jsonOK(w, map[string]any{"lines": []string{}, "error": err.Error()})
		return
	}
	jsonOK(w, map[string]any{"lines": tail, "service": svcName})
}

// tailFile reads the last n lines of a file efficiently.
func tailFile(path string, n int) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var lines []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
		if len(lines) > n {
			lines = lines[1:]
		}
	}
	return lines, scanner.Err()
}

// ── Config ────────────────────────────────────────────────────────────────────

// handleConfig serves GET (read) and POST (write + reload) for llama-swap.yaml.
func (s *Server) handleConfig(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		data, err := os.ReadFile(s.cfg.ConfigFile)
		if err != nil {
			jsonErr(w, http.StatusInternalServerError, err.Error())
			return
		}
		jsonOK(w, map[string]string{"content": string(data), "path": s.cfg.ConfigFile})

	case http.MethodPost:
		var body struct {
			Content string `json:"content"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			jsonErr(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
			return
		}
		if err := os.WriteFile(s.cfg.ConfigFile, []byte(body.Content), 0o644); err != nil {
			jsonErr(w, http.StatusInternalServerError, "write failed: "+err.Error())
			return
		}
		jsonOK(w, map[string]bool{"ok": true})

	default:
		http.Error(w, "GET or POST only", http.StatusMethodNotAllowed)
	}
}
