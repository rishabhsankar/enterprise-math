// Package middleware provides cross-cutting concerns for mathematical operations
// including logging, metrics, caching, validation, and audit trail middleware.
package middleware

import (
	"context"
	"fmt"
	"time"

	"github.com/rishabhsankar/enterprise-math/pkg/cache"
	"github.com/rishabhsankar/enterprise-math/pkg/logging"
	"github.com/rishabhsankar/enterprise-math/pkg/metrics"
	"github.com/rishabhsankar/enterprise-math/pkg/operations"
)

// LoggingMiddleware wraps operations with structured logging.
type LoggingMiddleware struct {
	inner  operations.Operation
	logger logging.Logger
}

// NewLoggingMiddleware creates a logging wrapper for an operation.
func NewLoggingMiddleware(inner operations.Operation, logger logging.Logger) *LoggingMiddleware {
	return &LoggingMiddleware{inner: inner, logger: logger.WithPrefix("op")}
}

func (m *LoggingMiddleware) Name() string                  { return m.inner.Name() }
func (m *LoggingMiddleware) Description() string           { return m.inner.Description() }
func (m *LoggingMiddleware) Category() operations.OperationCategory { return m.inner.Category() }
func (m *LoggingMiddleware) Arity() int                    { return m.inner.Arity() }

func (m *LoggingMiddleware) Validate(args ...operations.Number) error {
	return m.inner.Validate(args...)
}

func (m *LoggingMiddleware) Execute(ctx context.Context, args ...operations.Number) (*operations.OperationResult, error) {
	m.logger.Debug("Executing operation", map[string]interface{}{
		"operation": m.inner.Name(),
		"args":      len(args),
	})

	start := time.Now()
	result, err := m.inner.Execute(ctx, args...)
	duration := time.Since(start)

	if err != nil {
		m.logger.Error("Operation failed", map[string]interface{}{
			"operation": m.inner.Name(),
			"duration":  duration.String(),
			"error":     err.Error(),
		})
		return nil, err
	}

	m.logger.Debug("Operation completed", map[string]interface{}{
		"operation": m.inner.Name(),
		"duration":  duration.String(),
		"result":    result.Value.Value,
	})

	return result, nil
}

// MetricsMiddleware collects performance metrics for operations.
type MetricsMiddleware struct {
	inner     operations.Operation
	collector *metrics.MetricsCollector
}

// NewMetricsMiddleware creates a metrics-collecting wrapper.
func NewMetricsMiddleware(inner operations.Operation, collector *metrics.MetricsCollector) *MetricsMiddleware {
	return &MetricsMiddleware{inner: inner, collector: collector}
}

func (m *MetricsMiddleware) Name() string                  { return m.inner.Name() }
func (m *MetricsMiddleware) Description() string           { return m.inner.Description() }
func (m *MetricsMiddleware) Category() operations.OperationCategory { return m.inner.Category() }
func (m *MetricsMiddleware) Arity() int                    { return m.inner.Arity() }
func (m *MetricsMiddleware) Validate(args ...operations.Number) error { return m.inner.Validate(args...) }

func (m *MetricsMiddleware) Execute(ctx context.Context, args ...operations.Number) (*operations.OperationResult, error) {
	start := time.Now()
	result, err := m.inner.Execute(ctx, args...)
	duration := time.Since(start)

	labels := map[string]string{
		"operation": m.inner.Name(),
		"category":  m.inner.Category().String(),
	}

	if err != nil {
		labels["status"] = "error"
		m.collector.IncrementCounter("operation_errors_total", labels)
	} else {
		labels["status"] = "success"
		m.collector.IncrementCounter("operation_success_total", labels)
	}

	m.collector.ObserveHistogram("operation_duration_seconds", duration.Seconds(), labels)
	m.collector.SetGauge("operation_last_duration_seconds", duration.Seconds(), labels)

	return result, err
}

// CachingMiddleware adds result caching to operations.
type CachingMiddleware struct {
	inner  operations.Operation
	cache  cache.Cache
	logger logging.Logger
}

// NewCachingMiddleware creates a cache-aside wrapper for an operation.
func NewCachingMiddleware(inner operations.Operation, c cache.Cache, logger logging.Logger) *CachingMiddleware {
	return &CachingMiddleware{inner: inner, cache: c, logger: logger}
}

func (m *CachingMiddleware) Name() string                  { return m.inner.Name() }
func (m *CachingMiddleware) Description() string           { return m.inner.Description() }
func (m *CachingMiddleware) Category() operations.OperationCategory { return m.inner.Category() }
func (m *CachingMiddleware) Arity() int                    { return m.inner.Arity() }
func (m *CachingMiddleware) Validate(args ...operations.Number) error { return m.inner.Validate(args...) }

func (m *CachingMiddleware) Execute(ctx context.Context, args ...operations.Number) (*operations.OperationResult, error) {
	key := cache.GenerateCacheKey(m.inner.Name(), args)

	if cached, ok := m.cache.Get(key); ok {
		m.logger.Debug("Cache hit", map[string]interface{}{
			"operation": m.inner.Name(),
			"key":       key,
		})
		cached.CacheHit = true
		return cached, nil
	}

	result, err := m.inner.Execute(ctx, args...)
	if err != nil {
		return nil, err
	}

	m.cache.Set(key, result)
	return result, nil
}

// ValidationMiddleware adds strict input validation to operations.
type ValidationMiddleware struct {
	inner      operations.Operation
	validators []InputValidator
	logger     logging.Logger
}

// InputValidator defines a custom validation function for operation inputs.
type InputValidator struct {
	Name     string
	Validate func(args []operations.Number) error
}

// NewValidationMiddleware creates a validation wrapper.
func NewValidationMiddleware(inner operations.Operation, validators []InputValidator, logger logging.Logger) *ValidationMiddleware {
	return &ValidationMiddleware{inner: inner, validators: validators, logger: logger}
}

func (m *ValidationMiddleware) Name() string                  { return m.inner.Name() }
func (m *ValidationMiddleware) Description() string           { return m.inner.Description() }
func (m *ValidationMiddleware) Category() operations.OperationCategory { return m.inner.Category() }
func (m *ValidationMiddleware) Arity() int                    { return m.inner.Arity() }

func (m *ValidationMiddleware) Validate(args ...operations.Number) error {
	if err := m.inner.Validate(args...); err != nil {
		return err
	}
	for _, v := range m.validators {
		if err := v.Validate(args); err != nil {
			return fmt.Errorf("validation %q failed: %w", v.Name, err)
		}
	}
	return nil
}

func (m *ValidationMiddleware) Execute(ctx context.Context, args ...operations.Number) (*operations.OperationResult, error) {
	if err := m.Validate(args...); err != nil {
		return nil, err
	}
	return m.inner.Execute(ctx, args...)
}

// AuditMiddleware records all operation executions for compliance.
type AuditMiddleware struct {
	inner   operations.Operation
	auditor *AuditLog
}

// AuditLog stores audit entries for operation executions.
type AuditLog struct {
	entries []AuditRecord
	maxSize int
	logger  logging.Logger
}

// AuditRecord captures the full context of an operation execution.
type AuditRecord struct {
	Timestamp   time.Time
	Operation   string
	Category    string
	InputCount  int
	InputValues []float64
	OutputValue float64
	Duration    time.Duration
	Success     bool
	Error       string
}

// NewAuditLog creates a new audit log with the given capacity.
func NewAuditLog(maxSize int, logger logging.Logger) *AuditLog {
	return &AuditLog{
		entries: make([]AuditRecord, 0, maxSize),
		maxSize: maxSize,
		logger:  logger,
	}
}

// Record adds an audit entry.
func (al *AuditLog) Record(record AuditRecord) {
	if len(al.entries) >= al.maxSize {
		al.entries = al.entries[1:]
	}
	al.entries = append(al.entries, record)
}

// Entries returns all audit records.
func (al *AuditLog) Entries() []AuditRecord {
	result := make([]AuditRecord, len(al.entries))
	copy(result, al.entries)
	return result
}

// NewAuditMiddleware creates an audit wrapper for an operation.
func NewAuditMiddleware(inner operations.Operation, auditor *AuditLog) *AuditMiddleware {
	return &AuditMiddleware{inner: inner, auditor: auditor}
}

func (m *AuditMiddleware) Name() string                  { return m.inner.Name() }
func (m *AuditMiddleware) Description() string           { return m.inner.Description() }
func (m *AuditMiddleware) Category() operations.OperationCategory { return m.inner.Category() }
func (m *AuditMiddleware) Arity() int                    { return m.inner.Arity() }
func (m *AuditMiddleware) Validate(args ...operations.Number) error { return m.inner.Validate(args...) }

func (m *AuditMiddleware) Execute(ctx context.Context, args ...operations.Number) (*operations.OperationResult, error) {
	start := time.Now()
	result, err := m.inner.Execute(ctx, args...)
	duration := time.Since(start)

	record := AuditRecord{
		Timestamp:  start,
		Operation:  m.inner.Name(),
		Category:   m.inner.Category().String(),
		InputCount: len(args),
		Duration:   duration,
		Success:    err == nil,
	}

	record.InputValues = make([]float64, len(args))
	for i, arg := range args {
		record.InputValues[i] = arg.Value
	}

	if err != nil {
		record.Error = err.Error()
	} else {
		record.OutputValue = result.Value.Value
	}

	m.auditor.Record(record)

	return result, err
}
