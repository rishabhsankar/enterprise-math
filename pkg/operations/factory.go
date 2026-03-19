package operations

import (
	"fmt"
	"sync"
)

// DefaultOperationFactory manages the registry of available operations.
type DefaultOperationFactory struct {
	constructors map[string]func(map[string]interface{}) Operation
	cache        map[string]Operation
	mu           sync.RWMutex
}

// NewDefaultOperationFactory creates a factory pre-loaded with all built-in operations.
func NewDefaultOperationFactory() *DefaultOperationFactory {
	f := &DefaultOperationFactory{
		constructors: make(map[string]func(map[string]interface{}) Operation),
		cache:        make(map[string]Operation),
	}
	f.registerBuiltins()
	return f
}

func (f *DefaultOperationFactory) registerBuiltins() {
	f.Register("add", func(cfg map[string]interface{}) Operation {
		return NewAddOperation(getIntOrDefault(cfg, "precision", 64), getBoolOrDefault(cfg, "overflow_protect", true), getBoolOrDefault(cfg, "audit", true))
	})
	f.Register("subtract", func(cfg map[string]interface{}) Operation {
		return NewSubtractOperation(getIntOrDefault(cfg, "precision", 64), getBoolOrDefault(cfg, "overflow_protect", true), getBoolOrDefault(cfg, "audit", true))
	})
	f.Register("multiply", func(cfg map[string]interface{}) Operation {
		return NewMultiplyOperation(getIntOrDefault(cfg, "precision", 64), getBoolOrDefault(cfg, "overflow_protect", true), getBoolOrDefault(cfg, "audit", true))
	})
	f.Register("divide", func(cfg map[string]interface{}) Operation {
		return NewDivideOperation(getIntOrDefault(cfg, "precision", 64), getBoolOrDefault(cfg, "overflow_protect", true), getBoolOrDefault(cfg, "audit", true))
	})
	f.Register("modulo", func(cfg map[string]interface{}) Operation {
		return NewModuloOperation(getIntOrDefault(cfg, "precision", 64), getBoolOrDefault(cfg, "audit", true))
	})
	f.Register("power", func(cfg map[string]interface{}) Operation {
		return NewPowerOperation(getFloatOrDefault(cfg, "max_exponent", 1000), getBoolOrDefault(cfg, "overflow_protect", true), getBoolOrDefault(cfg, "audit", true))
	})

	// Trig
	f.Register("sin", func(cfg map[string]interface{}) Operation {
		return NewSineOperation(Radians, getBoolOrDefault(cfg, "audit", true))
	})
	f.Register("cos", func(cfg map[string]interface{}) Operation {
		return NewCosineOperation(Radians, getBoolOrDefault(cfg, "audit", true))
	})
	f.Register("tan", func(cfg map[string]interface{}) Operation {
		return NewTangentOperation(Radians, true, getBoolOrDefault(cfg, "audit", true))
	})
	f.Register("asin", func(cfg map[string]interface{}) Operation {
		return NewArcSineOperation(Radians, getBoolOrDefault(cfg, "audit", true))
	})
	f.Register("atan2", func(cfg map[string]interface{}) Operation {
		return NewAtan2Operation(Radians, getBoolOrDefault(cfg, "audit", true))
	})
	f.Register("sinh", func(cfg map[string]interface{}) Operation {
		return NewHyperbolicSineOperation(getBoolOrDefault(cfg, "audit", true))
	})

	// Statistical
	f.Register("mean", func(cfg map[string]interface{}) Operation {
		return NewMeanOperation(getBoolOrDefault(cfg, "audit", true))
	})
	f.Register("median", func(cfg map[string]interface{}) Operation {
		return NewMedianOperation(getBoolOrDefault(cfg, "audit", true))
	})
	f.Register("variance", func(cfg map[string]interface{}) Operation {
		return NewVarianceOperation(getBoolOrDefault(cfg, "population", false), getBoolOrDefault(cfg, "audit", true))
	})
	f.Register("stddev", func(cfg map[string]interface{}) Operation {
		return NewStandardDeviationOperation(getBoolOrDefault(cfg, "population", false), getBoolOrDefault(cfg, "audit", true))
	})
	f.Register("correlation", func(cfg map[string]interface{}) Operation {
		return NewCorrelationOperation(getBoolOrDefault(cfg, "audit", true))
	})

	// Bitwise
	f.Register("bitwise_and", func(cfg map[string]interface{}) Operation {
		return NewBitwiseAndOperation(getIntOrDefault(cfg, "bit_width", 64), getBoolOrDefault(cfg, "audit", true))
	})
	f.Register("bitwise_or", func(cfg map[string]interface{}) Operation {
		return NewBitwiseOrOperation(getIntOrDefault(cfg, "bit_width", 64), getBoolOrDefault(cfg, "audit", true))
	})
	f.Register("bitwise_xor", func(cfg map[string]interface{}) Operation {
		return NewBitwiseXorOperation(getIntOrDefault(cfg, "bit_width", 64), getBoolOrDefault(cfg, "audit", true))
	})
	f.Register("shift_left", func(cfg map[string]interface{}) Operation {
		return NewBitShiftLeftOperation(getIntOrDefault(cfg, "max_shift", 63), getBoolOrDefault(cfg, "audit", true))
	})
	f.Register("shift_right", func(cfg map[string]interface{}) Operation {
		return NewBitShiftRightOperation(getIntOrDefault(cfg, "max_shift", 63), true, getBoolOrDefault(cfg, "audit", true))
	})
	f.Register("popcount", func(cfg map[string]interface{}) Operation {
		return NewPopCountOperation(getBoolOrDefault(cfg, "audit", true))
	})
}

// Create instantiates an operation by name with the given configuration.
func (f *DefaultOperationFactory) Create(name string, config map[string]interface{}) (Operation, error) {
	f.mu.RLock()
	constructor, exists := f.constructors[name]
	f.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("unknown operation: %s", name)
	}

	return constructor(config), nil
}

// Register adds a new operation constructor to the factory.
func (f *DefaultOperationFactory) Register(name string, constructor func(map[string]interface{}) Operation) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.constructors[name] = constructor
}

// List returns the names of all registered operations.
func (f *DefaultOperationFactory) List() []string {
	f.mu.RLock()
	defer f.mu.RUnlock()
	names := make([]string, 0, len(f.constructors))
	for name := range f.constructors {
		names = append(names, name)
	}
	return names
}

// GetOrCreate retrieves a cached operation or creates a new one.
func (f *DefaultOperationFactory) GetOrCreate(name string, config map[string]interface{}) (Operation, error) {
	f.mu.RLock()
	if cached, exists := f.cache[name]; exists {
		f.mu.RUnlock()
		return cached, nil
	}
	f.mu.RUnlock()

	op, err := f.Create(name, config)
	if err != nil {
		return nil, err
	}

	f.mu.Lock()
	f.cache[name] = op
	f.mu.Unlock()

	return op, nil
}

func getIntOrDefault(cfg map[string]interface{}, key string, def int) int {
	if cfg == nil {
		return def
	}
	if v, ok := cfg[key]; ok {
		switch val := v.(type) {
		case int:
			return val
		case float64:
			return int(val)
		}
	}
	return def
}

func getFloatOrDefault(cfg map[string]interface{}, key string, def float64) float64 {
	if cfg == nil {
		return def
	}
	if v, ok := cfg[key]; ok {
		switch val := v.(type) {
		case float64:
			return val
		case int:
			return float64(val)
		}
	}
	return def
}

func getBoolOrDefault(cfg map[string]interface{}, key string, def bool) bool {
	if cfg == nil {
		return def
	}
	if v, ok := cfg[key]; ok {
		if val, ok := v.(bool); ok {
			return val
		}
	}
	return def
}
