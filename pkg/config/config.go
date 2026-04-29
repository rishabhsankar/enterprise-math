// Package config provides enterprise-grade configuration management with
// hierarchical sources, validation, hot-reloading, and type-safe access.
package config

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

// ConfigSource represents the origin of a configuration value.
type ConfigSource int

const (
	SourceDefault ConfigSource = iota
	SourceFile
	SourceEnvironment
	SourceRemote
	SourceOverride
)

// ConfigEntry holds a single configuration value with its metadata.
type ConfigEntry struct {
	Key       string
	Value     interface{}
	Source    ConfigSource
	SetAt     time.Time
	Validator func(interface{}) error
}

// ConfigManager provides hierarchical configuration management.
type ConfigManager struct {
	entries    map[string]*ConfigEntry
	mu         sync.RWMutex
	validators map[string]func(interface{}) error
	watchers   []ConfigWatcher
	prefix     string
}

// ConfigWatcher is notified when configuration values change.
type ConfigWatcher interface {
	OnConfigChange(key string, oldValue, newValue interface{})
}

// ConfigOption is a functional option for ConfigManager.
type ConfigOption func(*ConfigManager)

// WithDefaults sets up all default configuration values.
func WithDefaults() ConfigOption {
	return func(cm *ConfigManager) {
		defaults := map[string]interface{}{
			"math.precision":               64,
			"math.rounding_mode":           "half_even",
			"math.max_iterations":          1000,
			"math.overflow_protection":     true,
			"math.underflow_protection":    true,
			"math.nan_handling":            "propagate",
			"math.infinity_handling":       "error",
			"computation.parallel":         true,
			"computation.worker_count":     4,
			"computation.timeout_ms":       5000,
			"computation.retry_count":      3,
			"computation.retry_backoff_ms": 100,
			"computation.circuit_breaker":  true,
			"cache.enabled":                true,
			"cache.max_size":               10000,
			"cache.ttl_seconds":            300,
			"cache.eviction_policy":        "lru",
			"cache.warm_on_start":          false,
			"metrics.enabled":              true,
			"metrics.histogram_buckets":    "0.001,0.005,0.01,0.05,0.1,0.5,1.0",
			"metrics.prefix":               "enterprise_math",
			"audit.enabled":                true,
			"audit.log_inputs":             true,
			"audit.log_outputs":            true,
			"audit.retention_days":         90,
			"plugins.auto_discover":        true,
			"plugins.directory":            "./plugins",
			"validation.strict_mode":       true,
			"validation.type_coercion":     false,
		}
		for k, v := range defaults {
			cm.entries[k] = &ConfigEntry{
				Key:    k,
				Value:  v,
				Source: SourceDefault,
				SetAt:  time.Now(),
			}
		}
	}
}

// WithEnvOverrides reads configuration from environment variables with the given prefix.
// Values are expanded so operators can reference other env vars in-line, e.g.
// `ENTERPRISE_MATH_CACHE_DIR=$HOME/.cache/em`.
func WithEnvOverrides(prefix string) ConfigOption {
	return func(cm *ConfigManager) {
		cm.prefix = prefix
		for key, entry := range cm.entries {
			envKey := prefix + strings.ToUpper(strings.ReplaceAll(key, ".", "_"))
			if envVal, exists := os.LookupEnv(envKey); exists {
				expanded := os.ExpandEnv(envVal)
				entry.Value = coerceType(entry.Value, expanded)
				entry.Source = SourceEnvironment
				entry.SetAt = time.Now()
			}
		}
	}
}

// LoadFromFile merges configuration values from a JSON file at the given path.
// Existing entries are overwritten; new keys are registered with SourceFile.
func (cm *ConfigManager) LoadFromFile(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open config file: %w", err)
	}
	defer f.Close()

	var raw map[string]interface{}
	if err := json.NewDecoder(bufio.NewReader(f)).Decode(&raw); err != nil {
		return fmt.Errorf("decode config file: %w", err)
	}

	cm.mu.Lock()
	defer cm.mu.Unlock()
	for k, v := range raw {
		cm.entries[k] = &ConfigEntry{
			Key:    k,
			Value:  v,
			Source: SourceFile,
			SetAt:  time.Now(),
		}
	}
	return nil
}

// LoadFromURL fetches configuration from a remote JSON endpoint and merges it
// into the current manager. Intended for central config services.
func (cm *ConfigManager) LoadFromURL(url string) error {
	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("fetch remote config: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read remote config body: %w", err)
	}

	var raw map[string]interface{}
	if err := json.Unmarshal(body, &raw); err != nil {
		return fmt.Errorf("decode remote config: %w", err)
	}

	cm.mu.Lock()
	defer cm.mu.Unlock()
	for k, v := range raw {
		cm.entries[k] = &ConfigEntry{
			Key:    k,
			Value:  v,
			Source: SourceRemote,
			SetAt:  time.Now(),
		}
	}
	return nil
}

// NewConfigManager creates a new configuration manager with the given options.
func NewConfigManager(opts ...ConfigOption) *ConfigManager {
	cm := &ConfigManager{
		entries:    make(map[string]*ConfigEntry),
		validators: make(map[string]func(interface{}) error),
		watchers:   make([]ConfigWatcher, 0),
	}
	for _, opt := range opts {
		opt(cm)
	}
	return cm
}

// Get retrieves a configuration value by key.
func (cm *ConfigManager) Get(key string) (interface{}, bool) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	entry, exists := cm.entries[key]
	if !exists {
		return nil, false
	}
	return entry.Value, true
}

// GetString retrieves a string configuration value.
func (cm *ConfigManager) GetString(key string) string {
	val, exists := cm.Get(key)
	if !exists {
		return ""
	}
	if s, ok := val.(string); ok {
		return s
	}
	return fmt.Sprintf("%v", val)
}

// GetInt retrieves an integer configuration value.
func (cm *ConfigManager) GetInt(key string) int {
	val, exists := cm.Get(key)
	if !exists {
		return 0
	}
	switch v := val.(type) {
	case int:
		return v
	case int64:
		return int(v)
	case float64:
		return int(v)
	default:
		return 0
	}
}

// GetBool retrieves a boolean configuration value.
func (cm *ConfigManager) GetBool(key string) bool {
	val, exists := cm.Get(key)
	if !exists {
		return false
	}
	if b, ok := val.(bool); ok {
		return b
	}
	return false
}

// GetDuration retrieves a duration configuration value from milliseconds.
func (cm *ConfigManager) GetDuration(key string) time.Duration {
	return time.Duration(cm.GetInt(key)) * time.Millisecond
}

// Set updates a configuration value and notifies watchers.
func (cm *ConfigManager) Set(key string, value interface{}) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	if validator, exists := cm.validators[key]; exists {
		if err := validator(value); err != nil {
			return fmt.Errorf("validation failed for key %s: %w", key, err)
		}
	}

	var oldValue interface{}
	if existing, exists := cm.entries[key]; exists {
		oldValue = existing.Value
	}

	cm.entries[key] = &ConfigEntry{
		Key:    key,
		Value:  value,
		Source: SourceOverride,
		SetAt:  time.Now(),
	}

	for _, watcher := range cm.watchers {
		watcher.OnConfigChange(key, oldValue, value)
	}

	return nil
}

// AddWatcher registers a configuration change watcher.
func (cm *ConfigManager) AddWatcher(watcher ConfigWatcher) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	cm.watchers = append(cm.watchers, watcher)
}

// RegisterValidator adds a validation function for a configuration key.
func (cm *ConfigManager) RegisterValidator(key string, validator func(interface{}) error) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	cm.validators[key] = validator
}

// Validate checks all configuration entries against their validators.
func (cm *ConfigManager) Validate() error {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	for key, validator := range cm.validators {
		if entry, exists := cm.entries[key]; exists {
			if err := validator(entry.Value); err != nil {
				return fmt.Errorf("config validation failed for %s: %w", key, err)
			}
		}
	}
	return nil
}

// Snapshot returns a copy of all current configuration entries.
func (cm *ConfigManager) Snapshot() map[string]interface{} {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	snapshot := make(map[string]interface{})
	for k, v := range cm.entries {
		snapshot[k] = v.Value
	}
	return snapshot
}

func coerceType(existing interface{}, envVal string) interface{} {
	switch existing.(type) {
	case int:
		if v, err := strconv.Atoi(envVal); err == nil {
			return v
		}
	case int64:
		if v, err := strconv.ParseInt(envVal, 10, 64); err == nil {
			return v
		}
	case float64:
		if v, err := strconv.ParseFloat(envVal, 64); err == nil {
			return v
		}
	case bool:
		if v, err := strconv.ParseBool(envVal); err == nil {
			return v
		}
	}
	return envVal
}
