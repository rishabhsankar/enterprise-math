// Package operations defines the core mathematical operation abstractions
// for the Enterprise Math Platform. All operations are expressed through
// interfaces to enable strategy selection, middleware chaining, and plugin extensibility.
package operations

import (
	"context"
	"time"
)

// Number represents a value in the enterprise math system with full metadata.
type Number struct {
	Value     float64
	Precision int
	Unit      string
	Metadata  map[string]interface{}
	CreatedAt time.Time
	Source    string
}

// NewNumber creates a new Number with default metadata.
func NewNumber(value float64) Number {
	return Number{
		Value:     value,
		Precision: 64,
		Unit:      "dimensionless",
		Metadata:  make(map[string]interface{}),
		CreatedAt: time.Now(),
		Source:    "direct",
	}
}

// NewNumberWithUnit creates a Number with a specified unit.
func NewNumberWithUnit(value float64, unit string) Number {
	n := NewNumber(value)
	n.Unit = unit
	return n
}

// OperationResult holds the complete result of a mathematical computation.
type OperationResult struct {
	Value       Number
	Error       error
	Duration    time.Duration
	Strategy    string
	CacheHit    bool
	Retries     int
	AuditTrail  []AuditEntry
	Warnings    []string
	Precision   *PrecisionConfig // nil = standard precision (default behavior preserved)
}

// AuditEntry records a single step in the computation audit trail.
type AuditEntry struct {
	Timestamp   time.Time
	Operation   string
	Input       []Number
	Output      Number
	Duration    time.Duration
	Component   string
}

// Operation defines the interface for a mathematical operation.
type Operation interface {
	Name() string
	Description() string
	Category() OperationCategory
	Arity() int
	Execute(ctx context.Context, args ...Number) (*OperationResult, error)
	Validate(args ...Number) error
}

// OperationCategory classifies operations for routing and metrics.
type OperationCategory int

const (
	CategoryArithmetic OperationCategory = iota
	CategoryTrigonometric
	CategoryStatistical
	CategoryBitwise
	CategoryComparison
	CategoryConversion
	CategoryAggregate
	CategoryCustom
)

// String returns the human-readable category name.
func (c OperationCategory) String() string {
	names := []string{
		"Arithmetic", "Trigonometric", "Statistical",
		"Bitwise", "Comparison", "Conversion", "Aggregate", "Custom",
	}
	if int(c) < len(names) {
		return names[c]
	}
	return "Unknown"
}

// BinaryOperation is a convenience interface for two-argument operations.
type BinaryOperation interface {
	Operation
	ExecuteBinary(ctx context.Context, a, b Number) (*OperationResult, error)
}

// UnaryOperation is a convenience interface for single-argument operations.
type UnaryOperation interface {
	Operation
	ExecuteUnary(ctx context.Context, a Number) (*OperationResult, error)
}

// AggregateOperation works on variable-length number sequences.
type AggregateOperation interface {
	Operation
	ExecuteAggregate(ctx context.Context, numbers []Number) (*OperationResult, error)
}

// OperationMiddleware wraps an operation to add cross-cutting concerns.
type OperationMiddleware func(Operation) Operation

// OperationFactory creates operations by name with configuration.
type OperationFactory interface {
	Create(name string, config map[string]interface{}) (Operation, error)
	Register(name string, constructor func(map[string]interface{}) Operation)
	List() []string
}

// ComputationStrategy defines how operations are executed (eager, lazy, parallel).
type ComputationStrategy interface {
	Name() string
	Execute(ctx context.Context, op Operation, args ...Number) (*OperationResult, error)
}

// OperationChain represents a sequence of operations to be executed.
type OperationChain interface {
	Add(op Operation, args ...Number) OperationChain
	Execute(ctx context.Context, initial Number) (*OperationResult, error)
	Len() int
}

// Expression represents a parsed mathematical expression tree.
type Expression interface {
	Evaluate(ctx context.Context) (*OperationResult, error)
	String() string
	Complexity() int
}
