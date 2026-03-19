package logging

import (
	"fmt"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// SamplingMiddleware reduces log volume by only passing through every Nth entry.
type SamplingMiddleware struct {
	rate    int
	counter atomic.Int64
}

// NewSamplingMiddleware creates a middleware that samples 1 in every `rate` log entries.
func NewSamplingMiddleware(rate int) LogMiddleware {
	sm := &SamplingMiddleware{rate: rate}
	return func(entry *LogEntry) *LogEntry {
		count := sm.counter.Add(1)
		if count%int64(sm.rate) == 0 {
			return entry
		}
		return nil
	}
}

// EnrichmentMiddleware adds system information to every log entry.
type EnrichmentMiddleware struct {
	hostname  string
	pid       int
	goVersion string
}

// NewEnrichmentMiddleware creates a middleware that enriches entries with system metadata.
func NewEnrichmentMiddleware() LogMiddleware {
	em := &EnrichmentMiddleware{
		goVersion: runtime.Version(),
		pid:       0,
	}
	return func(entry *LogEntry) *LogEntry {
		if entry.Fields == nil {
			entry.Fields = make(map[string]interface{})
		}
		entry.Fields["go_version"] = em.goVersion
		entry.Fields["goroutines"] = runtime.NumGoroutine()
		return entry
	}
}

// RedactionMiddleware removes sensitive fields from log entries.
type RedactionMiddleware struct {
	sensitiveKeys map[string]bool
	replacement   string
}

// NewRedactionMiddleware creates a middleware that redacts specified fields.
func NewRedactionMiddleware(keys []string, replacement string) LogMiddleware {
	keyMap := make(map[string]bool)
	for _, k := range keys {
		keyMap[strings.ToLower(k)] = true
	}
	rm := &RedactionMiddleware{sensitiveKeys: keyMap, replacement: replacement}
	return func(entry *LogEntry) *LogEntry {
		if entry.Fields == nil {
			return entry
		}
		for k := range entry.Fields {
			if rm.sensitiveKeys[strings.ToLower(k)] {
				entry.Fields[k] = rm.replacement
			}
		}
		return entry
	}
}

// ThrottleMiddleware limits the rate of log entries per time window.
type ThrottleMiddleware struct {
	maxPerWindow int
	window       time.Duration
	count        int
	windowStart  time.Time
	mu           sync.Mutex
}

// NewThrottleMiddleware creates a rate-limiting log middleware.
func NewThrottleMiddleware(maxPerWindow int, window time.Duration) LogMiddleware {
	tm := &ThrottleMiddleware{
		maxPerWindow: maxPerWindow,
		window:       window,
		windowStart:  time.Now(),
	}
	return func(entry *LogEntry) *LogEntry {
		tm.mu.Lock()
		defer tm.mu.Unlock()

		now := time.Now()
		if now.Sub(tm.windowStart) > tm.window {
			tm.windowStart = now
			tm.count = 0
		}

		tm.count++
		if tm.count > tm.maxPerWindow {
			return nil
		}
		return entry
	}
}

// CorrelationMiddleware adds correlation IDs for distributed tracing.
type CorrelationMiddleware struct {
	idGenerator func() string
	headerKey   string
}

// NewCorrelationMiddleware creates a middleware that adds correlation IDs.
func NewCorrelationMiddleware(headerKey string) LogMiddleware {
	counter := atomic.Int64{}
	cm := &CorrelationMiddleware{
		headerKey: headerKey,
		idGenerator: func() string {
			return fmt.Sprintf("corr-%d-%d", time.Now().UnixNano(), counter.Add(1))
		},
	}
	return func(entry *LogEntry) *LogEntry {
		if entry.Fields == nil {
			entry.Fields = make(map[string]interface{})
		}
		if _, exists := entry.Fields[cm.headerKey]; !exists {
			entry.Fields[cm.headerKey] = cm.idGenerator()
		}
		return entry
	}
}

// AggregationMiddleware batches similar log messages together.
type AggregationMiddleware struct {
	counts   map[string]int
	mu       sync.Mutex
	interval time.Duration
	logger   Logger
}

// NewAggregationMiddleware creates a middleware that aggregates repeated messages.
func NewAggregationMiddleware(interval time.Duration) LogMiddleware {
	am := &AggregationMiddleware{
		counts:   make(map[string]int),
		interval: interval,
	}

	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for range ticker.C {
			am.mu.Lock()
			for msg, count := range am.counts {
				if count > 1 {
					fmt.Printf("[AGGREGATED] Message '%s' occurred %d times\n", msg, count)
				}
			}
			am.counts = make(map[string]int)
			am.mu.Unlock()
		}
	}()

	return func(entry *LogEntry) *LogEntry {
		am.mu.Lock()
		am.counts[entry.Message]++
		count := am.counts[entry.Message]
		am.mu.Unlock()

		if count == 1 {
			return entry
		}
		return nil
	}
}
