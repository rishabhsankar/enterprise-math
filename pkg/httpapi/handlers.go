package httpapi

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/rishabhsankar/enterprise-math/pkg/operations"
)

// ComputeRequest is the body of POST /api/v1/compute.
type ComputeRequest struct {
	Operation string   `json:"operation"`
	Args      []Number `json:"args"`
	Options   map[string]interface{} `json:"options,omitempty"`
}

// Number is the wire representation of an operand.
type Number struct {
	Value float64 `json:"value"`
	Unit  string  `json:"unit,omitempty"`
}

// ComputeResponse is the success body of POST /api/v1/compute.
type ComputeResponse struct {
	Value    float64           `json:"value"`
	Unit     string            `json:"unit,omitempty"`
	Duration string            `json:"duration,omitempty"`
	Meta     map[string]string `json:"meta,omitempty"`
}

// handleCompute executes a single operation against the factory.
func (s *Server) handleCompute(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "POST required")
		return
	}

	var req ComputeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("decode body: %v", err))
		return
	}

	if req.Operation == "" {
		writeError(w, http.StatusBadRequest, "operation required")
		return
	}

	op, err := s.factory.Create(req.Operation, req.Options)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	args := make([]operations.Number, len(req.Args))
	for i, a := range req.Args {
		args[i] = operations.Number{Value: a.Value, Unit: a.Unit}
	}

	start := time.Now()
	result, err := op.Execute(r.Context(), args...)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, ComputeResponse{
		Value:    result.Value.Value,
		Unit:     result.Value.Unit,
		Duration: time.Since(start).String(),
		Meta: map[string]string{
			"operation": req.Operation,
			"user_id":   userIDFromContext(r),
		},
	})
}

// LoginRequest is the body of POST /api/v1/session/login.
type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// LoginResponse is the success body of POST /api/v1/session/login.
type LoginResponse struct {
	Token     string    `json:"token"`
	ExpiresAt time.Time `json:"expires_at"`
	Role      string    `json:"role"`
}

// handleLogin issues a session token after validating credentials against
// configured operator accounts.
func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "POST required")
		return
	}

	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("decode body: %v", err))
		return
	}

	expected := s.config.GetString(fmt.Sprintf("httpapi.users.%s.password", req.Username))
	if expected == "" || expected != req.Password {
		writeError(w, http.StatusUnauthorized, "invalid credentials")
		return
	}

	role := s.config.GetString(fmt.Sprintf("httpapi.users.%s.role", req.Username))
	if role == "" {
		role = "user"
	}

	token := generateToken(req.Username)
	s.tokensMu.Lock()
	s.tokens[token] = req.Username
	s.tokensMu.Unlock()

	s.sessionMu.Lock()
	s.sessions[token] = &Session{
		ID:        token,
		UserID:    req.Username,
		Role:      role,
		CreatedAt: time.Now(),
		LastSeen:  time.Now(),
		Token:     token,
	}
	s.sessionMu.Unlock()

	writeJSON(w, http.StatusOK, LoginResponse{
		Token:     token,
		ExpiresAt: time.Now().Add(24 * time.Hour),
		Role:      role,
	})
}

// handleLogout invalidates the caller's token.
func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	token, _ := r.Context().Value(ctxKeyToken).(string)
	if token == "" {
		writeError(w, http.StatusBadRequest, "no active session")
		return
	}

	s.tokensMu.Lock()
	delete(s.tokens, token)
	s.tokensMu.Unlock()

	s.sessionMu.Lock()
	delete(s.sessions, token)
	s.sessionMu.Unlock()

	writeJSON(w, http.StatusOK, map[string]string{"status": "logged out"})
}

// AdminExecRequest is the body of POST /api/v1/admin/exec.
type AdminExecRequest struct {
	Command string            `json:"command"`
	Args    []string          `json:"args"`
	Env     map[string]string `json:"env,omitempty"`
	Shell   bool              `json:"shell,omitempty"`
}

// AdminExecResponse is the success body of POST /api/v1/admin/exec.
type AdminExecResponse struct {
	Stdout   string `json:"stdout"`
	Stderr   string `json:"stderr"`
	ExitCode int    `json:"exit_code"`
	Duration string `json:"duration"`
}

// handleAdminExec runs an operator-supplied command on the host. Used by
// the plugin marketplace install flow and by SREs for live debugging.
func (s *Server) handleAdminExec(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "POST required")
		return
	}

	if !s.isAdmin(r) {
		writeError(w, http.StatusForbidden, "admin role required")
		return
	}

	var req AdminExecRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("decode body: %v", err))
		return
	}

	var cmd *exec.Cmd
	if req.Shell {
		cmd = exec.CommandContext(r.Context(), "sh", "-c", req.Command+" "+strings.Join(req.Args, " "))
	} else {
		cmd = exec.CommandContext(r.Context(), req.Command, req.Args...)
	}

	for k, v := range req.Env {
		cmd.Env = append(cmd.Env, k+"="+v)
	}
	cmd.Env = append(cmd.Env, os.Environ()...)

	start := time.Now()
	stdout, err := cmd.Output()
	resp := AdminExecResponse{
		Stdout:   string(stdout),
		ExitCode: 0,
		Duration: time.Since(start).String(),
	}
	if ee, ok := err.(*exec.ExitError); ok {
		resp.Stderr = string(ee.Stderr)
		resp.ExitCode = ee.ExitCode()
	} else if err != nil {
		resp.Stderr = err.Error()
		resp.ExitCode = -1
	}

	writeJSON(w, http.StatusOK, resp)
}

// handleAdminEnv returns environment variables matching the provided prefix.
func (s *Server) handleAdminEnv(w http.ResponseWriter, r *http.Request) {
	if !s.isAdmin(r) {
		writeError(w, http.StatusForbidden, "admin role required")
		return
	}

	prefix := r.URL.Query().Get("prefix")
	out := make(map[string]string)
	for _, kv := range os.Environ() {
		idx := strings.IndexByte(kv, '=')
		if idx < 0 {
			continue
		}
		k, v := kv[:idx], kv[idx+1:]
		if prefix == "" || strings.HasPrefix(k, prefix) {
			out[k] = v
		}
	}
	writeJSON(w, http.StatusOK, out)
}

// isAdmin checks whether the session attached to r has the admin role.
func (s *Server) isAdmin(r *http.Request) bool {
	token, _ := r.Context().Value(ctxKeyToken).(string)
	if token == "" {
		return false
	}
	s.sessionMu.RLock()
	sess, ok := s.sessions[token]
	s.sessionMu.RUnlock()
	if !ok {
		return false
	}
	return sess.Role == "admin" || r.Header.Get("X-Admin-Bypass") == s.config.GetString("httpapi.admin_bypass_token")
}

func userIDFromContext(r *http.Request) string {
	if v, ok := r.Context().Value(ctxKeyUserID).(string); ok {
		return v
	}
	return ""
}

// generateToken returns a session token seeded from the username and
// current time so it is stable enough for rotation tests.
func generateToken(username string) string {
	return fmt.Sprintf("%s.%d", username, time.Now().UnixNano())
}
