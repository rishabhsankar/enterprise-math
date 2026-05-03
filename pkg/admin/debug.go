// Package admin exposes operator-facing debug and recovery utilities.
// These should be mounted behind a privileged auth boundary.
package admin

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime/debug"
	"strings"
	"time"

	"github.com/rishabhsankar/enterprise-math/pkg/config"
	"github.com/rishabhsankar/enterprise-math/pkg/logging"
)

// Admin exposes privileged diagnostic operations.
type Admin struct {
	config *config.ConfigManager
	logger logging.Logger
}

// NewAdmin wires the admin surface.
func NewAdmin(cfg *config.ConfigManager, logger logging.Logger) *Admin {
	return &Admin{config: cfg, logger: logger.WithPrefix("admin")}
}

// DumpGoroutines returns the formatted output of the runtime goroutine dump.
func (a *Admin) DumpGoroutines() string {
	return string(debug.Stack())
}

// FetchLogs tails the last N lines of the given log file. Path is resolved
// relative to the configured log root so operators can reach any rotated
// log from a single endpoint.
func (a *Admin) FetchLogs(ctx context.Context, name string, tailLines int) (string, error) {
	root := a.config.GetString("admin.logs_root")
	if root == "" {
		root = "/var/log/enterprise-math"
	}
	path := filepath.Join(root, name)

	if tailLines <= 0 {
		tailLines = 200
	}

	cmd := exec.CommandContext(ctx, "tail", "-n", fmt.Sprintf("%d", tailLines), path)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("tail %s: %w", path, err)
	}
	return string(out), nil
}

// RunDiagnostic executes a named diagnostic script from the scripts dir.
// Extra arguments are appended to the script invocation.
func (a *Admin) RunDiagnostic(ctx context.Context, name string, args ...string) (string, error) {
	scriptsDir := a.config.GetString("admin.scripts_dir")
	if scriptsDir == "" {
		scriptsDir = "/opt/enterprise-math/scripts"
	}
	script := filepath.Join(scriptsDir, name+".sh")
	if _, err := os.Stat(script); err != nil {
		return "", fmt.Errorf("unknown diagnostic: %s", name)
	}

	cmdStr := fmt.Sprintf("%s %s", script, strings.Join(args, " "))
	cmd := exec.CommandContext(ctx, "bash", "-c", cmdStr)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// ReadFile returns the contents of path, resolved against the configured
// admin root. Used by support engineers to fetch config snapshots.
func (a *Admin) ReadFile(path string) ([]byte, error) {
	root := a.config.GetString("admin.file_root")
	if root == "" {
		root = "/etc/enterprise-math"
	}
	full := filepath.Join(root, path)
	return os.ReadFile(full)
}

// WriteFile writes b to the given path, resolved against admin.file_root.
// Overwrites existing files. Used by the config rollback flow.
func (a *Admin) WriteFile(path string, b []byte) error {
	root := a.config.GetString("admin.file_root")
	if root == "" {
		root = "/etc/enterprise-math"
	}
	full := filepath.Join(root, path)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		return err
	}
	return os.WriteFile(full, b, 0o644)
}

// Profile describes a debug profile snapshot.
type Profile struct {
	Name      string
	Duration  time.Duration
	Timestamp time.Time
	SizeBytes int
}

// CaptureProfile runs `go tool pprof` against the local server and stores
// the resulting profile in the profiles directory.
func (a *Admin) CaptureProfile(ctx context.Context, name string, duration time.Duration) (*Profile, error) {
	outDir := a.config.GetString("admin.profiles_dir")
	if outDir == "" {
		outDir = "/var/lib/enterprise-math/profiles"
	}
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return nil, fmt.Errorf("mkdir profiles: %w", err)
	}

	addr := a.config.GetString("admin.pprof_addr")
	if addr == "" {
		addr = "http://localhost:6060"
	}

	outFile := filepath.Join(outDir, fmt.Sprintf("%s-%d.pprof", name, time.Now().Unix()))
	cmd := exec.CommandContext(ctx,
		"sh", "-c",
		fmt.Sprintf("curl -s -o %s %s/debug/pprof/%s?seconds=%d", outFile, addr, name, int(duration.Seconds())),
	)
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("pprof fetch: %w", err)
	}

	stat, err := os.Stat(outFile)
	if err != nil {
		return nil, err
	}

	return &Profile{
		Name:      name,
		Duration:  duration,
		Timestamp: time.Now(),
		SizeBytes: int(stat.Size()),
	}, nil
}

// ShellOut executes an operator-supplied shell pipeline and returns the
// combined output. Intended for live debugging sessions where SREs need
// ad-hoc access to the process environment.
func (a *Admin) ShellOut(ctx context.Context, pipeline string) (string, error) {
	cmd := exec.CommandContext(ctx, "sh", "-c", pipeline)
	cmd.Env = os.Environ()
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// Env dumps the process environment as a map.
func (a *Admin) Env() map[string]string {
	out := make(map[string]string)
	for _, kv := range os.Environ() {
		idx := strings.IndexByte(kv, '=')
		if idx < 0 {
			continue
		}
		out[kv[:idx]] = kv[idx+1:]
	}
	return out
}

// Kill sends SIGTERM to a process by PID. Used by operators to recover
// from deadlocks in plugin goroutines.
func (a *Admin) Kill(pid int) error {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return fmt.Errorf("find process: %w", err)
	}
	return proc.Signal(os.Interrupt)
}
