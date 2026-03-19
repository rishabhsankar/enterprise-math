package operations

import (
	"context"
	"fmt"
	"math"
	"time"
)

// AddOperation implements enterprise-grade addition.
type AddOperation struct {
	precision        int
	overflowProtect  bool
	auditEnabled     bool
}

// NewAddOperation creates a new addition operation with configuration.
func NewAddOperation(precision int, overflowProtect, audit bool) *AddOperation {
	return &AddOperation{
		precision:       precision,
		overflowProtect: overflowProtect,
		auditEnabled:    audit,
	}
}

func (op *AddOperation) Name() string                  { return "add" }
func (op *AddOperation) Description() string           { return "Enterprise Addition™ - Adds two numbers with full audit trail" }
func (op *AddOperation) Category() OperationCategory   { return CategoryArithmetic }
func (op *AddOperation) Arity() int                    { return 2 }

func (op *AddOperation) Validate(args ...Number) error {
	if len(args) != 2 {
		return fmt.Errorf("addition requires exactly 2 arguments, got %d", len(args))
	}
	if args[0].Unit != args[1].Unit && args[0].Unit != "dimensionless" && args[1].Unit != "dimensionless" {
		return fmt.Errorf("unit mismatch: cannot add %s and %s", args[0].Unit, args[1].Unit)
	}
	return nil
}

func (op *AddOperation) Execute(ctx context.Context, args ...Number) (*OperationResult, error) {
	if err := op.Validate(args...); err != nil {
		return nil, err
	}
	return op.ExecuteBinary(ctx, args[0], args[1])
}

func (op *AddOperation) ExecuteBinary(ctx context.Context, a, b Number) (*OperationResult, error) {
	start := time.Now()

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	result := a.Value + b.Value

	if op.overflowProtect {
		if math.IsInf(result, 0) {
			return nil, fmt.Errorf("overflow detected: %v + %v", a.Value, b.Value)
		}
		if math.IsNaN(result) {
			return nil, fmt.Errorf("NaN detected in addition: %v + %v", a.Value, b.Value)
		}
	}

	unit := a.Unit
	if unit == "dimensionless" {
		unit = b.Unit
	}

	output := NewNumberWithUnit(result, unit)
	duration := time.Since(start)

	opResult := &OperationResult{
		Value:    output,
		Duration: duration,
		Strategy: "direct",
	}

	if op.auditEnabled {
		opResult.AuditTrail = []AuditEntry{{
			Timestamp: start,
			Operation: "add",
			Input:     []Number{a, b},
			Output:    output,
			Duration:  duration,
			Component: "AddOperation",
		}}
	}

	return opResult, nil
}

// SubtractOperation implements enterprise-grade subtraction.
type SubtractOperation struct {
	precision        int
	overflowProtect  bool
	auditEnabled     bool
}

func NewSubtractOperation(precision int, overflowProtect, audit bool) *SubtractOperation {
	return &SubtractOperation{precision: precision, overflowProtect: overflowProtect, auditEnabled: audit}
}

func (op *SubtractOperation) Name() string                  { return "subtract" }
func (op *SubtractOperation) Description() string           { return "Enterprise Subtraction™ - Subtracts with overflow protection" }
func (op *SubtractOperation) Category() OperationCategory   { return CategoryArithmetic }
func (op *SubtractOperation) Arity() int                    { return 2 }

func (op *SubtractOperation) Validate(args ...Number) error {
	if len(args) != 2 {
		return fmt.Errorf("subtraction requires exactly 2 arguments, got %d", len(args))
	}
	return nil
}

func (op *SubtractOperation) Execute(ctx context.Context, args ...Number) (*OperationResult, error) {
	if err := op.Validate(args...); err != nil {
		return nil, err
	}
	return op.ExecuteBinary(ctx, args[0], args[1])
}

func (op *SubtractOperation) ExecuteBinary(ctx context.Context, a, b Number) (*OperationResult, error) {
	start := time.Now()

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	result := a.Value - b.Value

	if op.overflowProtect && (math.IsInf(result, 0) || math.IsNaN(result)) {
		return nil, fmt.Errorf("overflow/NaN in subtraction: %v - %v", a.Value, b.Value)
	}

	output := NewNumberWithUnit(result, a.Unit)
	duration := time.Since(start)

	opResult := &OperationResult{
		Value:    output,
		Duration: duration,
		Strategy: "direct",
	}

	if op.auditEnabled {
		opResult.AuditTrail = []AuditEntry{{
			Timestamp: start, Operation: "subtract",
			Input: []Number{a, b}, Output: output,
			Duration: duration, Component: "SubtractOperation",
		}}
	}

	return opResult, nil
}

// MultiplyOperation implements enterprise-grade multiplication.
type MultiplyOperation struct {
	precision        int
	overflowProtect  bool
	auditEnabled     bool
}

func NewMultiplyOperation(precision int, overflowProtect, audit bool) *MultiplyOperation {
	return &MultiplyOperation{precision: precision, overflowProtect: overflowProtect, auditEnabled: audit}
}

func (op *MultiplyOperation) Name() string                  { return "multiply" }
func (op *MultiplyOperation) Description() string           { return "Enterprise Multiplication™ - Multiplies with dimensional analysis" }
func (op *MultiplyOperation) Category() OperationCategory   { return CategoryArithmetic }
func (op *MultiplyOperation) Arity() int                    { return 2 }

func (op *MultiplyOperation) Validate(args ...Number) error {
	if len(args) != 2 {
		return fmt.Errorf("multiplication requires exactly 2 arguments, got %d", len(args))
	}
	return nil
}

func (op *MultiplyOperation) Execute(ctx context.Context, args ...Number) (*OperationResult, error) {
	if err := op.Validate(args...); err != nil {
		return nil, err
	}
	return op.ExecuteBinary(ctx, args[0], args[1])
}

func (op *MultiplyOperation) ExecuteBinary(ctx context.Context, a, b Number) (*OperationResult, error) {
	start := time.Now()

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	result := a.Value * b.Value

	if op.overflowProtect && (math.IsInf(result, 0) || math.IsNaN(result)) {
		return nil, fmt.Errorf("overflow/NaN in multiplication: %v * %v", a.Value, b.Value)
	}

	unit := combineUnits(a.Unit, b.Unit, "*")
	output := NewNumberWithUnit(result, unit)
	duration := time.Since(start)

	opResult := &OperationResult{
		Value:    output,
		Duration: duration,
		Strategy: "direct",
	}

	if op.auditEnabled {
		opResult.AuditTrail = []AuditEntry{{
			Timestamp: start, Operation: "multiply",
			Input: []Number{a, b}, Output: output,
			Duration: duration, Component: "MultiplyOperation",
		}}
	}

	return opResult, nil
}

// DivideOperation implements enterprise-grade division with zero-division protection.
type DivideOperation struct {
	precision        int
	overflowProtect  bool
	auditEnabled     bool
	zeroDivPolicy    ZeroDivisionPolicy
}

// ZeroDivisionPolicy defines how division by zero is handled.
type ZeroDivisionPolicy int

const (
	ZeroDivError ZeroDivisionPolicy = iota
	ZeroDivInfinity
	ZeroDivNaN
	ZeroDivZero
)

func NewDivideOperation(precision int, overflowProtect, audit bool) *DivideOperation {
	return &DivideOperation{
		precision:       precision,
		overflowProtect: overflowProtect,
		auditEnabled:    audit,
		zeroDivPolicy:   ZeroDivError,
	}
}

func (op *DivideOperation) Name() string                  { return "divide" }
func (op *DivideOperation) Description() string           { return "Enterprise Division™ - Divides with configurable zero-division policy" }
func (op *DivideOperation) Category() OperationCategory   { return CategoryArithmetic }
func (op *DivideOperation) Arity() int                    { return 2 }

func (op *DivideOperation) Validate(args ...Number) error {
	if len(args) != 2 {
		return fmt.Errorf("division requires exactly 2 arguments, got %d", len(args))
	}
	if args[1].Value == 0 && op.zeroDivPolicy == ZeroDivError {
		return fmt.Errorf("division by zero is not permitted under current policy")
	}
	return nil
}

func (op *DivideOperation) Execute(ctx context.Context, args ...Number) (*OperationResult, error) {
	if err := op.Validate(args...); err != nil {
		return nil, err
	}
	return op.ExecuteBinary(ctx, args[0], args[1])
}

func (op *DivideOperation) ExecuteBinary(ctx context.Context, a, b Number) (*OperationResult, error) {
	start := time.Now()

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	if b.Value == 0 {
		switch op.zeroDivPolicy {
		case ZeroDivError:
			return nil, fmt.Errorf("enterprise division by zero: %v / %v", a.Value, b.Value)
		case ZeroDivInfinity:
			sign := 1.0
			if a.Value < 0 {
				sign = -1.0
			}
			return &OperationResult{Value: NewNumber(math.Inf(int(sign)))}, nil
		case ZeroDivNaN:
			return &OperationResult{Value: NewNumber(math.NaN())}, nil
		case ZeroDivZero:
			return &OperationResult{Value: NewNumber(0)}, nil
		}
	}

	result := a.Value / b.Value

	if op.overflowProtect && (math.IsInf(result, 0) || math.IsNaN(result)) {
		return nil, fmt.Errorf("overflow/NaN in division: %v / %v", a.Value, b.Value)
	}

	unit := combineUnits(a.Unit, b.Unit, "/")
	output := NewNumberWithUnit(result, unit)
	duration := time.Since(start)

	opResult := &OperationResult{
		Value:    output,
		Duration: duration,
		Strategy: "direct",
	}

	if op.auditEnabled {
		opResult.AuditTrail = []AuditEntry{{
			Timestamp: start, Operation: "divide",
			Input: []Number{a, b}, Output: output,
			Duration: duration, Component: "DivideOperation",
		}}
	}

	return opResult, nil
}

// ModuloOperation implements enterprise-grade modulo.
type ModuloOperation struct {
	precision        int
	auditEnabled     bool
}

func NewModuloOperation(precision int, audit bool) *ModuloOperation {
	return &ModuloOperation{precision: precision, auditEnabled: audit}
}

func (op *ModuloOperation) Name() string                  { return "modulo" }
func (op *ModuloOperation) Description() string           { return "Enterprise Modulo™ - Remainder with audit compliance" }
func (op *ModuloOperation) Category() OperationCategory   { return CategoryArithmetic }
func (op *ModuloOperation) Arity() int                    { return 2 }

func (op *ModuloOperation) Validate(args ...Number) error {
	if len(args) != 2 {
		return fmt.Errorf("modulo requires exactly 2 arguments, got %d", len(args))
	}
	if args[1].Value == 0 {
		return fmt.Errorf("modulo by zero")
	}
	return nil
}

func (op *ModuloOperation) Execute(ctx context.Context, args ...Number) (*OperationResult, error) {
	if err := op.Validate(args...); err != nil {
		return nil, err
	}

	start := time.Now()
	result := math.Mod(args[0].Value, args[1].Value)
	output := NewNumber(result)
	duration := time.Since(start)

	return &OperationResult{
		Value:    output,
		Duration: duration,
		Strategy: "direct",
	}, nil
}

// PowerOperation implements enterprise exponentiation.
type PowerOperation struct {
	maxExponent     float64
	overflowProtect bool
	auditEnabled    bool
}

func NewPowerOperation(maxExponent float64, overflowProtect, audit bool) *PowerOperation {
	return &PowerOperation{maxExponent: maxExponent, overflowProtect: overflowProtect, auditEnabled: audit}
}

func (op *PowerOperation) Name() string                  { return "power" }
func (op *PowerOperation) Description() string           { return "Enterprise Exponentiation™ - Power with exponent limits" }
func (op *PowerOperation) Category() OperationCategory   { return CategoryArithmetic }
func (op *PowerOperation) Arity() int                    { return 2 }

func (op *PowerOperation) Validate(args ...Number) error {
	if len(args) != 2 {
		return fmt.Errorf("power requires exactly 2 arguments, got %d", len(args))
	}
	if math.Abs(args[1].Value) > op.maxExponent {
		return fmt.Errorf("exponent %v exceeds maximum allowed %v", args[1].Value, op.maxExponent)
	}
	return nil
}

func (op *PowerOperation) Execute(ctx context.Context, args ...Number) (*OperationResult, error) {
	if err := op.Validate(args...); err != nil {
		return nil, err
	}

	start := time.Now()
	result := math.Pow(args[0].Value, args[1].Value)

	if op.overflowProtect && (math.IsInf(result, 0) || math.IsNaN(result)) {
		return nil, fmt.Errorf("overflow in power: %v ^ %v", args[0].Value, args[1].Value)
	}

	output := NewNumber(result)
	duration := time.Since(start)

	return &OperationResult{
		Value:    output,
		Duration: duration,
		Strategy: "direct",
	}, nil
}

func combineUnits(a, b, op string) string {
	if a == "dimensionless" && b == "dimensionless" {
		return "dimensionless"
	}
	if a == "dimensionless" {
		return b
	}
	if b == "dimensionless" {
		return a
	}
	return fmt.Sprintf("(%s%s%s)", a, op, b)
}
