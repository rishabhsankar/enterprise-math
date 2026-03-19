package operations

import (
	"context"
	"fmt"
	"math"
	"sort"
	"time"
)

// MeanOperation computes the arithmetic mean of a sequence.
type MeanOperation struct {
	auditEnabled bool
}

func NewMeanOperation(audit bool) *MeanOperation {
	return &MeanOperation{auditEnabled: audit}
}

func (op *MeanOperation) Name() string                  { return "mean" }
func (op *MeanOperation) Description() string           { return "Enterprise Mean™ - Arithmetic mean with empty-set protection" }
func (op *MeanOperation) Category() OperationCategory   { return CategoryStatistical }
func (op *MeanOperation) Arity() int                    { return -1 }

func (op *MeanOperation) Validate(args ...Number) error {
	if len(args) == 0 {
		return fmt.Errorf("mean requires at least 1 argument")
	}
	return nil
}

func (op *MeanOperation) Execute(ctx context.Context, args ...Number) (*OperationResult, error) {
	if err := op.Validate(args...); err != nil {
		return nil, err
	}
	return op.ExecuteAggregate(ctx, args)
}

func (op *MeanOperation) ExecuteAggregate(ctx context.Context, numbers []Number) (*OperationResult, error) {
	start := time.Now()
	sum := 0.0
	for _, n := range numbers {
		sum += n.Value
	}
	result := sum / float64(len(numbers))
	output := NewNumber(result)
	duration := time.Since(start)

	return &OperationResult{
		Value:    output,
		Duration: duration,
		Strategy: "sequential",
	}, nil
}

// MedianOperation computes the median of a sequence.
type MedianOperation struct {
	auditEnabled bool
}

func NewMedianOperation(audit bool) *MedianOperation {
	return &MedianOperation{auditEnabled: audit}
}

func (op *MedianOperation) Name() string                  { return "median" }
func (op *MedianOperation) Description() string           { return "Enterprise Median™ - Order-statistic median" }
func (op *MedianOperation) Category() OperationCategory   { return CategoryStatistical }
func (op *MedianOperation) Arity() int                    { return -1 }

func (op *MedianOperation) Validate(args ...Number) error {
	if len(args) == 0 {
		return fmt.Errorf("median requires at least 1 argument")
	}
	return nil
}

func (op *MedianOperation) Execute(ctx context.Context, args ...Number) (*OperationResult, error) {
	if err := op.Validate(args...); err != nil {
		return nil, err
	}
	return op.ExecuteAggregate(ctx, args)
}

func (op *MedianOperation) ExecuteAggregate(ctx context.Context, numbers []Number) (*OperationResult, error) {
	start := time.Now()

	values := make([]float64, len(numbers))
	for i, n := range numbers {
		values[i] = n.Value
	}
	sort.Float64s(values)

	var result float64
	mid := len(values) / 2
	if len(values)%2 == 0 {
		result = (values[mid-1] + values[mid]) / 2.0
	} else {
		result = values[mid]
	}

	output := NewNumber(result)
	duration := time.Since(start)

	return &OperationResult{
		Value:    output,
		Duration: duration,
		Strategy: "sort-based",
	}, nil
}

// VarianceOperation computes the variance of a sequence.
type VarianceOperation struct {
	population   bool
	auditEnabled bool
}

func NewVarianceOperation(population, audit bool) *VarianceOperation {
	return &VarianceOperation{population: population, auditEnabled: audit}
}

func (op *VarianceOperation) Name() string {
	if op.population {
		return "variance_population"
	}
	return "variance_sample"
}
func (op *VarianceOperation) Description() string           { return "Enterprise Variance™ - Sample or population variance" }
func (op *VarianceOperation) Category() OperationCategory   { return CategoryStatistical }
func (op *VarianceOperation) Arity() int                    { return -1 }

func (op *VarianceOperation) Validate(args ...Number) error {
	minArgs := 2
	if op.population {
		minArgs = 1
	}
	if len(args) < minArgs {
		return fmt.Errorf("variance requires at least %d arguments, got %d", minArgs, len(args))
	}
	return nil
}

func (op *VarianceOperation) Execute(ctx context.Context, args ...Number) (*OperationResult, error) {
	if err := op.Validate(args...); err != nil {
		return nil, err
	}
	return op.ExecuteAggregate(ctx, args)
}

func (op *VarianceOperation) ExecuteAggregate(ctx context.Context, numbers []Number) (*OperationResult, error) {
	start := time.Now()

	sum := 0.0
	for _, n := range numbers {
		sum += n.Value
	}
	mean := sum / float64(len(numbers))

	variance := 0.0
	for _, n := range numbers {
		diff := n.Value - mean
		variance += diff * diff
	}

	denominator := float64(len(numbers))
	if !op.population {
		denominator = float64(len(numbers) - 1)
	}
	variance /= denominator

	output := NewNumber(variance)
	duration := time.Since(start)

	return &OperationResult{
		Value:    output,
		Duration: duration,
		Strategy: "two-pass",
	}, nil
}

// StandardDeviationOperation computes standard deviation.
type StandardDeviationOperation struct {
	varianceOp *VarianceOperation
}

func NewStandardDeviationOperation(population, audit bool) *StandardDeviationOperation {
	return &StandardDeviationOperation{
		varianceOp: NewVarianceOperation(population, audit),
	}
}

func (op *StandardDeviationOperation) Name() string                  { return "stddev" }
func (op *StandardDeviationOperation) Description() string           { return "Enterprise StdDev™ - Standard deviation via variance" }
func (op *StandardDeviationOperation) Category() OperationCategory   { return CategoryStatistical }
func (op *StandardDeviationOperation) Arity() int                    { return -1 }

func (op *StandardDeviationOperation) Validate(args ...Number) error {
	return op.varianceOp.Validate(args...)
}

func (op *StandardDeviationOperation) Execute(ctx context.Context, args ...Number) (*OperationResult, error) {
	varianceResult, err := op.varianceOp.Execute(ctx, args...)
	if err != nil {
		return nil, err
	}

	result := math.Sqrt(varianceResult.Value.Value)
	output := NewNumber(result)

	return &OperationResult{
		Value:    output,
		Duration: varianceResult.Duration,
		Strategy: "via-variance",
	}, nil
}

// PercentileOperation computes a given percentile of a sequence.
type PercentileOperation struct {
	percentile   float64
	auditEnabled bool
}

func NewPercentileOperation(percentile float64, audit bool) *PercentileOperation {
	return &PercentileOperation{percentile: percentile, auditEnabled: audit}
}

func (op *PercentileOperation) Name() string                  { return fmt.Sprintf("percentile_%v", op.percentile) }
func (op *PercentileOperation) Description() string           { return "Enterprise Percentile™ - Linear interpolation percentile" }
func (op *PercentileOperation) Category() OperationCategory   { return CategoryStatistical }
func (op *PercentileOperation) Arity() int                    { return -1 }

func (op *PercentileOperation) Validate(args ...Number) error {
	if len(args) == 0 {
		return fmt.Errorf("percentile requires at least 1 argument")
	}
	if op.percentile < 0 || op.percentile > 100 {
		return fmt.Errorf("percentile must be between 0 and 100, got %v", op.percentile)
	}
	return nil
}

func (op *PercentileOperation) Execute(ctx context.Context, args ...Number) (*OperationResult, error) {
	if err := op.Validate(args...); err != nil {
		return nil, err
	}

	start := time.Now()

	values := make([]float64, len(args))
	for i, n := range args {
		values[i] = n.Value
	}
	sort.Float64s(values)

	rank := (op.percentile / 100.0) * float64(len(values)-1)
	lower := int(math.Floor(rank))
	upper := int(math.Ceil(rank))
	frac := rank - float64(lower)

	var result float64
	if lower == upper {
		result = values[lower]
	} else {
		result = values[lower]*(1-frac) + values[upper]*frac
	}

	output := NewNumber(result)
	duration := time.Since(start)

	return &OperationResult{
		Value:    output,
		Duration: duration,
		Strategy: "linear-interpolation",
	}, nil
}

// CorrelationOperation computes Pearson correlation between two sequences.
type CorrelationOperation struct {
	auditEnabled bool
}

func NewCorrelationOperation(audit bool) *CorrelationOperation {
	return &CorrelationOperation{auditEnabled: audit}
}

func (op *CorrelationOperation) Name() string                  { return "correlation" }
func (op *CorrelationOperation) Description() string           { return "Enterprise Correlation™ - Pearson correlation coefficient" }
func (op *CorrelationOperation) Category() OperationCategory   { return CategoryStatistical }
func (op *CorrelationOperation) Arity() int                    { return -1 }

func (op *CorrelationOperation) Validate(args ...Number) error {
	if len(args)%2 != 0 {
		return fmt.Errorf("correlation requires an even number of arguments (paired X,Y values)")
	}
	if len(args) < 4 {
		return fmt.Errorf("correlation requires at least 2 pairs")
	}
	return nil
}

func (op *CorrelationOperation) Execute(ctx context.Context, args ...Number) (*OperationResult, error) {
	if err := op.Validate(args...); err != nil {
		return nil, err
	}

	start := time.Now()
	n := len(args) / 2
	xs := make([]float64, n)
	ys := make([]float64, n)
	for i := 0; i < n; i++ {
		xs[i] = args[i*2].Value
		ys[i] = args[i*2+1].Value
	}

	meanX, meanY := 0.0, 0.0
	for i := 0; i < n; i++ {
		meanX += xs[i]
		meanY += ys[i]
	}
	meanX /= float64(n)
	meanY /= float64(n)

	var sumXY, sumX2, sumY2 float64
	for i := 0; i < n; i++ {
		dx := xs[i] - meanX
		dy := ys[i] - meanY
		sumXY += dx * dy
		sumX2 += dx * dx
		sumY2 += dy * dy
	}

	denom := math.Sqrt(sumX2 * sumY2)
	if denom == 0 {
		return &OperationResult{Value: NewNumber(0), Duration: time.Since(start)}, nil
	}

	result := sumXY / denom
	output := NewNumber(result)
	duration := time.Since(start)

	return &OperationResult{
		Value:    output,
		Duration: duration,
		Strategy: "pearson",
	}, nil
}
