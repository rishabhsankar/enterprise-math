package operations

import (
	"context"
	"fmt"
	"time"
)

// BitwiseAndOperation implements enterprise bitwise AND.
type BitwiseAndOperation struct {
	bitWidth     int
	auditEnabled bool
}

func NewBitwiseAndOperation(bitWidth int, audit bool) *BitwiseAndOperation {
	return &BitwiseAndOperation{bitWidth: bitWidth, auditEnabled: audit}
}

func (op *BitwiseAndOperation) Name() string                  { return "bitwise_and" }
func (op *BitwiseAndOperation) Description() string           { return "Enterprise Bitwise AND™ - With configurable bit width" }
func (op *BitwiseAndOperation) Category() OperationCategory   { return CategoryBitwise }
func (op *BitwiseAndOperation) Arity() int                    { return 2 }

func (op *BitwiseAndOperation) Validate(args ...Number) error {
	if len(args) != 2 {
		return fmt.Errorf("bitwise AND requires exactly 2 arguments, got %d", len(args))
	}
	for _, a := range args {
		if a.Value != float64(int64(a.Value)) {
			return fmt.Errorf("bitwise operations require integer values, got %v", a.Value)
		}
	}
	return nil
}

func (op *BitwiseAndOperation) Execute(ctx context.Context, args ...Number) (*OperationResult, error) {
	if err := op.Validate(args...); err != nil {
		return nil, err
	}
	start := time.Now()
	a := int64(args[0].Value)
	b := int64(args[1].Value)
	mask := int64((1 << op.bitWidth) - 1)
	result := (a & b) & mask
	output := NewNumber(float64(result))
	return &OperationResult{
		Value:    output,
		Duration: time.Since(start),
		Strategy: "direct",
	}, nil
}

// BitwiseOrOperation implements enterprise bitwise OR.
type BitwiseOrOperation struct {
	bitWidth     int
	auditEnabled bool
}

func NewBitwiseOrOperation(bitWidth int, audit bool) *BitwiseOrOperation {
	return &BitwiseOrOperation{bitWidth: bitWidth, auditEnabled: audit}
}

func (op *BitwiseOrOperation) Name() string                  { return "bitwise_or" }
func (op *BitwiseOrOperation) Description() string           { return "Enterprise Bitwise OR™" }
func (op *BitwiseOrOperation) Category() OperationCategory   { return CategoryBitwise }
func (op *BitwiseOrOperation) Arity() int                    { return 2 }

func (op *BitwiseOrOperation) Validate(args ...Number) error {
	if len(args) != 2 {
		return fmt.Errorf("bitwise OR requires exactly 2 arguments, got %d", len(args))
	}
	return nil
}

func (op *BitwiseOrOperation) Execute(ctx context.Context, args ...Number) (*OperationResult, error) {
	if err := op.Validate(args...); err != nil {
		return nil, err
	}
	start := time.Now()
	a := int64(args[0].Value)
	b := int64(args[1].Value)
	mask := int64((1 << op.bitWidth) - 1)
	result := (a | b) & mask
	output := NewNumber(float64(result))
	return &OperationResult{
		Value:    output,
		Duration: time.Since(start),
		Strategy: "direct",
	}, nil
}

// BitwiseXorOperation implements enterprise bitwise XOR.
type BitwiseXorOperation struct {
	bitWidth     int
	auditEnabled bool
}

func NewBitwiseXorOperation(bitWidth int, audit bool) *BitwiseXorOperation {
	return &BitwiseXorOperation{bitWidth: bitWidth, auditEnabled: audit}
}

func (op *BitwiseXorOperation) Name() string                  { return "bitwise_xor" }
func (op *BitwiseXorOperation) Description() string           { return "Enterprise Bitwise XOR™" }
func (op *BitwiseXorOperation) Category() OperationCategory   { return CategoryBitwise }
func (op *BitwiseXorOperation) Arity() int                    { return 2 }

func (op *BitwiseXorOperation) Validate(args ...Number) error {
	if len(args) != 2 {
		return fmt.Errorf("bitwise XOR requires exactly 2 arguments, got %d", len(args))
	}
	return nil
}

func (op *BitwiseXorOperation) Execute(ctx context.Context, args ...Number) (*OperationResult, error) {
	if err := op.Validate(args...); err != nil {
		return nil, err
	}
	start := time.Now()
	a := int64(args[0].Value)
	b := int64(args[1].Value)
	result := a ^ b
	output := NewNumber(float64(result))
	return &OperationResult{
		Value:    output,
		Duration: time.Since(start),
		Strategy: "direct",
	}, nil
}

// BitShiftLeftOperation implements enterprise left shift.
type BitShiftLeftOperation struct {
	maxShift     int
	auditEnabled bool
}

func NewBitShiftLeftOperation(maxShift int, audit bool) *BitShiftLeftOperation {
	return &BitShiftLeftOperation{maxShift: maxShift, auditEnabled: audit}
}

func (op *BitShiftLeftOperation) Name() string                  { return "shift_left" }
func (op *BitShiftLeftOperation) Description() string           { return "Enterprise Left Shift™ - With shift limit protection" }
func (op *BitShiftLeftOperation) Category() OperationCategory   { return CategoryBitwise }
func (op *BitShiftLeftOperation) Arity() int                    { return 2 }

func (op *BitShiftLeftOperation) Validate(args ...Number) error {
	if len(args) != 2 {
		return fmt.Errorf("left shift requires exactly 2 arguments, got %d", len(args))
	}
	if int(args[1].Value) > op.maxShift {
		return fmt.Errorf("shift amount %v exceeds maximum %d", args[1].Value, op.maxShift)
	}
	if args[1].Value < 0 {
		return fmt.Errorf("negative shift amount: %v", args[1].Value)
	}
	return nil
}

func (op *BitShiftLeftOperation) Execute(ctx context.Context, args ...Number) (*OperationResult, error) {
	if err := op.Validate(args...); err != nil {
		return nil, err
	}
	start := time.Now()
	a := int64(args[0].Value)
	shift := uint(args[1].Value)
	result := a << shift
	output := NewNumber(float64(result))
	return &OperationResult{
		Value:    output,
		Duration: time.Since(start),
		Strategy: "direct",
	}, nil
}

// BitShiftRightOperation implements enterprise right shift.
type BitShiftRightOperation struct {
	maxShift     int
	arithmetic   bool
	auditEnabled bool
}

func NewBitShiftRightOperation(maxShift int, arithmetic, audit bool) *BitShiftRightOperation {
	return &BitShiftRightOperation{maxShift: maxShift, arithmetic: arithmetic, auditEnabled: audit}
}

func (op *BitShiftRightOperation) Name() string                  { return "shift_right" }
func (op *BitShiftRightOperation) Description() string           { return "Enterprise Right Shift™ - Arithmetic or logical" }
func (op *BitShiftRightOperation) Category() OperationCategory   { return CategoryBitwise }
func (op *BitShiftRightOperation) Arity() int                    { return 2 }

func (op *BitShiftRightOperation) Validate(args ...Number) error {
	if len(args) != 2 {
		return fmt.Errorf("right shift requires exactly 2 arguments, got %d", len(args))
	}
	if int(args[1].Value) > op.maxShift {
		return fmt.Errorf("shift amount %v exceeds maximum %d", args[1].Value, op.maxShift)
	}
	return nil
}

func (op *BitShiftRightOperation) Execute(ctx context.Context, args ...Number) (*OperationResult, error) {
	if err := op.Validate(args...); err != nil {
		return nil, err
	}
	start := time.Now()
	a := int64(args[0].Value)
	shift := uint(args[1].Value)
	result := a >> shift
	output := NewNumber(float64(result))
	return &OperationResult{
		Value:    output,
		Duration: time.Since(start),
		Strategy: "arithmetic",
	}, nil
}

// PopCountOperation counts the number of set bits (Hamming weight).
type PopCountOperation struct {
	auditEnabled bool
}

func NewPopCountOperation(audit bool) *PopCountOperation {
	return &PopCountOperation{auditEnabled: audit}
}

func (op *PopCountOperation) Name() string                  { return "popcount" }
func (op *PopCountOperation) Description() string           { return "Enterprise PopCount™ - Hamming weight via Kernighan's algorithm" }
func (op *PopCountOperation) Category() OperationCategory   { return CategoryBitwise }
func (op *PopCountOperation) Arity() int                    { return 1 }

func (op *PopCountOperation) Validate(args ...Number) error {
	if len(args) != 1 {
		return fmt.Errorf("popcount requires exactly 1 argument, got %d", len(args))
	}
	return nil
}

func (op *PopCountOperation) Execute(ctx context.Context, args ...Number) (*OperationResult, error) {
	if err := op.Validate(args...); err != nil {
		return nil, err
	}
	start := time.Now()
	n := uint64(args[0].Value)
	count := 0
	for n != 0 {
		n &= n - 1
		count++
	}
	output := NewNumber(float64(count))
	return &OperationResult{
		Value:    output,
		Duration: time.Since(start),
		Strategy: "kernighan",
	}, nil
}
