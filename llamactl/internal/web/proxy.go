package web

// aiProxy provides a transparent reverse proxy from llamactl-web (port 3333) to
// llama-swap (port 8080) with one critical addition: SSE keepalive injection.
//
// Root cause of 90-second timeouts
// ----------------------------------
// Go's http.DefaultTransport has IdleConnTimeout: 90s. More importantly, during
// the *prefill* phase of LLM inference (processing the input tokens before any
// output is generated), the upstream (mlx_lm.server or llama-server) sends NO
// data to the client. For large contexts (10-50K tokens), prefill can take
// 30-120+ seconds. If the client has a "time-to-first-byte" timeout of ~90s,
// it disconnects before the first token is generated.
//
// sendLoadingState: true in llama-swap only injects heartbeats while the MODEL
// IS LOADING, not during the prefill phase of already-loaded models. So even
// with sendLoadingState enabled, warm-model long-context requests can still
// trigger the timeout.
//
// Fix
// ---
// This proxy wraps the http.ResponseWriter and starts a goroutine that writes
// `: keepalive N\n\n` (a valid SSE comment, ignored by all clients) every 15s
// while no real data has arrived from upstream. As soon as the first token
// arrives, the goroutine stops and the real SSE stream takes over.
//
// Usage: point your AI client at http://<mac-ip>:3333/v1/ instead of :8080/v1/

import (
	"fmt"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"sync"
	"time"
)

const (
	proxyKeepaliveInterval = 15 * time.Second
	proxyBackendTimeout    = 300 * time.Second // transport-level timeout for upstream
)

// handleAIProxy forwards /v1/* → llama-swap with SSE keepalive injection.
func (s *Server) handleAIProxy(w http.ResponseWriter, r *http.Request) {
	// Derive llama-swap base URL from config ("0.0.0.0:8080" → "http://localhost:8080")
	_, port, _ := net.SplitHostPort(s.cfg.Listen)
	if port == "" {
		port = "8080"
	}
	targetURL, err := url.Parse("http://127.0.0.1:" + port)
	if err != nil {
		http.Error(w, "proxy misconfigured", http.StatusInternalServerError)
		return
	}

	proxy := httputil.NewSingleHostReverseProxy(targetURL)

	// Custom transport: no ResponseHeaderTimeout (models can take minutes to
	// start), extended IdleConnTimeout (well above the default 90s).
	proxy.Transport = &http.Transport{
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		ResponseHeaderTimeout: 0,                  // unlimited — model may load for 175s
		IdleConnTimeout:       proxyBackendTimeout, // 300s > any reasonable inference time
		DisableKeepAlives:     false,
	}

	// Wrap the response writer to inject SSE keepalives during silent prefill.
	kw := newKeepaliveWriter(w)
	defer kw.stop()

	proxy.ServeHTTP(kw, r)
}

// ─── keepaliveWriter ───────────────────────────────────────────────────────

// keepaliveWriter wraps http.ResponseWriter and injects an SSE comment every
// proxyKeepaliveInterval while the upstream hasn't sent any data yet.
// Once the first byte of the response body arrives, keepalive stops permanently.
type keepaliveWriter struct {
	http.ResponseWriter
	flusher http.Flusher

	mu      sync.Mutex
	isSSE   bool   // set in WriteHeader when Content-Type is text/event-stream
	active  bool   // true until first Write() call
	seq     int    // keepalive comment counter (for debugging)
	done    chan struct{}
	stopped bool
}

func newKeepaliveWriter(w http.ResponseWriter) *keepaliveWriter {
	kw := &keepaliveWriter{
		ResponseWriter: w,
		done:           make(chan struct{}),
		active:         true,
	}
	if f, ok := w.(http.Flusher); ok {
		kw.flusher = f
	}
	go kw.run()
	return kw
}

// WriteHeader captures the Content-Type before forwarding the status code.
func (kw *keepaliveWriter) WriteHeader(code int) {
	ct := kw.ResponseWriter.Header().Get("Content-Type")
	kw.mu.Lock()
	kw.isSSE = strings.Contains(ct, "text/event-stream")
	kw.mu.Unlock()
	kw.ResponseWriter.WriteHeader(code)
}

// Write stops the keepalive loop on first call, then forwards the bytes.
// All subsequent Write calls bypass the mutex (active=false and stopped=true).
func (kw *keepaliveWriter) Write(b []byte) (int, error) {
	kw.mu.Lock()
	kw.active = false
	kw.mu.Unlock()
	return kw.ResponseWriter.Write(b)
}

// Flush propagates flush calls (needed for SSE).
func (kw *keepaliveWriter) Flush() {
	if kw.flusher != nil {
		kw.flusher.Flush()
	}
}

// stop signals the keepalive goroutine to exit (called via defer in handleAIProxy).
func (kw *keepaliveWriter) stop() {
	kw.mu.Lock()
	if !kw.stopped {
		kw.stopped = true
		close(kw.done)
	}
	kw.mu.Unlock()
}

// run is the keepalive goroutine. It sends `: keepalive N\n\n` SSE comments
// while the upstream is silent (active=true, no Write calls received yet).
func (kw *keepaliveWriter) run() {
	ticker := time.NewTicker(proxyKeepaliveInterval)
	defer ticker.Stop()

	for {
		select {
		case <-kw.done:
			return
		case <-ticker.C:
			kw.mu.Lock()
			if !kw.active || !kw.isSSE || kw.flusher == nil {
				kw.mu.Unlock()
				// Still tick; active becomes false once first real write arrives
				continue
			}
			kw.seq++
			// SSE comment — all spec-compliant SSE clients ignore this line.
			fmt.Fprintf(kw.ResponseWriter, ": keepalive %d/%d\n\n", kw.seq, 20)
			kw.flusher.Flush()
			kw.mu.Unlock()
		}
	}
}
