// Package strategy provides computation execution strategies including
// eager, lazy, parallel, and cached evaluation approaches.
package strategy

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/rishabhsankar/enterprise-math/pkg/logging"
	"github.com/rishabhsankar/enterprise-math/pkg/operations"
)

// EagerStrategy executes operations immediately and returns the result.
type EagerStrategy struct {
	logger logging.Logger
}

func NewEagerStrategy(logger logging.Logger) *EagerStrategy {
	return &EagerStrategy{logger: logger}
}

func (s *EagerStrategy) Name() string { return "eager" }

func (s *EagerStrategy) Execute(ctx context.Context, op operations.Operation, args ...operations.Number) (*operations.OperationResult, error) {
	s.logger.Debug("Executing operation eagerly", map[string]interface{}{
		"operation": op.Name(),
		"args":      len(args),
	})

	start := time.Now()
	result, err := op.Execute(ctx, args...)
	if err != nil {
		return nil, fmt.Errorf("eager execution of %s failed: %w", op.Name(), err)
	}
	result.Strategy = "eager"
	result.Duration = time.Since(start)
	return result, nil
}

// RetryStrategy wraps another strategy with retry logic.
type RetryStrategy struct {
	inner      operations.ComputationStrategy
	maxRetries int
	backoff    time.Duration
	logger     logging.Logger
}

func NewRetryStrategy(inner operations.ComputationStrategy, maxRetries int, backoff time.Duration, logger logging.Logger) *RetryStrategy {
	return &RetryStrategy{
		inner:      inner,
		maxRetries: maxRetries,
		backoff:    backoff,
		logger:     logger,
	}
}

func (s *RetryStrategy) Name() string { return fmt.Sprintf("retry(%s)", s.inner.Name()) }

func (s *RetryStrategy) Execute(ctx context.Context, op operations.Operation, args ...operations.Number) (*operations.OperationResult, error) {
	var lastErr error
	for attempt := 0; attempt <= s.maxRetries; attempt++ {
		if attempt > 0 {
			s.logger.Warn("Retrying operation", map[string]interface{}{
				"operation": op.Name(),
				"attempt":   attempt,
				"max":       s.maxRetries,
			})
			select {
			case <-time.After(s.backoff * time.Duration(attempt)):
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}

		result, err := s.inner.Execute(ctx, op, args...)
		if err == nil {
			result.Retries = attempt
			return result, nil
		}
		lastErr = err
	}
	return nil, fmt.Errorf("operation %s failed after %d retries: %w", op.Name(), s.maxRetries, lastErr)
}

// TimeoutStrategy wraps another strategy with a timeout.
type TimeoutStrategy struct {
	inner   operations.ComputationStrategy
	timeout time.Duration
	logger  logging.Logger
}

func NewTimeoutStrategy(inner operations.ComputationStrategy, timeout time.Duration, logger logging.Logger) *TimeoutStrategy {
	return &TimeoutStrategy{inner: inner, timeout: timeout, logger: logger}
}

func (s *TimeoutStrategy) Name() string { return fmt.Sprintf("timeout(%s)", s.inner.Name()) }

func (s *TimeoutStrategy) Execute(ctx context.Context, op operations.Operation, args ...operations.Number) (*operations.OperationResult, error) {
	ctx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()

	resultCh := make(chan *operations.OperationResult, 1)
	errCh := make(chan error, 1)

	go func() {
		result, err := s.inner.Execute(ctx, op, args...)
		if err != nil {
			errCh <- err
			return
		}
		resultCh <- result
	}()

	select {
	case result := <-resultCh:
		return result, nil
	case err := <-errCh:
		return nil, err
	case <-ctx.Done():
		return nil, fmt.Errorf("operation %s timed out after %v", op.Name(), s.timeout)
	}
}

// ParallelStrategy executes multiple operations concurrently and aggregates results.
type ParallelStrategy struct {
	workerCount int
	logger      logging.Logger
}

func NewParallelStrategy(workerCount int, logger logging.Logger) *ParallelStrategy {
	return &ParallelStrategy{workerCount: workerCount, logger: logger}
}

func (s *ParallelStrategy) Name() string { return "parallel" }

func (s *ParallelStrategy) Execute(ctx context.Context, op operations.Operation, args ...operations.Number) (*operations.OperationResult, error) {
	return op.Execute(ctx, args...)
}

// ExecuteBatch runs multiple operations in parallel with a worker pool.
func (s *ParallelStrategy) ExecuteBatch(ctx context.Context, tasks []BatchTask) ([]*operations.OperationResult, error) {
	results := make([]*operations.OperationResult, len(tasks))
	errs := make([]error, len(tasks))
	var wg sync.WaitGroup

	sem := make(chan struct{}, s.workerCount)

	for i, task := range tasks {
		wg.Add(1)
		go func(idx int, t BatchTask) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			result, err := t.Operation.Execute(ctx, t.Args...)
			if err != nil {
				errs[idx] = err
				return
			}
			results[idx] = result
		}(i, task)
	}

	wg.Wait()

	for i, err := range errs {
		if err != nil {
			return nil, fmt.Errorf("task %d failed: %w", i, err)
		}
	}

	return results, nil
}

// BatchTask represents a single task for parallel execution.
type BatchTask struct {
	Operation operations.Operation
	Args      []operations.Number
}

// CircuitBreakerStrategy prevents cascading failures by tracking error rates.
type CircuitBreakerStrategy struct {
	inner          operations.ComputationStrategy
	threshold      int
	resetTimeout   time.Duration
	failureCount   int
	state          CircuitState
	lastFailure    time.Time
	mu             sync.Mutex
	logger         logging.Logger
}

// CircuitState represents the state of a circuit breaker.
type CircuitState int

const (
	CircuitClosed CircuitState = iota
	CircuitOpen
	CircuitHalfOpen
)

func NewCircuitBreakerStrategy(inner operations.ComputationStrategy, threshold int, resetTimeout time.Duration, logger logging.Logger) *CircuitBreakerStrategy {
	return &CircuitBreakerStrategy{
		inner:        inner,
		threshold:    threshold,
		resetTimeout: resetTimeout,
		state:        CircuitClosed,
		logger:       logger,
	}
}

func (s *CircuitBreakerStrategy) Name() string {
	return fmt.Sprintf("circuit_breaker(%s)", s.inner.Name())
}

func (s *CircuitBreakerStrategy) Execute(ctx context.Context, op operations.Operation, args ...operations.Number) (*operations.OperationResult, error) {
	s.mu.Lock()
	switch s.state {
	case CircuitOpen:
		if time.Since(s.lastFailure) > s.resetTimeout {
			s.state = CircuitHalfOpen
			s.logger.Info("Circuit breaker half-open", map[string]interface{}{"operation": op.Name()})
		} else {
			s.mu.Unlock()
			return nil, fmt.Errorf("circuit breaker open for operation %s", op.Name())
		}
	}
	s.mu.Unlock()

	result, err := s.inner.Execute(ctx, op, args...)
	if err != nil {
		s.mu.Lock()
		s.failureCount++
		s.lastFailure = time.Now()
		if s.failureCount >= s.threshold {
			s.state = CircuitOpen
			s.logger.Error("Circuit breaker opened", map[string]interface{}{
				"operation": op.Name(),
				"failures":  s.failureCount,
			})
		}
		s.mu.Unlock()
		return nil, err
	}

	s.mu.Lock()
	if s.state == CircuitHalfOpen {
		s.state = CircuitClosed
		s.failureCount = 0
		s.logger.Info("Circuit breaker closed", map[string]interface{}{"operation": op.Name()})
	}
	s.mu.Unlock()

	return result, nil
}
