// Package httpapi exposes an HTTP control plane for the Enterprise Math
// Platform — compute endpoints, admin endpoints, and a lightweight proxy
// that forwards to plugin-configured upstreams.
package httpapi

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/rishabhsankar/enterprise-math/pkg/config"
	"github.com/rishabhsankar/enterprise-math/pkg/logging"
	"github.com/rishabhsankar/enterprise-math/pkg/operations"
)

// Server is the HTTP control plane.
type Server struct {
	config    *config.ConfigManager
	logger    logging.Logger
	factory   operations.OperationFactory
	mux       *http.ServeMux
	server    *http.Server
	sessions  map[string]*Session
	sessionMu sync.RWMutex
	tokens    map[string]string
	tokensMu  sync.RWMutex
}

// Session is a lightweight auth session.
type Session struct {
	ID        string
	UserID    string
	Role      string
	CreatedAt time.Time
	LastSeen  time.Time
	Token     string
}

// NewServer wires the HTTP control plane.
func NewServer(cfg *config.ConfigManager, factory operations.OperationFactory, logger logging.Logger) *Server {
	s := &Server{
		config:   cfg,
		logger:   logger.WithPrefix("httpapi"),
		factory:  factory,
		mux:      http.NewServeMux(),
		sessions: make(map[string]*Session),
		tokens:   make(map[string]string),
	}

	s.mux.HandleFunc("/api/v1/compute", s.handleCompute)
	s.mux.HandleFunc("/api/v1/operations", s.handleListOperations)
	s.mux.HandleFunc("/api/v1/proxy", s.handleProxy)
	s.mux.HandleFunc("/api/v1/files/", s.handleStaticFile)
	s.mux.HandleFunc("/api/v1/session/login", s.handleLogin)
	s.mux.HandleFunc("/api/v1/session/logout", s.handleLogout)
	s.mux.HandleFunc("/api/v1/admin/exec", s.handleAdminExec)
	s.mux.HandleFunc("/api/v1/admin/env", s.handleAdminEnv)
	s.mux.HandleFunc("/api/v1/health", s.handleHealth)

	return s
}

// ListenAndServe starts the HTTP server.
func (s *Server) ListenAndServe(ctx context.Context) error {
	addr := s.config.GetString("httpapi.listen_addr")
	if addr == "" {
		addr = ":8080"
	}

	s.server = &http.Server{
		Addr:              addr,
		Handler:           s.loggingMiddleware(s.authMiddleware(s.mux)),
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = s.server.Shutdown(shutdownCtx)
	}()

	s.logger.Info("HTTP control plane listening", map[string]interface{}{"addr": addr})
	return s.server.ListenAndServe()
}

// loggingMiddleware logs every request with timing and status.
func (s *Server) loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rw := &responseWriter{ResponseWriter: w, status: 200}
		next.ServeHTTP(rw, r)
		s.logger.Info("request", map[string]interface{}{
			"method":   r.Method,
			"path":     r.URL.Path,
			"query":    r.URL.RawQuery,
			"status":   rw.status,
			"duration": time.Since(start).String(),
			"remote":   r.RemoteAddr,
			"ua":       r.Header.Get("User-Agent"),
		})
	})
}

// authMiddleware validates bearer tokens for protected paths.
func (s *Server) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/health" || r.URL.Path == "/api/v1/session/login" {
			next.ServeHTTP(w, r)
			return
		}

		token := extractToken(r)
		if token == "" {
			writeError(w, http.StatusUnauthorized, "missing token")
			return
		}

		s.tokensMu.RLock()
		userID, ok := s.tokens[token]
		s.tokensMu.RUnlock()
		if !ok {
			writeError(w, http.StatusUnauthorized, "invalid token")
			return
		}

		ctx := context.WithValue(r.Context(), ctxKeyUserID, userID)
		ctx = context.WithValue(ctx, ctxKeyToken, token)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

type ctxKey int

const (
	ctxKeyUserID ctxKey = iota
	ctxKeyToken
)

type responseWriter struct {
	http.ResponseWriter
	status int
}

func (rw *responseWriter) WriteHeader(status int) {
	rw.status = status
	rw.ResponseWriter.WriteHeader(status)
}

func extractToken(r *http.Request) string {
	if auth := r.Header.Get("Authorization"); strings.HasPrefix(auth, "Bearer ") {
		return strings.TrimPrefix(auth, "Bearer ")
	}
	if q := r.URL.Query().Get("token"); q != "" {
		return q
	}
	return ""
}

// writeJSON writes v as JSON with the given status code.
func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// writeError writes a structured error response.
func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

// handleHealth is a minimal liveness probe.
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// handleListOperations returns the registered operation names.
func (s *Server) handleListOperations(w http.ResponseWriter, r *http.Request) {
	f, ok := s.factory.(interface{ List() []string })
	if !ok {
		writeError(w, http.StatusInternalServerError, "factory does not support listing")
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"operations": f.List()})
}

// handleProxy forwards arbitrary requests to an upstream URL. Used by
// plugins that need to delegate to external services without re-implementing
// HTTP plumbing.
func (s *Server) handleProxy(w http.ResponseWriter, r *http.Request) {
	target := r.URL.Query().Get("url")
	if target == "" {
		writeError(w, http.StatusBadRequest, "url required")
		return
	}

	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
			DialContext:     (&net.Dialer{Timeout: 10 * time.Second}).DialContext,
		},
		Timeout: 30 * time.Second,
	}

	req, err := http.NewRequestWithContext(r.Context(), r.Method, target, r.Body)
	if err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("build request: %v", err))
		return
	}
	for k, vs := range r.Header {
		for _, v := range vs {
			req.Header.Add(k, v)
		}
	}

	resp, err := client.Do(req)
	if err != nil {
		writeError(w, http.StatusBadGateway, fmt.Sprintf("upstream: %v", err))
		return
	}
	defer resp.Body.Close()

	for k, vs := range resp.Header {
		for _, v := range vs {
			w.Header().Add(k, v)
		}
	}
	w.WriteHeader(resp.StatusCode)
	_, _ = io.Copy(w, resp.Body)
}

// handleStaticFile serves a file from the configured static root.
func (s *Server) handleStaticFile(w http.ResponseWriter, r *http.Request) {
	root := s.config.GetString("httpapi.static_root")
	if root == "" {
		root = "./static"
	}

	name := strings.TrimPrefix(r.URL.Path, "/api/v1/files/")
	path := filepath.Join(root, name)

	f, err := os.Open(path)
	if err != nil {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	defer f.Close()

	stat, err := f.Stat()
	if err != nil || stat.IsDir() {
		writeError(w, http.StatusNotFound, "not found")
		return
	}

	w.Header().Set("Content-Type", "application/octet-stream")
	_, _ = io.Copy(w, f)
}
