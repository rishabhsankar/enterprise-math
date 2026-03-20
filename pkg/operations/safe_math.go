package operations

import (
	"context"
	"fmt"
	"math"
	"time"
)

// SafeMathError wraps a math error with context about the operation that failed.
type SafeMathError struct {
	Operation string
	Input     []Number
	Cause     error
	Timestamp time.Time
}

func (e *SafeMathError) Error() string {
	return fmt.Sprintf("safe_math[%s]: %v (inputs: %v)", e.Operation, e.Cause, e.Input)
}

func (e *SafeMathError) Unwrap() error { return e.Cause }

// SafeAddOperation wraps AddOperation with NaN/Inf input rejection.
type SafeAddOperation struct {
	inner *AddOperation
}

func NewSafeAddOperation(precision int, audit bool) *SafeAddOperation {
	return &SafeAddOperation{
		inner: NewAddOperation(precision, true, audit),
	}
}

func (op *SafeAddOperation) Name() string                { return "safe_add" }
func (op *SafeAddOperation) Description() string         { return "Safe Addition - rejects NaN/Inf inputs before delegating to AddOperation" }
func (op *SafeAddOperation) Category() OperationCategory { return CategoryArithmetic }
func (op *SafeAddOperation) Arity() int                  { return 2 }

func (op *SafeAddOperation) Validate(args ...Number) error {
	for i, arg := range args {
		if math.IsNaN(arg.Value) {
			return &SafeMathError{Operation: "safe_add", Input: args, Cause: fmt.Errorf("argument %d is NaN", i)}
		}
		if math.IsInf(arg.Value, 0) {
			return &SafeMathError{Operation: "safe_add", Input: args, Cause: fmt.Errorf("argument %d is Inf", i)}
		}
	}
	return op.inner.Validate(args...)
}

func (op *SafeAddOperation) Execute(ctx context.Context, args ...Number) (*OperationResult, error) {
	if err := op.Validate(args...); err != nil {
		return nil, err
	}
	result, err := op.inner.Execute(ctx, args...)
	if err != nil {
		return nil, &SafeMathError{Operation: "safe_add", Input: args, Cause: err}
	}
	return result, nil
}

func (op *SafeAddOperation) ExecuteBinary(ctx context.Context, a, b Number) (*OperationResult, error) {
	return op.Execute(ctx, a, b)
}

// SafeSubtractOperation wraps SubtractOperation with input validation.
type SafeSubtractOperation struct {
	inner *SubtractOperation
}

func NewSafeSubtractOperation(precision int, audit bool) *SafeSubtractOperation {
	return &SafeSubtractOperation{
		inner: NewSubtractOperation(precision, true, audit),
	}
}

func (op *SafeSubtractOperation) Name() string                { return "safe_subtract" }
func (op *SafeSubtractOperation) Description() string         { return "Safe Subtraction - rejects NaN/Inf inputs" }
func (op *SafeSubtractOperation) Category() OperationCategory { return CategoryArithmetic }
func (op *SafeSubtractOperation) Arity() int                  { return 2 }

func (op *SafeSubtractOperation) Validate(args ...Number) error {
	for i, arg := range args {
		if math.IsNaN(arg.Value) || math.IsInf(arg.Value, 0) {
			return &SafeMathError{Operation: "safe_subtract", Input: args, Cause: fmt.Errorf("argument %d is NaN or Inf", i)}
		}
	}
	return op.inner.Validate(args...)
}

func (op *SafeSubtractOperation) Execute(ctx context.Context, args ...Number) (*OperationResult, error) {
	if err := op.Validate(args...); err != nil {
		return nil, err
	}
	result, err := op.inner.Execute(ctx, args...)
	if err != nil {
		return nil, &SafeMathError{Operation: "safe_subtract", Input: args, Cause: err}
	}
	return result, nil
}

func (op *SafeSubtractOperation) ExecuteBinary(ctx context.Context, a, b Number) (*OperationResult, error) {
	return op.Execute(ctx, a, b)
}

// SafeDivideOperation wraps DivideOperation with stricter zero and NaN checks.
type SafeDivideOperation struct {
	inner    *DivideOperation
	epsilon  float64
}

func NewSafeDivideOperation(precision int, epsilon float64, audit bool) *SafeDivideOperation {
	return &SafeDivideOperation{
		inner:   NewDivideOperation(precision, true, audit),
		epsilon: epsilon,
	}
}

func (op *SafeDivideOperation) Name() string                { return "safe_divide" }
func (op *SafeDivideOperation) Description() string         { return "Safe Division - epsilon-based near-zero detection and NaN rejection" }
func (op *SafeDivideOperation) Category() OperationCategory { return CategoryArithmetic }
func (op *SafeDivideOperation) Arity() int                  { return 2 }

func (op *SafeDivideOperation) Validate(args ...Number) error {
	if len(args) != 2 {
		return fmt.Errorf("safe_divide requires exactly 2 arguments, got %d", len(args))
	}
	for i, arg := range args {
		if math.IsNaN(arg.Value) {
			return &SafeMathError{Operation: "safe_divide", Input: args, Cause: fmt.Errorf("argument %d is NaN", i)}
		}
		if math.IsInf(arg.Value, 0) {
			return &SafeMathError{Operation: "safe_divide", Input: args, Cause: fmt.Errorf("argument %d is Inf", i)}
		}
	}
	if math.Abs(args[1].Value) < op.epsilon {
		return &SafeMathError{
			Operation: "safe_divide",
			Input:     args,
			Cause:     fmt.Errorf("divisor %v is within epsilon %v of zero", args[1].Value, op.epsilon),
			Timestamp: time.Now(),
		}
	}
	return nil
}

func (op *SafeDivideOperation) Execute(ctx context.Context, args ...Number) (*OperationResult, error) {
	if err := op.Validate(args...); err != nil {
		return nil, err
	}
	result, err := op.inner.Execute(ctx, args...)
	if err != nil {
		return nil, &SafeMathError{Operation: "safe_divide", Input: args, Cause: err}
	}
	return result, nil
}

func (op *SafeDivideOperation) ExecuteBinary(ctx context.Context, a, b Number) (*OperationResult, error) {
	return op.Execute(ctx, a, b)
}

// SafeMultiplyOperation wraps MultiplyOperation with input validation.
type SafeMultiplyOperation struct {
	inner    *MultiplyOperation
	maxValue float64
}

func NewSafeMultiplyOperation(precision int, maxValue float64, audit bool) *SafeMultiplyOperation {
	return &SafeMultiplyOperation{
		inner:    NewMultiplyOperation(precision, true, audit),
		maxValue: maxValue,
	}
}

func (op *SafeMultiplyOperation) Name() string                { return "safe_multiply" }
func (op *SafeMultiplyOperation) Description() string         { return "Safe Multiplication - bounds checking and NaN rejection" }
func (op *SafeMultiplyOperation) Category() OperationCategory { return CategoryArithmetic }
func (op *SafeMultiplyOperation) Arity() int                  { return 2 }

func (op *SafeMultiplyOperation) Validate(args ...Number) error {
	for i, arg := range args {
		if math.IsNaN(arg.Value) || math.IsInf(arg.Value, 0) {
			return &SafeMathError{Operation: "safe_multiply", Input: args, Cause: fmt.Errorf("argument %d is NaN or Inf", i)}
		}
		if math.Abs(arg.Value) > op.maxValue {
			return &SafeMathError{Operation: "safe_multiply", Input: args, Cause: fmt.Errorf("argument %d exceeds max value %v", i, op.maxValue)}
		}
	}
	return op.inner.Validate(args...)
}

func (op *SafeMultiplyOperation) Execute(ctx context.Context, args ...Number) (*OperationResult, error) {
	if err := op.Validate(args...); err != nil {
		return nil, err
	}
	result, err := op.inner.Execute(ctx, args...)
	if err != nil {
		return nil, &SafeMathError{Operation: "safe_multiply", Input: args, Cause: err}
	}
	return result, nil
}

func (op *SafeMultiplyOperation) ExecuteBinary(ctx context.Context, a, b Number) (*OperationResult, error) {
	return op.Execute(ctx, a, b)
}

// SafePowerOperation wraps PowerOperation with stricter bounds.
type SafePowerOperation struct {
	inner       *PowerOperation
	maxBase     float64
	maxExponent float64
}

func NewSafePowerOperation(maxBase, maxExponent float64, audit bool) *SafePowerOperation {
	return &SafePowerOperation{
		inner:       NewPowerOperation(maxExponent, true, audit),
		maxBase:     maxBase,
		maxExponent: maxExponent,
	}
}

func (op *SafePowerOperation) Name() string                { return "safe_power" }
func (op *SafePowerOperation) Description() string         { return "Safe Exponentiation - base and exponent bounds checking" }
func (op *SafePowerOperation) Category() OperationCategory { return CategoryArithmetic }
func (op *SafePowerOperation) Arity() int                  { return 2 }

func (op *SafePowerOperation) Validate(args ...Number) error {
	if len(args) != 2 {
		return fmt.Errorf("safe_power requires exactly 2 arguments, got %d", len(args))
	}
	base, exp := args[0], args[1]
	if math.IsNaN(base.Value) || math.IsNaN(exp.Value) {
		return &SafeMathError{Operation: "safe_power", Input: args, Cause: fmt.Errorf("NaN input")}
	}
	if math.Abs(base.Value) > op.maxBase {
		return &SafeMathError{Operation: "safe_power", Input: args, Cause: fmt.Errorf("base %v exceeds max %v", base.Value, op.maxBase)}
	}
	if math.Abs(exp.Value) > op.maxExponent {
		return &SafeMathError{Operation: "safe_power", Input: args, Cause: fmt.Errorf("exponent %v exceeds max %v", exp.Value, op.maxExponent)}
	}
	if base.Value < 0 && exp.Value != math.Trunc(exp.Value) {
		return &SafeMathError{Operation: "safe_power", Input: args, Cause: fmt.Errorf("negative base with non-integer exponent")}
	}
	return nil
}

func (op *SafePowerOperation) Execute(ctx context.Context, args ...Number) (*OperationResult, error) {
	if err := op.Validate(args...); err != nil {
		return nil, err
	}
	result, err := op.inner.Execute(ctx, args...)
	if err != nil {
		return nil, &SafeMathError{Operation: "safe_power", Input: args, Cause: err}
	}
	return result, nil
}

// RunSafeChain executes a sequence of operations, stopping at the first error.
// Each operation receives the output of the previous one as its first argument.
func RunSafeChain(ctx context.Context, initial Number, ops []Operation, seconds []Number) (*OperationResult, []AuditEntry, error) {
	current := initial
	var trail []AuditEntry

	for i, op := range ops {
		var args []Number
		if i < len(seconds) {
			args = []Number{current, seconds[i]}
		} else {
			args = []Number{current}
		}

		result, err := op.Execute(ctx, args...)
		if err != nil {
			return nil, trail, &SafeMathError{
				Operation: op.Name(),
				Input:     args,
				Cause:     fmt.Errorf("chain step %d failed: %w", i, err),
				Timestamp: time.Now(),
			}
		}
		current = result.Value
		trail = append(trail, result.AuditTrail...)
	}

	return &OperationResult{
		Value:      current,
		AuditTrail: trail,
		Strategy:   "safe_chain",
	}, trail, nil
}
