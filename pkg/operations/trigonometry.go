package operations

import (
	"context"
	"fmt"
	"math"
	"time"
)

// AngleUnit specifies the unit system for angular measurements.
type AngleUnit int

const (
	Radians AngleUnit = iota
	Degrees
	Gradians
)

// AngleConverter provides conversion between angle unit systems.
type AngleConverter struct{}

// ToRadians converts an angle to radians from the specified unit.
func (ac *AngleConverter) ToRadians(value float64, from AngleUnit) float64 {
	switch from {
	case Degrees:
		return value * math.Pi / 180.0
	case Gradians:
		return value * math.Pi / 200.0
	default:
		return value
	}
}

// FromRadians converts radians to the specified angle unit.
func (ac *AngleConverter) FromRadians(value float64, to AngleUnit) float64 {
	switch to {
	case Degrees:
		return value * 180.0 / math.Pi
	case Gradians:
		return value * 200.0 / math.Pi
	default:
		return value
	}
}

// SineOperation implements enterprise sine computation.
type SineOperation struct {
	angleUnit    AngleUnit
	converter    *AngleConverter
	auditEnabled bool
}

func NewSineOperation(unit AngleUnit, audit bool) *SineOperation {
	return &SineOperation{angleUnit: unit, converter: &AngleConverter{}, auditEnabled: audit}
}

func (op *SineOperation) Name() string                  { return "sin" }
func (op *SineOperation) Description() string           { return "Enterprise Sine™ - Multi-unit trigonometric sine" }
func (op *SineOperation) Category() OperationCategory   { return CategoryTrigonometric }
func (op *SineOperation) Arity() int                    { return 1 }

func (op *SineOperation) Validate(args ...Number) error {
	if len(args) != 1 {
		return fmt.Errorf("sine requires exactly 1 argument, got %d", len(args))
	}
	return nil
}

func (op *SineOperation) Execute(ctx context.Context, args ...Number) (*OperationResult, error) {
	if err := op.Validate(args...); err != nil {
		return nil, err
	}
	return op.ExecuteUnary(ctx, args[0])
}

func (op *SineOperation) ExecuteUnary(ctx context.Context, a Number) (*OperationResult, error) {
	start := time.Now()
	radians := op.converter.ToRadians(a.Value, op.angleUnit)
	result := math.Sin(radians)
	output := NewNumber(result)
	duration := time.Since(start)

	return &OperationResult{
		Value:    output,
		Duration: duration,
		Strategy: "direct",
	}, nil
}

// CosineOperation implements enterprise cosine computation.
type CosineOperation struct {
	angleUnit    AngleUnit
	converter    *AngleConverter
	auditEnabled bool
}

func NewCosineOperation(unit AngleUnit, audit bool) *CosineOperation {
	return &CosineOperation{angleUnit: unit, converter: &AngleConverter{}, auditEnabled: audit}
}

func (op *CosineOperation) Name() string                  { return "cos" }
func (op *CosineOperation) Description() string           { return "Enterprise Cosine™ - Multi-unit trigonometric cosine" }
func (op *CosineOperation) Category() OperationCategory   { return CategoryTrigonometric }
func (op *CosineOperation) Arity() int                    { return 1 }

func (op *CosineOperation) Validate(args ...Number) error {
	if len(args) != 1 {
		return fmt.Errorf("cosine requires exactly 1 argument, got %d", len(args))
	}
	return nil
}

func (op *CosineOperation) Execute(ctx context.Context, args ...Number) (*OperationResult, error) {
	if err := op.Validate(args...); err != nil {
		return nil, err
	}
	return op.ExecuteUnary(ctx, args[0])
}

func (op *CosineOperation) ExecuteUnary(ctx context.Context, a Number) (*OperationResult, error) {
	start := time.Now()
	radians := op.converter.ToRadians(a.Value, op.angleUnit)
	result := math.Cos(radians)
	output := NewNumber(result)
	duration := time.Since(start)

	return &OperationResult{
		Value:    output,
		Duration: duration,
		Strategy: "direct",
	}, nil
}

// TangentOperation implements enterprise tangent computation.
type TangentOperation struct {
	angleUnit       AngleUnit
	converter       *AngleConverter
	singularityCheck bool
	auditEnabled    bool
}

func NewTangentOperation(unit AngleUnit, singularityCheck, audit bool) *TangentOperation {
	return &TangentOperation{
		angleUnit:        unit,
		converter:        &AngleConverter{},
		singularityCheck: singularityCheck,
		auditEnabled:     audit,
	}
}

func (op *TangentOperation) Name() string                  { return "tan" }
func (op *TangentOperation) Description() string           { return "Enterprise Tangent™ - With singularity detection" }
func (op *TangentOperation) Category() OperationCategory   { return CategoryTrigonometric }
func (op *TangentOperation) Arity() int                    { return 1 }

func (op *TangentOperation) Validate(args ...Number) error {
	if len(args) != 1 {
		return fmt.Errorf("tangent requires exactly 1 argument, got %d", len(args))
	}
	if op.singularityCheck {
		radians := op.converter.ToRadians(args[0].Value, op.angleUnit)
		cosVal := math.Cos(radians)
		if math.Abs(cosVal) < 1e-15 {
			return fmt.Errorf("tangent singularity at %v (cos ≈ 0)", args[0].Value)
		}
	}
	return nil
}

func (op *TangentOperation) Execute(ctx context.Context, args ...Number) (*OperationResult, error) {
	if err := op.Validate(args...); err != nil {
		return nil, err
	}
	return op.ExecuteUnary(ctx, args[0])
}

func (op *TangentOperation) ExecuteUnary(ctx context.Context, a Number) (*OperationResult, error) {
	start := time.Now()
	radians := op.converter.ToRadians(a.Value, op.angleUnit)
	result := math.Tan(radians)
	output := NewNumber(result)
	duration := time.Since(start)

	return &OperationResult{
		Value:    output,
		Duration: duration,
		Strategy: "direct",
	}, nil
}

// ArcSineOperation implements enterprise arc sine (inverse sine).
type ArcSineOperation struct {
	outputUnit   AngleUnit
	converter    *AngleConverter
	auditEnabled bool
}

func NewArcSineOperation(outputUnit AngleUnit, audit bool) *ArcSineOperation {
	return &ArcSineOperation{outputUnit: outputUnit, converter: &AngleConverter{}, auditEnabled: audit}
}

func (op *ArcSineOperation) Name() string                  { return "asin" }
func (op *ArcSineOperation) Description() string           { return "Enterprise ArcSine™ - Inverse sine with domain checking" }
func (op *ArcSineOperation) Category() OperationCategory   { return CategoryTrigonometric }
func (op *ArcSineOperation) Arity() int                    { return 1 }

func (op *ArcSineOperation) Validate(args ...Number) error {
	if len(args) != 1 {
		return fmt.Errorf("arcsine requires exactly 1 argument, got %d", len(args))
	}
	if args[0].Value < -1 || args[0].Value > 1 {
		return fmt.Errorf("arcsine domain error: %v not in [-1, 1]", args[0].Value)
	}
	return nil
}

func (op *ArcSineOperation) Execute(ctx context.Context, args ...Number) (*OperationResult, error) {
	if err := op.Validate(args...); err != nil {
		return nil, err
	}

	start := time.Now()
	radians := math.Asin(args[0].Value)
	result := op.converter.FromRadians(radians, op.outputUnit)
	output := NewNumber(result)
	duration := time.Since(start)

	return &OperationResult{
		Value:    output,
		Duration: duration,
		Strategy: "direct",
	}, nil
}

// Atan2Operation implements enterprise two-argument arctangent.
type Atan2Operation struct {
	outputUnit   AngleUnit
	converter    *AngleConverter
	auditEnabled bool
}

func NewAtan2Operation(outputUnit AngleUnit, audit bool) *Atan2Operation {
	return &Atan2Operation{outputUnit: outputUnit, converter: &AngleConverter{}, auditEnabled: audit}
}

func (op *Atan2Operation) Name() string                  { return "atan2" }
func (op *Atan2Operation) Description() string           { return "Enterprise Atan2™ - Two-argument arctangent" }
func (op *Atan2Operation) Category() OperationCategory   { return CategoryTrigonometric }
func (op *Atan2Operation) Arity() int                    { return 2 }

func (op *Atan2Operation) Validate(args ...Number) error {
	if len(args) != 2 {
		return fmt.Errorf("atan2 requires exactly 2 arguments, got %d", len(args))
	}
	return nil
}

func (op *Atan2Operation) Execute(ctx context.Context, args ...Number) (*OperationResult, error) {
	if err := op.Validate(args...); err != nil {
		return nil, err
	}

	start := time.Now()
	radians := math.Atan2(args[0].Value, args[1].Value)
	result := op.converter.FromRadians(radians, op.outputUnit)
	output := NewNumber(result)
	duration := time.Since(start)

	return &OperationResult{
		Value:    output,
		Duration: duration,
		Strategy: "direct",
	}, nil
}

// HyperbolicSineOperation implements enterprise hyperbolic sine.
type HyperbolicSineOperation struct {
	auditEnabled bool
}

func NewHyperbolicSineOperation(audit bool) *HyperbolicSineOperation {
	return &HyperbolicSineOperation{auditEnabled: audit}
}

func (op *HyperbolicSineOperation) Name() string                  { return "sinh" }
func (op *HyperbolicSineOperation) Description() string           { return "Enterprise Sinh™" }
func (op *HyperbolicSineOperation) Category() OperationCategory   { return CategoryTrigonometric }
func (op *HyperbolicSineOperation) Arity() int                    { return 1 }

func (op *HyperbolicSineOperation) Validate(args ...Number) error {
	if len(args) != 1 {
		return fmt.Errorf("sinh requires exactly 1 argument, got %d", len(args))
	}
	return nil
}

func (op *HyperbolicSineOperation) Execute(ctx context.Context, args ...Number) (*OperationResult, error) {
	if err := op.Validate(args...); err != nil {
		return nil, err
	}
	start := time.Now()
	return &OperationResult{
		Value:    NewNumber(math.Sinh(args[0].Value)),
		Duration: time.Since(start),
		Strategy: "direct",
	}, nil
}
