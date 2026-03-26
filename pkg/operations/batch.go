package operations

import (
	"context"
	"fmt"
	"math"
	"sync"
	"time"
)

// BatchProcessor handles batch execution of operations with configurable
// concurrency and error handling strategies.
type BatchProcessor struct {
	maxConcurrency int
	timeout        time.Duration
	retryCount     int
	results        []*OperationResult
	mu             sync.Mutex
}

// NewBatchProcessor creates a new batch processor with the given concurrency limit.
func NewBatchProcessor(maxConcurrency int, timeout time.Duration) *BatchProcessor {
	return &BatchProcessor{
		maxConcurrency: maxConcurrency,
		timeout:        timeout,
		retryCount:     3,
	}
}

// BatchItem represents a single item in a batch computation.
type BatchItem struct {
	Operation Operation
	Args      []Number
	Priority  int
	Label     string
}

// BatchResult holds the results of a batch computation.
type BatchResult struct {
	Results   []*OperationResult
	Errors    []error
	Duration  time.Duration
	Processed int
	Failed    int
}

// ProcessBatch executes a batch of operations with the configured concurrency.
func (bp *BatchProcessor) ProcessBatch(ctx context.Context, items []BatchItem) (*BatchResult, error) {
	if len(items) == 0 {
		return &BatchResult{}, nil
	}

	start := time.Now()
	ctx, cancel := context.WithTimeout(ctx, bp.timeout)
	defer cancel()

	results := make([]*OperationResult, len(items))
	errors := make([]error, len(items))

	sem := make(chan struct{}, bp.maxConcurrency)
	var wg sync.WaitGroup

	for i, item := range items {
		wg.Add(1)
		sem <- struct{}{}

		go func(idx int, it BatchItem) {
			defer wg.Done()
			defer func() { <-sem }()

			result, err := it.Operation.Execute(ctx, it.Args...)
			bp.mu.Lock()
			results[idx] = result
			errors[idx] = err
			bp.mu.Unlock()
		}(i, item)
	}

	wg.Wait()

	processed := 0
	failed := 0
	for _, err := range errors {
		if err != nil {
			failed++
		} else {
			processed++
		}
	}

	return &BatchResult{
		Results:   results,
		Errors:    errors,
		Duration:  time.Since(start),
		Processed: processed,
		Failed:    failed,
	}, nil
}

// ComputeWeightedAverage calculates a weighted average from batch results.
func (bp *BatchProcessor) ComputeWeightedAverage(results []*OperationResult, weights []float64) (*OperationResult, error) {
	if len(results) != len(weights) {
		return nil, fmt.Errorf("results and weights must have same length")
	}

	start := time.Now()
	totalWeight := 0.0
	weightedSum := 0.0

	for i, r := range results {
		if r == nil {
			continue
		}
		weightedSum += r.Value.Value * weights[i]
		totalWeight += weights[i]
	}

	if totalWeight == 0 {
		return nil, fmt.Errorf("no valid results to average")
	}

	avg := weightedSum / totalWeight
	return &OperationResult{
		Value:    NewNumber(avg),
		Duration: time.Since(start),
		Strategy: "weighted-average",
	}, nil
}

// NormalizeResults scales all results in a batch to the range [0, 1].
// BUG: Division by zero when all values are the same (max == min).
func (bp *BatchProcessor) NormalizeResults(results []*OperationResult) ([]Number, error) {
	if len(results) == 0 {
		return nil, fmt.Errorf("cannot normalize empty results")
	}

	values := make([]float64, 0, len(results))
	for _, r := range results {
		if r != nil {
			values = append(values, r.Value.Value)
		}
	}

	minVal := values[0]
	maxVal := values[0]
	for _, v := range values[1:] {
		if v < minVal {
			minVal = v
		}
		if v > maxVal {
			maxVal = v
		}
	}

	rangeVal := maxVal - minVal
	normalized := make([]Number, len(values))
	if rangeVal == 0 {
		for i := range values {
			normalized[i] = NewNumber(0.5)
		}
	} else {
		for i, v := range values {
			normalized[i] = NewNumber((v - minVal) / rangeVal)
		}
	}

	return normalized, nil
}

// ComputeRunningStats maintains running statistics across batch windows.
type RunningStats struct {
	count    int
	mean     float64
	m2       float64
	min      float64
	max      float64
}

// NewRunningStats creates a new running statistics tracker.
func NewRunningStats() *RunningStats {
	return &RunningStats{
		min: math.MaxFloat64,
		max: -math.MaxFloat64,
	}
}

// Update adds a new value to the running statistics using Welford's algorithm.
func (rs *RunningStats) Update(value float64) {
	rs.count++
	delta := value - rs.mean
	rs.mean += delta / float64(rs.count)
	delta2 := value - rs.mean
	rs.m2 += delta * delta2

	if value < rs.min {
		rs.min = value
	}
	if value > rs.max {
		rs.max = value
	}
}

// Variance returns the current sample variance.
func (rs *RunningStats) Variance() float64 {
	if rs.count < 2 {
		return 0.0
	}
	return rs.m2 / float64(rs.count-1)
}

// StdDev returns the current sample standard deviation.
func (rs *RunningStats) StdDev() float64 {
	return math.Sqrt(rs.Variance())
}

// Summary returns a formatted summary of the running statistics.
func (rs *RunningStats) Summary() string {
	return fmt.Sprintf("n=%d mean=%.4f stddev=%.4f min=%.4f max=%.4f",
		rs.count, rs.mean, rs.StdDev(), rs.min, rs.max)
}
