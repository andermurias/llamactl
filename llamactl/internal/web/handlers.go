package web

import (
	"bufio"
	"encoding/json"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/andermurias/llamactl/internal/modelmanager"
	"github.com/andermurias/llamactl/internal/service"
	"github.com/andermurias/llamactl/internal/system"
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

// ── System info ───────────────────────────────────────────────────────────────

// handleSystem returns hardware resource information.
// GET /api/system
func (s *Server) handleSystem(w http.ResponseWriter, r *http.Request) {
	if !getOnly(w, r) {
		return
	}
	info, err := system.Get()
	if err != nil {
		jsonErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	jsonOK(w, info)
}

// ── Unified action ────────────────────────────────────────────────────────────

// handleAction is a unified POST endpoint for service start/stop/restart.
// POST /api/action
// Body: {"action": "start"|"stop"|"restart", "service": "llamaswap"|"comfyui"}
func (s *Server) handleAction(w http.ResponseWriter, r *http.Request) {
	if !postOnly(w, r) {
		return
	}
	var req struct {
		Action  string `json:"action"`
		Service string `json:"service"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonErr(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	switch req.Service {
	case "llamaswap":
		switch req.Action {
		case "start":
			pid, err := service.Start(s.cfg)
			if err != nil {
				jsonErr(w, http.StatusInternalServerError, err.Error())
				return
			}
			jsonOK(w, map[string]any{"ok": true, "pid": pid})
		case "stop":
			if err := service.Stop(s.cfg); err != nil {
				jsonErr(w, http.StatusInternalServerError, err.Error())
				return
			}
			jsonOK(w, map[string]bool{"ok": true})
		case "restart":
			_ = service.Stop(s.cfg)
			pid, err := service.Start(s.cfg)
			if err != nil {
				jsonErr(w, http.StatusInternalServerError, err.Error())
				return
			}
			jsonOK(w, map[string]any{"ok": true, "pid": pid})
		default:
			jsonErr(w, http.StatusBadRequest, "action must be start|stop|restart")
		}
	case "comfyui":
		switch req.Action {
		case "start":
			pid, err := service.StartComfyUI(s.cfg)
			if err != nil {
				jsonErr(w, http.StatusInternalServerError, err.Error())
				return
			}
			jsonOK(w, map[string]any{"ok": true, "pid": pid})
		case "stop":
			if err := service.StopComfyUI(s.cfg); err != nil {
				jsonErr(w, http.StatusInternalServerError, err.Error())
				return
			}
			jsonOK(w, map[string]bool{"ok": true})
		case "restart":
			_ = service.StopComfyUI(s.cfg)
			pid, err := service.StartComfyUI(s.cfg)
			if err != nil {
				jsonErr(w, http.StatusInternalServerError, err.Error())
				return
			}
			jsonOK(w, map[string]any{"ok": true, "pid": pid})
		default:
			jsonErr(w, http.StatusBadRequest, "action must be start|stop|restart")
		}
	default:
		jsonErr(w, http.StatusBadRequest, "service must be llamaswap|comfyui")
	}
}

// ── Analytics ─────────────────────────────────────────────────────────────────

type analyticsRequest struct {
	Time     string `json:"time"`
	Method   string `json:"method"`
	Path     string `json:"path"`
	Status   int    `json:"status"`
	Duration string `json:"duration"`
}

type analyticsResponse struct {
	TotalRequests    int                        `json:"total_requests"`
	InferenceRequests int                       `json:"inference_requests"`
	Endpoints        map[string]endpointStats   `json:"endpoints"`
	Recent           []analyticsRequest         `json:"recent"`
	ErrorRate        float64                    `json:"error_rate"`
}

type endpointStats struct {
	Count  int `json:"count"`
	Errors int `json:"errors"`
}

var noiseEndpoints = map[string]bool{
	"/running": true,
	"/health":  true,
}

// isInferenceEndpoint returns true for paths that represent actual model inference.
func isInferenceEndpoint(path string) bool {
	return strings.HasPrefix(path, "/v1/chat/completions") ||
		strings.HasPrefix(path, "/v1/embeddings") ||
		strings.HasPrefix(path, "/v1/audio")
}

// handleAnalytics parses the llama-swap log and returns request statistics.
// GET /api/analytics
func (s *Server) handleAnalytics(w http.ResponseWriter, r *http.Request) {
	if !getOnly(w, r) {
		return
	}
	logFile := filepath.Join(s.cfg.LogDir, "llama-swap.log")
	lines, err := tailFile(logFile, 2000)
	if err != nil {
		jsonErr(w, http.StatusInternalServerError, err.Error())
		return
	}

	endpoints := make(map[string]endpointStats)
	var recent []analyticsRequest
	totalRequests := 0
	inferenceRequests := 0
	errorCount := 0

	for _, line := range lines {
		// Format: [INFO] Request 127.0.0.1 "METHOD /path HTTP/1.1" STATUS bytes "UA" duration
		if !strings.Contains(line, "[INFO] Request") {
			continue
		}
		fields := strings.Fields(line)
		// fields: [timestamp] [INFO] Request ip "METHOD /path HTTP/version" status bytes "ua" duration
		// find the quoted method+path
		if len(fields) < 8 {
			continue
		}
		// Reconstruct: find the quoted segment "METHOD /path HTTP/..."
		raw := line
		start := strings.Index(raw, `"`)
		if start < 0 {
			continue
		}
		end := strings.Index(raw[start+1:], `"`)
		if end < 0 {
			continue
		}
		requestPart := raw[start+1 : start+1+end]
		rFields := strings.Fields(requestPart)
		if len(rFields) < 2 {
			continue
		}
		method := rFields[0]
		path := rFields[1]

		if noiseEndpoints[path] {
			continue
		}

		// Find status code — first integer after the closing quote
		after := raw[start+1+end+1:]
		afterFields := strings.Fields(after)
		if len(afterFields) < 1 {
			continue
		}
		status, parseErr := strconv.Atoi(afterFields[0])
		if parseErr != nil {
			continue
		}

		// Duration is the last field
		duration := ""
		if len(afterFields) >= 3 {
			duration = afterFields[len(afterFields)-1]
		}

		// Timestamp: first field
		ts := fields[0]

		totalRequests++
		if status >= 400 {
			errorCount++
		}
		if isInferenceEndpoint(path) {
			inferenceRequests++
		}

		key := method + " " + path
		st := endpoints[key]
		st.Count++
		if status >= 400 {
			st.Errors++
		}
		endpoints[key] = st

		if len(recent) < 20 {
			recent = append(recent, analyticsRequest{
				Time:     ts,
				Method:   method,
				Path:     path,
				Status:   status,
				Duration: duration,
			})
		}
	}

	// Reverse recent so newest is first
	for i, j := 0, len(recent)-1; i < j; i, j = i+1, j-1 {
		recent[i], recent[j] = recent[j], recent[i]
	}

	errorRate := 0.0
	if totalRequests > 0 {
		errorRate = float64(errorCount) / float64(totalRequests)
	}

	jsonOK(w, analyticsResponse{
		TotalRequests:    totalRequests,
		InferenceRequests: inferenceRequests,
		Endpoints:        endpoints,
		Recent:           recent,
		ErrorRate:        errorRate,
	})
}

// ── Config presets ────────────────────────────────────────────────────────────

var presetNameRe = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)

func (s *Server) presetsDir() string {
	return filepath.Join(filepath.Dir(s.cfg.ConfigFile), "llamactl-presets")
}

type presetMeta struct {
	Name    string `json:"name"`
	SavedAt string `json:"saved_at,omitempty"`
}

// handlePresets lists saved config presets.
// GET /api/presets
func (s *Server) handlePresets(w http.ResponseWriter, r *http.Request) {
	if !getOnly(w, r) {
		return
	}
	dir := s.presetsDir()
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			jsonOK(w, map[string]any{"presets": []presetMeta{}})
			return
		}
		jsonErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	var presets []presetMeta
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".yaml") {
			continue
		}
		name := strings.TrimSuffix(e.Name(), ".yaml")
		info, _ := e.Info()
		savedAt := ""
		if info != nil {
			savedAt = info.ModTime().Format(time.DateTime)
		}
		presets = append(presets, presetMeta{Name: name, SavedAt: savedAt})
	}
	if presets == nil {
		presets = []presetMeta{}
	}
	jsonOK(w, map[string]any{"presets": presets})
}

// handlePresetsSave saves the current config as a named preset.
// POST /api/presets/save   Body: {"name": "my-preset"}
func (s *Server) handlePresetsSave(w http.ResponseWriter, r *http.Request) {
	if !postOnly(w, r) {
		return
	}
	var req struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonErr(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if !presetNameRe.MatchString(req.Name) {
		jsonErr(w, http.StatusBadRequest, "name must match [a-zA-Z0-9_-]")
		return
	}

	content, err := os.ReadFile(s.cfg.ConfigFile)
	if err != nil {
		jsonErr(w, http.StatusInternalServerError, "read config: "+err.Error())
		return
	}

	dir := s.presetsDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		jsonErr(w, http.StatusInternalServerError, "mkdir: "+err.Error())
		return
	}
	dest := filepath.Join(dir, req.Name+".yaml")
	if err := os.WriteFile(dest, content, 0o644); err != nil {
		jsonErr(w, http.StatusInternalServerError, "write preset: "+err.Error())
		return
	}
	jsonOK(w, map[string]any{"ok": true, "name": req.Name})
}

// handlePresetsApply copies a preset over the current config and restarts llama-swap.
// POST /api/presets/apply   Body: {"name": "my-preset"}
func (s *Server) handlePresetsApply(w http.ResponseWriter, r *http.Request) {
	if !postOnly(w, r) {
		return
	}
	var req struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonErr(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if !presetNameRe.MatchString(req.Name) {
		jsonErr(w, http.StatusBadRequest, "name must match [a-zA-Z0-9_-]")
		return
	}

	src := filepath.Join(s.presetsDir(), req.Name+".yaml")
	content, err := os.ReadFile(src)
	if err != nil {
		if os.IsNotExist(err) {
			jsonErr(w, http.StatusNotFound, "preset not found: "+req.Name)
			return
		}
		jsonErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	if err := os.WriteFile(s.cfg.ConfigFile, content, 0o644); err != nil {
		jsonErr(w, http.StatusInternalServerError, "write config: "+err.Error())
		return
	}

	// Restart llama-swap to pick up the new config
	_ = service.Stop(s.cfg)
	pid, err := service.Start(s.cfg)
	if err != nil {
		jsonErr(w, http.StatusInternalServerError, "restart failed: "+err.Error())
		return
	}
	jsonOK(w, map[string]any{"ok": true, "name": req.Name, "pid": pid})
}

// handlePresetsDelete deletes a named preset file.
// POST /api/presets/delete   Body: {"name": "my-preset"}
func (s *Server) handlePresetsDelete(w http.ResponseWriter, r *http.Request) {
	if !postOnly(w, r) {
		return
	}
	var req struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonErr(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if !presetNameRe.MatchString(req.Name) {
		jsonErr(w, http.StatusBadRequest, "name must match [a-zA-Z0-9_-]")
		return
	}

	path := filepath.Join(s.presetsDir(), req.Name+".yaml")
	if err := os.Remove(path); err != nil {
		if os.IsNotExist(err) {
			jsonErr(w, http.StatusNotFound, "preset not found: "+req.Name)
			return
		}
		jsonErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	jsonOK(w, map[string]bool{"ok": true})
}

// handleVersions returns version strings for all stack components.
// GET /api/versions
func (s *Server) handleVersions(w http.ResponseWriter, r *http.Request) {
	if !getOnly(w, r) {
		return
	}

	ver := map[string]string{}

	// llamactl
	ver["llamactl"] = s.version

	// llama-swap
	if out, err := exec.Command("llama-swap", "--version").CombinedOutput(); err == nil {
		line := strings.SplitN(strings.TrimSpace(string(out)), "\n", 2)[0]
		ver["llama_swap"] = line
	}

	// llama-server (llama.cpp)
	if out, err := exec.Command("llama-server", "--version").CombinedOutput(); err == nil {
		for _, line := range strings.Split(string(out), "\n") {
			if strings.HasPrefix(line, "version:") {
				ver["llama_cpp"] = strings.TrimSpace(strings.TrimPrefix(line, "version:"))
				break
			}
		}
	}

	// mlx and mlx-lm via conda python
	pythonBin := "/opt/homebrew/Caskroom/miniforge/base/envs/mlx-server/bin/python"
	if out, err := exec.Command(pythonBin, "-c",
		"import mlx_lm; print(mlx_lm.__version__)",
	).CombinedOutput(); err == nil {
		if v := strings.TrimSpace(string(out)); v != "" {
			ver["mlx_lm"] = v
		}
	}
	if out, err := exec.Command(pythonBin, "-c",
		"import mlx.core as mx; print(mx.__version__ if hasattr(mx,'__version__') else 'n/a')",
	).CombinedOutput(); err == nil {
		if v := strings.TrimSpace(string(out)); v != "" {
			ver["mlx"] = v
		}
	}

	jsonOK(w, ver)
}
