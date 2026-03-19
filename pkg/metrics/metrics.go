// Package metrics provides enterprise-grade metrics collection including
// counters, gauges, histograms, and summary statistics.
package metrics

import (
	"fmt"
	"math"
	"sort"
	"sync"
	"time"
)

// MetricType classifies the kind of metric being tracked.
type MetricType int

const (
	Counter MetricType = iota
	Gauge
	Histogram
	Summary
)

// MetricValue represents a single metric data point.
type MetricValue struct {
	Name      string
	Type      MetricType
	Value     float64
	Labels    map[string]string
	Timestamp time.Time
}

// HistogramBucket represents a single histogram bucket.
type HistogramBucket struct {
	UpperBound float64
	Count      int64
}

// HistogramData holds the complete histogram state.
type HistogramData struct {
	Buckets    []HistogramBucket
	Sum        float64
	Count      int64
	Min        float64
	Max        float64
	Quantiles  map[float64]float64
}

// MetricsCollector manages all metrics for the application.
type MetricsCollector struct {
	counters   map[string]*counterMetric
	gauges     map[string]*gaugeMetric
	histograms map[string]*histogramMetric
	mu         sync.RWMutex
	prefix     string
	buckets    []float64
}

type counterMetric struct {
	values map[string]float64
	mu     sync.Mutex
}

type gaugeMetric struct {
	values map[string]float64
	mu     sync.Mutex
}

type histogramMetric struct {
	observations map[string][]float64
	buckets      []float64
	mu           sync.Mutex
}

// NewMetricsCollector creates a new metrics collector with the given prefix and bucket boundaries.
func NewMetricsCollector(prefix string, buckets []float64) *MetricsCollector {
	if buckets == nil {
		buckets = []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1.0, 2.5, 5.0, 10.0}
	}
	sort.Float64s(buckets)
	return &MetricsCollector{
		counters:   make(map[string]*counterMetric),
		gauges:     make(map[string]*gaugeMetric),
		histograms: make(map[string]*histogramMetric),
		prefix:     prefix,
		buckets:    buckets,
	}
}

func labelsToKey(labels map[string]string) string {
	if len(labels) == 0 {
		return "__default__"
	}
	keys := make([]string, 0, len(labels))
	for k := range labels {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	key := ""
	for _, k := range keys {
		key += k + "=" + labels[k] + ","
	}
	return key
}

// IncrementCounter increments a counter metric by 1.
func (mc *MetricsCollector) IncrementCounter(name string, labels map[string]string) {
	mc.AddCounter(name, 1, labels)
}

// AddCounter adds a value to a counter metric.
func (mc *MetricsCollector) AddCounter(name string, value float64, labels map[string]string) {
	fullName := mc.prefix + "_" + name
	mc.mu.Lock()
	c, exists := mc.counters[fullName]
	if !exists {
		c = &counterMetric{values: make(map[string]float64)}
		mc.counters[fullName] = c
	}
	mc.mu.Unlock()

	key := labelsToKey(labels)
	c.mu.Lock()
	c.values[key] += value
	c.mu.Unlock()
}

// SetGauge sets a gauge metric to a specific value.
func (mc *MetricsCollector) SetGauge(name string, value float64, labels map[string]string) {
	fullName := mc.prefix + "_" + name
	mc.mu.Lock()
	g, exists := mc.gauges[fullName]
	if !exists {
		g = &gaugeMetric{values: make(map[string]float64)}
		mc.gauges[fullName] = g
	}
	mc.mu.Unlock()

	key := labelsToKey(labels)
	g.mu.Lock()
	g.values[key] = value
	g.mu.Unlock()
}

// ObserveHistogram records an observation in a histogram metric.
func (mc *MetricsCollector) ObserveHistogram(name string, value float64, labels map[string]string) {
	fullName := mc.prefix + "_" + name
	mc.mu.Lock()
	h, exists := mc.histograms[fullName]
	if !exists {
		h = &histogramMetric{
			observations: make(map[string][]float64),
			buckets:      mc.buckets,
		}
		mc.histograms[fullName] = h
	}
	mc.mu.Unlock()

	key := labelsToKey(labels)
	h.mu.Lock()
	h.observations[key] = append(h.observations[key], value)
	h.mu.Unlock()
}

// GetCounterValue retrieves the current value of a counter.
func (mc *MetricsCollector) GetCounterValue(name string, labels map[string]string) float64 {
	fullName := mc.prefix + "_" + name
	mc.mu.RLock()
	c, exists := mc.counters[fullName]
	mc.mu.RUnlock()
	if !exists {
		return 0
	}

	key := labelsToKey(labels)
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.values[key]
}

// GetGaugeValue retrieves the current value of a gauge.
func (mc *MetricsCollector) GetGaugeValue(name string, labels map[string]string) float64 {
	fullName := mc.prefix + "_" + name
	mc.mu.RLock()
	g, exists := mc.gauges[fullName]
	mc.mu.RUnlock()
	if !exists {
		return 0
	}

	key := labelsToKey(labels)
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.values[key]
}

// GetHistogramData retrieves the current histogram data.
func (mc *MetricsCollector) GetHistogramData(name string, labels map[string]string) *HistogramData {
	fullName := mc.prefix + "_" + name
	mc.mu.RLock()
	h, exists := mc.histograms[fullName]
	mc.mu.RUnlock()
	if !exists {
		return nil
	}

	key := labelsToKey(labels)
	h.mu.Lock()
	defer h.mu.Unlock()

	obs := h.observations[key]
	if len(obs) == 0 {
		return nil
	}

	sorted := make([]float64, len(obs))
	copy(sorted, obs)
	sort.Float64s(sorted)

	data := &HistogramData{
		Count:     int64(len(sorted)),
		Min:       sorted[0],
		Max:       sorted[len(sorted)-1],
		Quantiles: make(map[float64]float64),
	}

	for _, v := range sorted {
		data.Sum += v
	}

	// Build bucket counts
	data.Buckets = make([]HistogramBucket, len(h.buckets))
	for i, bound := range h.buckets {
		count := int64(0)
		for _, v := range sorted {
			if v <= bound {
				count++
			}
		}
		data.Buckets[i] = HistogramBucket{UpperBound: bound, Count: count}
	}

	// Calculate quantiles
	for _, q := range []float64{0.5, 0.9, 0.95, 0.99} {
		idx := int(math.Ceil(q*float64(len(sorted)))) - 1
		if idx < 0 {
			idx = 0
		}
		data.Quantiles[q] = sorted[idx]
	}

	return data
}

// Snapshot returns a formatted string of all metrics.
func (mc *MetricsCollector) Snapshot() string {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	result := "=== Metrics Snapshot ===\n"

	result += "\n--- Counters ---\n"
	for name, c := range mc.counters {
		c.mu.Lock()
		for key, val := range c.values {
			result += fmt.Sprintf("  %s{%s} = %.2f\n", name, key, val)
		}
		c.mu.Unlock()
	}

	result += "\n--- Gauges ---\n"
	for name, g := range mc.gauges {
		g.mu.Lock()
		for key, val := range g.values {
			result += fmt.Sprintf("  %s{%s} = %.6f\n", name, key, val)
		}
		g.mu.Unlock()
	}

	result += "\n--- Histograms ---\n"
	for name, h := range mc.histograms {
		h.mu.Lock()
		for key, obs := range h.observations {
			result += fmt.Sprintf("  %s{%s}: %d observations\n", name, key, len(obs))
		}
		h.mu.Unlock()
	}

	return result
}

// Reset clears all metrics.
func (mc *MetricsCollector) Reset() {
	mc.mu.Lock()
	defer mc.mu.Unlock()
	mc.counters = make(map[string]*counterMetric)
	mc.gauges = make(map[string]*gaugeMetric)
	mc.histograms = make(map[string]*histogramMetric)
}
