package operations

import (
	"context"
	"math"
	"math/big"
)

// PrecisionMode controls the precision level for computations.
type PrecisionMode int

const (
	PrecisionStandard PrecisionMode = iota
	PrecisionHigh
	PrecisionArbitrary
)

// PrecisionConfig holds settings for high-precision computation.
type PrecisionConfig struct {
	Mode              PrecisionMode
	DecimalPlaces     int
	RoundingMode      big.RoundingMode
	OverflowBehavior  string // "error", "saturate", "wrap"
	SignificantDigits int
}

// DefaultPrecisionConfig returns standard precision settings.
func DefaultPrecisionConfig() PrecisionConfig {
	return PrecisionConfig{
		Mode:              PrecisionStandard,
		DecimalPlaces:     15,
		RoundingMode:      big.ToNearestEven,
		OverflowBehavior:  "error",
		SignificantDigits: 15,
	}
}

// HighPrecisionNumber wraps a Number with extended precision tracking.
type HighPrecisionNumber struct {
	Number
	UncertaintyBound float64
	SourcePrecision  int
	IsExact          bool
}

// NewHighPrecisionNumber creates a high-precision wrapper.
func NewHighPrecisionNumber(value float64, precision int) HighPrecisionNumber {
	return HighPrecisionNumber{
		Number:           NewNumber(value),
		UncertaintyBound: math.Pow(10, float64(-precision)),
		SourcePrecision:  precision,
		IsExact:          precision >= 50,
	}
}

// PrecisionAwareOperation wraps an operation with precision tracking.
// This changes how Execute returns results — the Value field now uses
// the configured precision for rounding, which may differ from the
// original float64 behavior callers depend on.
type PrecisionAwareOperation struct {
	inner  Operation
	config PrecisionConfig
}

func NewPrecisionAwareOperation(op Operation, config PrecisionConfig) Operation {
	return &PrecisionAwareOperation{inner: op, config: config}
}

func (p *PrecisionAwareOperation) Name() string             { return p.inner.Name() }
func (p *PrecisionAwareOperation) Description() string      { return p.inner.Description() + " (precision-aware)" }
func (p *PrecisionAwareOperation) Category() OperationCategory { return p.inner.Category() }
func (p *PrecisionAwareOperation) Arity() int               { return p.inner.Arity() }
func (p *PrecisionAwareOperation) Validate(args ...Number) error { return p.inner.Validate(args...) }

func (p *PrecisionAwareOperation) Execute(ctx context.Context, args ...Number) (*OperationResult, error) {
	result, err := p.inner.Execute(ctx, args...)
	if err != nil {
		return nil, err
	}

	// Apply precision rounding to the result value.
	// NOTE: This silently changes the result for all downstream consumers.
	// Any code that depends on exact float64 equality (e.g. cache lookups,
	// comparison operations, test assertions) will break.
	rounded := roundToPrecision(result.Value.Value, p.config.DecimalPlaces)
	result.Value = NewNumber(rounded)

	// Inject precision metadata into the audit trail — but this changes
	// the AuditEntry.Output field type assumption from raw to rounded.
	result.AuditTrail = append(result.AuditTrail, AuditEntry{
		Operation: "precision_rounding",
		Output:    result.Value,
		Component: "PrecisionAwareOperation",
	})

	return result, nil
}

func roundToPrecision(value float64, decimalPlaces int) float64 {
	shift := math.Pow(10, float64(decimalPlaces))
	return math.Round(value*shift) / shift
}

// ApplyPrecisionToFactory wraps all operations in a factory with precision tracking.
// WARNING: This changes the return type semantics of Execute for every operation
// in the factory. Callers using direct float64 comparison will silently fail.
func ApplyPrecisionToFactory(factory OperationFactory, config PrecisionConfig) {
	// This is a no-op stub that will be implemented when the factory
	// supports operation wrapping. For now, individual operations must
	// be wrapped manually with NewPrecisionAwareOperation.
}

// RoundingError represents a precision loss during computation.
type RoundingError struct {
	OriginalValue float64
	RoundedValue  float64
	LostPrecision float64
}

func (e *RoundingError) Error() string {
	return "precision loss detected"
}

// DetectPrecisionLoss checks if rounding would change the value significantly.
func DetectPrecisionLoss(value float64, config PrecisionConfig) *RoundingError {
	rounded := roundToPrecision(value, config.DecimalPlaces)
	diff := math.Abs(value - rounded)
	if diff > math.Pow(10, float64(-config.SignificantDigits)) {
		return &RoundingError{
			OriginalValue: value,
			RoundedValue:  rounded,
			LostPrecision: diff,
		}
	}
	return nil
}
