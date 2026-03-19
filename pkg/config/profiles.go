package config

import (
	"fmt"
	"sync"
)

// Profile represents a named set of configuration overrides.
type Profile struct {
	Name        string
	Description string
	Overrides   map[string]interface{}
	Priority    int
	Active      bool
}

// ProfileManager manages configuration profiles and their activation.
type ProfileManager struct {
	profiles map[string]*Profile
	active   []string
	config   *ConfigManager
	mu       sync.RWMutex
}

// NewProfileManager creates a new profile manager backed by the given config.
func NewProfileManager(config *ConfigManager) *ProfileManager {
	pm := &ProfileManager{
		profiles: make(map[string]*Profile),
		active:   make([]string, 0),
		config:   config,
	}
	pm.registerBuiltinProfiles()
	return pm
}

func (pm *ProfileManager) registerBuiltinProfiles() {
	pm.Register(&Profile{
		Name:        "high-precision",
		Description: "Maximum precision for financial calculations",
		Priority:    10,
		Overrides: map[string]interface{}{
			"math.precision":            128,
			"math.rounding_mode":        "half_even",
			"math.overflow_protection":  true,
			"math.underflow_protection": true,
			"validation.strict_mode":    true,
		},
	})

	pm.Register(&Profile{
		Name:        "performance",
		Description: "Optimized for throughput over precision",
		Priority:    10,
		Overrides: map[string]interface{}{
			"math.precision":         32,
			"computation.parallel":   true,
			"computation.worker_count": 8,
			"cache.enabled":          true,
			"cache.max_size":         100000,
			"audit.enabled":          false,
			"metrics.enabled":        false,
		},
	})

	pm.Register(&Profile{
		Name:        "debug",
		Description: "Verbose logging and validation for development",
		Priority:    20,
		Overrides: map[string]interface{}{
			"audit.enabled":          true,
			"audit.log_inputs":       true,
			"audit.log_outputs":      true,
			"metrics.enabled":        true,
			"validation.strict_mode": true,
		},
	})

	pm.Register(&Profile{
		Name:        "minimal",
		Description: "Minimal overhead for embedded usage",
		Priority:    5,
		Overrides: map[string]interface{}{
			"computation.parallel":  false,
			"cache.enabled":         false,
			"metrics.enabled":       false,
			"audit.enabled":         false,
			"plugins.auto_discover": false,
		},
	})
}

// Register adds a new profile to the manager.
func (pm *ProfileManager) Register(profile *Profile) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	pm.profiles[profile.Name] = profile
}

// Activate enables a profile and applies its overrides.
func (pm *ProfileManager) Activate(name string) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	profile, exists := pm.profiles[name]
	if !exists {
		return fmt.Errorf("profile %q not found", name)
	}

	profile.Active = true
	pm.active = append(pm.active, name)

	for key, value := range profile.Overrides {
		if err := pm.config.Set(key, value); err != nil {
			return fmt.Errorf("failed to apply profile %q override for %s: %w", name, key, err)
		}
	}

	return nil
}

// Deactivate disables a profile and restores defaults.
func (pm *ProfileManager) Deactivate(name string) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	profile, exists := pm.profiles[name]
	if !exists {
		return fmt.Errorf("profile %q not found", name)
	}

	profile.Active = false

	newActive := make([]string, 0)
	for _, n := range pm.active {
		if n != name {
			newActive = append(newActive, n)
		}
	}
	pm.active = newActive

	return nil
}

// ListProfiles returns all registered profiles.
func (pm *ProfileManager) ListProfiles() []*Profile {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	result := make([]*Profile, 0, len(pm.profiles))
	for _, p := range pm.profiles {
		result = append(result, p)
	}
	return result
}

// ActiveProfiles returns the names of currently active profiles.
func (pm *ProfileManager) ActiveProfiles() []string {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	result := make([]string, len(pm.active))
	copy(result, pm.active)
	return result
}
