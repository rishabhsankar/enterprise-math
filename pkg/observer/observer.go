// Package observer implements the Observer pattern for mathematical computation
// events, enabling decoupled monitoring, alerting, and side-effect processing.
package observer

import (
	"fmt"
	"sync"
	"time"

	"github.com/rishabhsankar/enterprise-math/pkg/logging"
	"github.com/rishabhsankar/enterprise-math/pkg/operations"
)

// EventType classifies computation events.
type EventType int

const (
	EventOperationStarted EventType = iota
	EventOperationCompleted
	EventOperationFailed
	EventCacheHit
	EventCacheMiss
	EventCacheEviction
	EventPipelineStarted
	EventPipelineCompleted
	EventPipelineFailed
	EventPluginLoaded
	EventPluginError
	EventThresholdExceeded
	EventAnomalyDetected
)

// String returns the human-readable event type name.
func (e EventType) String() string {
	names := []string{
		"OperationStarted", "OperationCompleted", "OperationFailed",
		"CacheHit", "CacheMiss", "CacheEviction",
		"PipelineStarted", "PipelineCompleted", "PipelineFailed",
		"PluginLoaded", "PluginError",
		"ThresholdExceeded", "AnomalyDetected",
	}
	if int(e) < len(names) {
		return names[e]
	}
	return "Unknown"
}

// Event represents a computation event with full context.
type Event struct {
	Type      EventType
	Timestamp time.Time
	Source    string
	Data      map[string]interface{}
	Result    *operations.OperationResult
	Error     error
}

// Observer defines the interface for event handlers.
type Observer interface {
	ID() string
	OnEvent(event Event)
	EventTypes() []EventType
}

// EventBus manages event distribution to observers.
type EventBus struct {
	observers map[EventType][]Observer
	allObservers []Observer
	mu        sync.RWMutex
	logger    logging.Logger
	asyncMode bool
	eventCh   chan Event
}

// NewEventBus creates a new event distribution bus.
func NewEventBus(logger logging.Logger, asyncMode bool) *EventBus {
	bus := &EventBus{
		observers:    make(map[EventType][]Observer),
		allObservers: make([]Observer, 0),
		logger:       logger.WithPrefix("events"),
		asyncMode:    asyncMode,
	}

	if asyncMode {
		bus.eventCh = make(chan Event, 1000)
		go bus.processAsync()
	}

	return bus
}

// Subscribe registers an observer for its declared event types.
func (bus *EventBus) Subscribe(observer Observer) {
	bus.mu.Lock()
	defer bus.mu.Unlock()

	bus.allObservers = append(bus.allObservers, observer)

	for _, eventType := range observer.EventTypes() {
		bus.observers[eventType] = append(bus.observers[eventType], observer)
	}

	bus.logger.Debug("Observer subscribed", map[string]interface{}{
		"id":     observer.ID(),
		"events": len(observer.EventTypes()),
	})
}

// Unsubscribe removes an observer from all events.
func (bus *EventBus) Unsubscribe(id string) {
	bus.mu.Lock()
	defer bus.mu.Unlock()

	for eventType, observers := range bus.observers {
		filtered := make([]Observer, 0)
		for _, o := range observers {
			if o.ID() != id {
				filtered = append(filtered, o)
			}
		}
		bus.observers[eventType] = filtered
	}

	filtered := make([]Observer, 0)
	for _, o := range bus.allObservers {
		if o.ID() != id {
			filtered = append(filtered, o)
		}
	}
	bus.allObservers = filtered
}

// Publish sends an event to all subscribed observers.
func (bus *EventBus) Publish(event Event) {
	if bus.asyncMode {
		select {
		case bus.eventCh <- event:
		default:
			bus.logger.Warn("Event bus overflow, dropping event", map[string]interface{}{
				"type": event.Type.String(),
			})
		}
		return
	}

	bus.dispatch(event)
}

func (bus *EventBus) dispatch(event Event) {
	bus.mu.RLock()
	observers := bus.observers[event.Type]
	bus.mu.RUnlock()

	for _, observer := range observers {
		func() {
			defer func() {
				if r := recover(); r != nil {
					bus.logger.Error("Observer panicked", map[string]interface{}{
						"observer": observer.ID(),
						"panic":    fmt.Sprintf("%v", r),
					})
				}
			}()
			observer.OnEvent(event)
		}()
	}
}

func (bus *EventBus) processAsync() {
	for event := range bus.eventCh {
		bus.dispatch(event)
	}
}

// Close shuts down the event bus.
func (bus *EventBus) Close() {
	if bus.asyncMode && bus.eventCh != nil {
		close(bus.eventCh)
	}
}

// PerformanceObserver tracks operation performance and alerts on thresholds.
type PerformanceObserver struct {
	id              string
	thresholdMs     float64
	slowOperations  []Event
	mu              sync.Mutex
	logger          logging.Logger
}

// NewPerformanceObserver creates an observer that alerts on slow operations.
func NewPerformanceObserver(thresholdMs float64, logger logging.Logger) *PerformanceObserver {
	return &PerformanceObserver{
		id:             "performance-monitor",
		thresholdMs:    thresholdMs,
		slowOperations: make([]Event, 0),
		logger:         logger.WithPrefix("perf"),
	}
}

func (o *PerformanceObserver) ID() string { return o.id }
func (o *PerformanceObserver) EventTypes() []EventType {
	return []EventType{EventOperationCompleted}
}

func (o *PerformanceObserver) OnEvent(event Event) {
	if event.Result == nil {
		return
	}
	durationMs := event.Result.Duration.Seconds() * 1000
	if durationMs > o.thresholdMs {
		o.mu.Lock()
		o.slowOperations = append(o.slowOperations, event)
		o.mu.Unlock()

		o.logger.Warn("Slow operation detected", map[string]interface{}{
			"source":      event.Source,
			"duration_ms": durationMs,
			"threshold":   o.thresholdMs,
		})
	}
}

// SlowOperations returns all detected slow operations.
func (o *PerformanceObserver) SlowOperations() []Event {
	o.mu.Lock()
	defer o.mu.Unlock()
	result := make([]Event, len(o.slowOperations))
	copy(result, o.slowOperations)
	return result
}

// ErrorCountObserver tracks operation error rates.
type ErrorCountObserver struct {
	id     string
	counts map[string]int
	mu     sync.Mutex
	logger logging.Logger
}

// NewErrorCountObserver creates an observer that counts errors by operation.
func NewErrorCountObserver(logger logging.Logger) *ErrorCountObserver {
	return &ErrorCountObserver{
		id:     "error-counter",
		counts: make(map[string]int),
		logger: logger.WithPrefix("errors"),
	}
}

func (o *ErrorCountObserver) ID() string { return o.id }
func (o *ErrorCountObserver) EventTypes() []EventType {
	return []EventType{EventOperationFailed}
}

func (o *ErrorCountObserver) OnEvent(event Event) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.counts[event.Source]++

	o.logger.Debug("Error counted", map[string]interface{}{
		"source": event.Source,
		"total":  o.counts[event.Source],
	})
}

// GetCounts returns error counts by operation.
func (o *ErrorCountObserver) GetCounts() map[string]int {
	o.mu.Lock()
	defer o.mu.Unlock()
	result := make(map[string]int)
	for k, v := range o.counts {
		result[k] = v
	}
	return result
}

// AuditObserver logs all events for compliance audit trails.
type AuditObserver struct {
	id      string
	entries []Event
	maxSize int
	mu      sync.Mutex
	logger  logging.Logger
}

// NewAuditObserver creates an observer that records all events for audit.
func NewAuditObserver(maxSize int, logger logging.Logger) *AuditObserver {
	return &AuditObserver{
		id:      "audit-trail",
		entries: make([]Event, 0, maxSize),
		maxSize: maxSize,
		logger:  logger.WithPrefix("audit"),
	}
}

func (o *AuditObserver) ID() string { return o.id }
func (o *AuditObserver) EventTypes() []EventType {
	return []EventType{
		EventOperationStarted, EventOperationCompleted, EventOperationFailed,
		EventPipelineStarted, EventPipelineCompleted, EventPipelineFailed,
	}
}

func (o *AuditObserver) OnEvent(event Event) {
	o.mu.Lock()
	defer o.mu.Unlock()

	if len(o.entries) >= o.maxSize {
		o.entries = o.entries[1:]
	}
	o.entries = append(o.entries, event)
}

// Entries returns all audit entries.
func (o *AuditObserver) Entries() []Event {
	o.mu.Lock()
	defer o.mu.Unlock()
	result := make([]Event, len(o.entries))
	copy(result, o.entries)
	return result
}
