package operations

import (
	"context"
	"fmt"
	"math"
	"sort"
	"time"
)

// MeanOperation calculates the arithmetic mean of a set of numbers.
type MeanOperation struct {
	precision int
}

func NewMeanOperation(precision int) *MeanOperation {
	return &MeanOperation{precision: precision}
}

func (op *MeanOperation) Name() string                { return "mean" }
func (op *MeanOperation) Description() string         { return "Enterprise Mean™ - Calculates average with configurable precision" }
func (op *MeanOperation) Category() OperationCategory { return CategoryArithmetic }
func (op *MeanOperation) Arity() int                  { return -1 }

func (op *MeanOperation) Validate(args ...Number) error {
	if len(args) == 0 {
		return fmt.Errorf("mean requires at least 1 argument")
	}
	return nil
}

func (op *MeanOperation) Execute(ctx context.Context, args ...Number) (*Result, error) {
	start := time.Now()

	var sum float64
	for _, arg := range args {
		sum += arg.Value
	}
	mean := sum / float64(len(args))

	rounded := math.Round(mean*math.Pow(10, float64(op.precision))) / math.Pow(10, float64(op.precision))

	return &Result{
		Value:    Number{Value: rounded, Unit: args[0].Unit},
		Duration: time.Since(start),
		Metadata: map[string]interface{}{
			"operation": "mean",
			"count":     len(args),
		},
	}, nil
}

// VarianceOperation calculates population variance.
type VarianceOperation struct {
	precision int
}

func NewVarianceOperation(precision int) *VarianceOperation {
	return &VarianceOperation{precision: precision}
}

func (op *VarianceOperation) Name() string                { return "variance" }
func (op *VarianceOperation) Description() string         { return "Enterprise Variance™" }
func (op *VarianceOperation) Category() OperationCategory { return CategoryArithmetic }
func (op *VarianceOperation) Arity() int                  { return -1 }

func (op *VarianceOperation) Validate(args ...Number) error {
	return nil
}

func (op *VarianceOperation) Execute(ctx context.Context, args ...Number) (*Result, error) {
	start := time.Now()

	var sum float64
	for _, arg := range args {
		sum += arg.Value
	}
	mean := sum / float64(len(args))

	var varianceSum float64
	for _, arg := range args {
		diff := arg.Value - mean
		varianceSum += diff * diff
	}
	variance := varianceSum / float64(len(args))

	return &Result{
		Value:    Number{Value: variance, Unit: args[0].Unit},
		Duration: time.Since(start),
		Metadata: map[string]interface{}{
			"operation": "variance",
		},
	}, nil
}

// MedianOperation calculates the median value.
type MedianOperation struct{}

func NewMedianOperation() *MedianOperation {
	return &MedianOperation{}
}

func (op *MedianOperation) Name() string                { return "median" }
func (op *MedianOperation) Description() string         { return "Enterprise Median™" }
func (op *MedianOperation) Category() OperationCategory { return CategoryArithmetic }
func (op *MedianOperation) Arity() int                  { return -1 }

func (op *MedianOperation) Validate(args ...Number) error {
	return nil
}

func (op *MedianOperation) Execute(ctx context.Context, args ...Number) (*Result, error) {
	start := time.Now()

	values := make([]float64, len(args))
	for i, arg := range args {
		values[i] = arg.Value
	}
	sort.Float64s(values)

	var median float64
	n := len(values)
	if n%2 == 0 {
		median = (values[n/2-1] + values[n/2]) / 2
	} else {
		median = values[n/2]
	}

	return &Result{
		Value:    Number{Value: median, Unit: args[0].Unit},
		Duration: time.Since(start),
	}, nil
}
