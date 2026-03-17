package web

import (
	"bufio"
	"encoding/json"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/andermurias/llamactl/internal/modelmanager"
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

func getOnly(w http.ResponseWriter, r *http.Request) bool {
	if r.Method != http.MethodGet {
		http.Error(w, "GET only", http.StatusMethodNotAllowed)
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
	if !getOnly(w, r) {
		return
	}
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
	if !getOnly(w, r) {
		return
	}
	jsonOK(w, service.GetModelsInfo(s.cfg))
}

// ── Logs ──────────────────────────────────────────────────────────────────────

// handleLogs returns the last N lines of a log file.
// Query params: service=llamaswap|comfyui (default: llamaswap), lines=N (default: 100)
func (s *Server) handleLogs(w http.ResponseWriter, r *http.Request) {
	if !getOnly(w, r) {
		return
	}
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

// ── HuggingFace search ────────────────────────────────────────────────────────

// handleHFSearch proxies a HuggingFace model search request.
// GET /api/hf/search?q=<query>&type=<text-generation|mlx|gguf>&limit=<n>
func (s *Server) handleHFSearch(w http.ResponseWriter, r *http.Request) {
	if !getOnly(w, r) {
		return
	}
	query := strings.TrimSpace(r.URL.Query().Get("q"))
	if query == "" {
		jsonErr(w, http.StatusBadRequest, "q parameter is required")
		return
	}
	modelType := r.URL.Query().Get("type") // "", "mlx", "gguf", "text-generation"
	limit := 20
	if n, err := strconv.Atoi(r.URL.Query().Get("limit")); err == nil && n > 0 && n <= 50 {
		limit = n
	}

	results, err := modelmanager.SearchModels(query, modelType, limit)
	if err != nil {
		jsonErr(w, http.StatusBadGateway, "HuggingFace API: "+err.Error())
		return
	}
	jsonOK(w, map[string]any{"results": results, "count": len(results)})
}

// handleHFInfo returns info + MLX detection for a single HF model.
// GET /api/hf/info?id=<hf-model-id>
func (s *Server) handleHFInfo(w http.ResponseWriter, r *http.Request) {
	if !getOnly(w, r) {
		return
	}
	hfID := modelmanager.NormalizeHFID(r.URL.Query().Get("id"))
	if hfID == "" {
		jsonErr(w, http.StatusBadRequest, "id parameter is required")
		return
	}

	model, err := modelmanager.GetModelInfo(hfID)
	if err != nil {
		jsonErr(w, http.StatusBadGateway, "HuggingFace API: "+err.Error())
		return
	}
	if model == nil {
		jsonErr(w, http.StatusNotFound, "model not found: "+hfID)
		return
	}

	mlxInfo := modelmanager.FindMLXVariant(hfID)
	suggestedID := modelmanager.DeriveModelID(hfID)

	jsonOK(w, map[string]any{
		"model":        model,
		"mlx":          mlxInfo,
		"suggested_id": suggestedID,
	})
}

// ── Model install ─────────────────────────────────────────────────────────────

// handleModelInstall installs a HuggingFace model into the stack.
// POST /api/models/install
// Body: modelmanager.InstallRequest (JSON)
// Response: streaming text/plain lines with progress, ending in JSON result.
func (s *Server) handleModelInstall(w http.ResponseWriter, r *http.Request) {
	if !postOnly(w, r) {
		return
	}

	var req modelmanager.InstallRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonErr(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}

	// Use SSE-style streaming so the browser can show progress in real time.
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Accel-Buffering", "no")
	flusher, canFlush := w.(http.Flusher)

	sendLine := func(msg string) {
		_, _ = w.Write([]byte("data: " + msg + "\n\n"))
		if canFlush {
			flusher.Flush()
		}
	}

	ins := modelmanager.NewInstaller(s.cfg.AIDir, s.cfg.ConfigFile, s.cfg.LogDir)
	result, err := ins.Install(req, sendLine)
	if err != nil {
		sendLine("ERROR: " + err.Error())
		return
	}

	// Send final JSON result as the last SSE event
	resultJSON, _ := json.Marshal(result)
	sendLine("RESULT:" + string(resultJSON))
}

// ── Model management (enable / disable / remove) ──────────────────────────────

// handleModelManage performs enable, disable, or remove on an installed model.
// POST /api/models/manage
// Body: modelmanager.ManageRequest (JSON)
func (s *Server) handleModelManage(w http.ResponseWriter, r *http.Request) {
	if !postOnly(w, r) {
		return
	}

	var req modelmanager.ManageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonErr(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	if req.ModelID == "" {
		jsonErr(w, http.StatusBadRequest, "model_id is required")
		return
	}

	mgr := modelmanager.NewManager(s.cfg.ConfigFile, s.cfg.DisabledFile, s.cfg.ModelsDir)

	var err error
	switch req.Action {
	case modelmanager.ActionEnable:
		err = mgr.Enable(req.ModelID)
	case modelmanager.ActionDisable:
		err = mgr.Disable(req.ModelID)
	case modelmanager.ActionRemove:
		err = mgr.Remove(req.ModelID, req.DeleteFiles)
	default:
		jsonErr(w, http.StatusBadRequest, "action must be enable|disable|remove")
		return
	}

	if err != nil {
		jsonErr(w, http.StatusInternalServerError, err.Error())
		return
	}

	jsonOK(w, map[string]any{"ok": true, "model_id": req.ModelID, "action": req.Action})
}

// handleModelDisabledList returns the list of disabled models.
// GET /api/models/disabled
func (s *Server) handleModelDisabledList(w http.ResponseWriter, r *http.Request) {
	if !getOnly(w, r) {
		return
	}
	mgr := modelmanager.NewManager(s.cfg.ConfigFile, s.cfg.DisabledFile, s.cfg.ModelsDir)
	disabled, err := mgr.ListDisabled()
	if err != nil {
		jsonErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	jsonOK(w, map[string]any{"disabled": disabled})
}
