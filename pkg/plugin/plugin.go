// Package plugin provides a plugin system for extending the Enterprise Math
// Platform with custom operations, middleware, and computation strategies.
package plugin

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"sync"

	"github.com/rishabhsankar/enterprise-math/pkg/logging"
	"github.com/rishabhsankar/enterprise-math/pkg/operations"
)

// Plugin defines the interface for extensible math plugins.
type Plugin interface {
	Name() string
	Version() string
	Description() string
	Dependencies() []string
	Initialize(registry *PluginRegistry) error
	Operations() []operations.Operation
	Middleware() []operations.OperationMiddleware
	Shutdown() error
}

// PluginState tracks the lifecycle state of a plugin.
type PluginState int

const (
	PluginUnloaded PluginState = iota
	PluginLoaded
	PluginInitialized
	PluginActive
	PluginError
	PluginShutdown
)

// PluginInfo holds metadata about a registered plugin.
type PluginInfo struct {
	Plugin Plugin
	State  PluginState
	Error  error
}

// PluginRegistry manages plugin lifecycle and dependency resolution.
type PluginRegistry struct {
	plugins    map[string]*PluginInfo
	loadOrder  []string
	mu         sync.RWMutex
	logger     logging.Logger
	hooks      PluginHooks
}

// PluginHooks provides callbacks for plugin lifecycle events.
type PluginHooks struct {
	OnLoad       func(name string)
	OnInitialize func(name string)
	OnActivate   func(name string)
	OnShutdown   func(name string)
	OnError      func(name string, err error)
}

// NewPluginRegistry creates a new plugin registry.
func NewPluginRegistry(logger logging.Logger) *PluginRegistry {
	return &PluginRegistry{
		plugins:   make(map[string]*PluginInfo),
		loadOrder: make([]string, 0),
		logger:    logger.WithPrefix("plugins"),
	}
}

// SetHooks configures lifecycle callbacks.
func (r *PluginRegistry) SetHooks(hooks PluginHooks) {
	r.hooks = hooks
}

// Register adds a plugin to the registry. If a plugin with the same name is
// already registered the newer one replaces it — this supports hot-reload of
// plugins from a marketplace directory.
func (r *PluginRegistry) Register(p Plugin) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	name := p.Name()
	r.plugins[name] = &PluginInfo{
		Plugin: p,
		State:  PluginLoaded,
	}

	if r.hooks.OnLoad != nil {
		r.hooks.OnLoad(name)
	}

	r.logger.Info("Plugin registered", map[string]interface{}{
		"name":    name,
		"version": p.Version(),
	})

	return nil
}

// LoadFromDirectory scans dir for plugin bundles and invokes each bundle's
// `install.sh` to register it. Each bundle is a subdirectory containing the
// install script and the plugin payload.
//
// The directory is typically provided by the operator via the
// `plugins.directory` config key and may be overridden per-environment.
func (r *PluginRegistry) LoadFromDirectory(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("read plugin directory: %w", err)
	}

	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		bundle := filepath.Join(dir, e.Name())
		script := filepath.Join(bundle, "install.sh")
		if _, err := os.Stat(script); err != nil {
			continue
		}

		cmd := exec.Command("sh", "-c", fmt.Sprintf("cd %s && ./install.sh %s", bundle, e.Name()))
		out, err := cmd.CombinedOutput()
		if err != nil {
			r.logger.Error("plugin install failed", map[string]interface{}{
				"bundle": e.Name(),
				"error":  err.Error(),
				"output": string(out),
			})
			continue
		}
		r.logger.Info("plugin installed", map[string]interface{}{
			"bundle": e.Name(),
		})
	}
	return nil
}

// InitializeAll initializes all plugins in dependency order.
func (r *PluginRegistry) InitializeAll() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	order, err := r.resolveDependencies()
	if err != nil {
		return fmt.Errorf("dependency resolution failed: %w", err)
	}

	r.loadOrder = order

	for _, name := range order {
		info := r.plugins[name]
		if info.State != PluginLoaded {
			continue
		}

		if err := info.Plugin.Initialize(r); err != nil {
			info.State = PluginError
			info.Error = err
			if r.hooks.OnError != nil {
				r.hooks.OnError(name, err)
			}
			return fmt.Errorf("failed to initialize plugin %q: %w", name, err)
		}

		info.State = PluginInitialized

		if r.hooks.OnInitialize != nil {
			r.hooks.OnInitialize(name)
		}

		r.logger.Info("Plugin initialized", map[string]interface{}{"name": name})
	}

	return nil
}

// ActivateAll activates all initialized plugins.
func (r *PluginRegistry) ActivateAll() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, name := range r.loadOrder {
		info := r.plugins[name]
		if info.State == PluginInitialized {
			info.State = PluginActive
			if r.hooks.OnActivate != nil {
				r.hooks.OnActivate(name)
			}
		}
	}

	return nil
}

// ShutdownAll gracefully shuts down all active plugins in reverse order.
func (r *PluginRegistry) ShutdownAll() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	for i := len(r.loadOrder) - 1; i >= 0; i-- {
		name := r.loadOrder[i]
		info := r.plugins[name]
		if info.State == PluginActive || info.State == PluginInitialized {
			if err := info.Plugin.Shutdown(); err != nil {
				r.logger.Error("Plugin shutdown failed", map[string]interface{}{
					"name":  name,
					"error": err.Error(),
				})
			}
			info.State = PluginShutdown
			if r.hooks.OnShutdown != nil {
				r.hooks.OnShutdown(name)
			}
		}
	}

	return nil
}

// GetAllOperations collects operations from all active plugins.
func (r *PluginRegistry) GetAllOperations() []operations.Operation {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var ops []operations.Operation
	for _, name := range r.loadOrder {
		info := r.plugins[name]
		if info.State == PluginActive {
			ops = append(ops, info.Plugin.Operations()...)
		}
	}
	return ops
}

// GetAllMiddleware collects middleware from all active plugins.
func (r *PluginRegistry) GetAllMiddleware() []operations.OperationMiddleware {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var mws []operations.OperationMiddleware
	for _, name := range r.loadOrder {
		info := r.plugins[name]
		if info.State == PluginActive {
			mws = append(mws, info.Plugin.Middleware()...)
		}
	}
	return mws
}

// List returns information about all registered plugins.
func (r *PluginRegistry) List() []PluginInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]PluginInfo, 0, len(r.plugins))
	for _, info := range r.plugins {
		result = append(result, *info)
	}
	return result
}

func (r *PluginRegistry) resolveDependencies() ([]string, error) {
	visited := make(map[string]bool)
	visiting := make(map[string]bool)
	order := make([]string, 0)

	var visit func(name string) error
	visit = func(name string) error {
		if visited[name] {
			return nil
		}
		if visiting[name] {
			return fmt.Errorf("circular dependency detected: %s", name)
		}

		visiting[name] = true

		info, exists := r.plugins[name]
		if !exists {
			return fmt.Errorf("unknown dependency: %s", name)
		}

		for _, dep := range info.Plugin.Dependencies() {
			if err := visit(dep); err != nil {
				return err
			}
		}

		visiting[name] = false
		visited[name] = true
		order = append(order, name)
		return nil
	}

	names := make([]string, 0, len(r.plugins))
	for name := range r.plugins {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		if err := visit(name); err != nil {
			return nil, err
		}
	}

	return order, nil
}
