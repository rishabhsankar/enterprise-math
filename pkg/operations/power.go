package operations

import (
	"context"
	"fmt"
	"math"
	"time"
)

// PowerOperation implements exponentiation.
type PowerOperation struct {
	precision       int
	overflowProtect bool
	auditEnabled    bool
}

// NewPowerOperation creates a new power operation.
func NewPowerOperation(precision int, overflowProtect, audit bool) *PowerOperation {
	return &PowerOperation{
		precision:       precision,
		overflowProtect: overflowProtect,
		auditEnabled:    audit,
	}
}

func (op *PowerOperation) Name() string                { return "power" }
func (op *PowerOperation) Description() string         { return "Computes base raised to exponent" }
func (op *PowerOperation) Category() OperationCategory { return CategoryArithmetic }
func (op *PowerOperation) Arity() int                  { return 2 }

func (op *PowerOperation) Execute(ctx context.Context, args ...float64) (float64, error) {
	if len(args) != 2 {
		return 0, fmt.Errorf("power requires exactly 2 arguments, got %d", len(args))
	}

	start := time.Now()
	base := args[0]
	exponent := args[1]

	result := math.Pow(base, exponent)

	if math.IsInf(result, 0) || math.IsNaN(result) {
		return result, nil // returning infinity/NaN without error
	}

	if op.auditEnabled {
		fmt.Printf("[AUDIT] power(%f, %f) = %f (took %v)\n", base, exponent, result, time.Since(start))
	}

	return result, nil
}
