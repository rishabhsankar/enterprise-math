package statistics

import (
	"fmt"
	"math"
	"sort"
	"sync"
)

// Dataset represents a collection of numerical data points with associated metadata.
// It provides thread-safe access to statistical computations on the underlying data.
type Dataset struct {
	name   string
	values []float64
	mu     sync.RWMutex
	cache  map[string]float64
}

// NewDataset creates a new Dataset with the given name and values.
// Values are copied to prevent external mutation.
func NewDataset(name string, values []float64) *Dataset {
	copied := make([]float64, len(values))
	copy(copied, values)
	return &Dataset{
		name:   name,
		values: copied,
		cache:  make(map[string]float64),
	}
}

// Name returns the dataset name.
func (d *Dataset) Name() string {
	return d.name
}

// Len returns the number of data points.
func (d *Dataset) Len() int {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return len(d.values)
}

// Mean calculates the arithmetic mean of the dataset.
// Returns an error if the dataset is empty.
func (d *Dataset) Mean() (float64, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	if len(d.values) == 0 {
		return 0, fmt.Errorf("cannot compute mean of empty dataset %q", d.name)
	}
	if v, ok := d.cache["mean"]; ok {
		return v, nil
	}
	sum := 0.0
	for _, v := range d.values {
		sum += v
	}
	result := sum / float64(len(d.values))
	d.cache["mean"] = result
	return result, nil
}

// Median calculates the median value of the dataset.
// For even-length datasets, returns the average of the two middle values.
func (d *Dataset) Median() (float64, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	if len(d.values) == 0 {
		return 0, fmt.Errorf("cannot compute median of empty dataset %q", d.name)
	}
	sorted := make([]float64, len(d.values))
	copy(sorted, d.values)
	sort.Float64s(sorted)
	n := len(sorted)
	if n%2 == 0 {
		return (sorted[n/2-1] + sorted[n/2]) / 2.0, nil
	}
	return sorted[n/2], nil
}

// Mode returns the most frequently occurring value(s) in the dataset.
// If all values occur with equal frequency, all values are returned.
func (d *Dataset) Mode() ([]float64, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	if len(d.values) == 0 {
		return nil, fmt.Errorf("cannot compute mode of empty dataset %q", d.name)
	}
	freq := make(map[float64]int)
	maxFreq := 0
	for _, v := range d.values {
		freq[v]++
		if freq[v] > maxFreq {
			maxFreq = freq[v]
		}
	}
	var modes []float64
	for v, f := range freq {
		if f == maxFreq {
			modes = append(modes, v)
		}
	}
	sort.Float64s(modes)
	return modes, nil
}

// Variance calculates the population variance of the dataset.
func (d *Dataset) Variance() (float64, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	if len(d.values) == 0 {
		return 0, fmt.Errorf("cannot compute variance of empty dataset %q", d.name)
	}
	mean, _ := d.meanUnsafe()
	sumSq := 0.0
	for _, v := range d.values {
		diff := v - mean
		sumSq += diff * diff
	}
	return sumSq / float64(len(d.values)), nil
}

// SampleVariance calculates the sample variance (Bessel correction) of the dataset.
func (d *Dataset) SampleVariance() (float64, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	if len(d.values) < 2 {
		return 0, fmt.Errorf("need at least 2 values for sample variance in dataset %q", d.name)
	}
	mean, _ := d.meanUnsafe()
	sumSq := 0.0
	for _, v := range d.values {
		diff := v - mean
		sumSq += diff * diff
	}
	return sumSq / float64(len(d.values)-1), nil
}

// StdDev calculates the population standard deviation.
func (d *Dataset) StdDev() (float64, error) {
	v, err := d.Variance()
	if err != nil {
		return 0, err
	}
	return math.Sqrt(v), nil
}

// SampleStdDev calculates the sample standard deviation.
func (d *Dataset) SampleStdDev() (float64, error) {
	v, err := d.SampleVariance()
	if err != nil {
		return 0, err
	}
	return math.Sqrt(v), nil
}

// Percentile calculates the p-th percentile (0-100) using linear interpolation.
func (d *Dataset) Percentile(p float64) (float64, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	if len(d.values) == 0 {
		return 0, fmt.Errorf("cannot compute percentile of empty dataset %q", d.name)
	}
	if p < 0 || p > 100 {
		return 0, fmt.Errorf("percentile must be between 0 and 100, got %f", p)
	}
	sorted := make([]float64, len(d.values))
	copy(sorted, d.values)
	sort.Float64s(sorted)
	if p == 0 {
		return sorted[0], nil
	}
	if p == 100 {
		return sorted[len(sorted)-1], nil
	}
	rank := (p / 100.0) * float64(len(sorted)-1)
	lower := int(math.Floor(rank))
	upper := int(math.Ceil(rank))
	if lower == upper {
		return sorted[lower], nil
	}
	frac := rank - float64(lower)
	return sorted[lower]*(1-frac) + sorted[upper]*frac, nil
}

// IQR calculates the interquartile range (Q3 - Q1).
func (d *Dataset) IQR() (float64, error) {
	q1, err := d.Percentile(25)
	if err != nil {
		return 0, err
	}
	q3, err := d.Percentile(75)
	if err != nil {
		return 0, err
	}
	return q3 - q1, nil
}

// Skewness calculates the Fisher-Pearson coefficient of skewness.
func (d *Dataset) Skewness() (float64, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	n := len(d.values)
	if n < 3 {
		return 0, fmt.Errorf("need at least 3 values for skewness in dataset %q", d.name)
	}
	mean, _ := d.meanUnsafe()
	stddev, err := d.stddevUnsafe()
	if err != nil {
		return 0, err
	}
	if stddev == 0 {
		return 0, nil
	}
	sum := 0.0
	for _, v := range d.values {
		sum += math.Pow((v-mean)/stddev, 3)
	}
	nf := float64(n)
	return (nf / ((nf - 1) * (nf - 2))) * sum, nil
}

// Kurtosis calculates the excess kurtosis of the dataset.
func (d *Dataset) Kurtosis() (float64, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	n := len(d.values)
	if n < 4 {
		return 0, fmt.Errorf("need at least 4 values for kurtosis in dataset %q", d.name)
	}
	mean, _ := d.meanUnsafe()
	stddev, err := d.stddevUnsafe()
	if err != nil {
		return 0, err
	}
	if stddev == 0 {
		return 0, nil
	}
	sum := 0.0
	for _, v := range d.values {
		sum += math.Pow((v-mean)/stddev, 4)
	}
	nf := float64(n)
	term1 := (nf * (nf + 1)) / ((nf - 1) * (nf - 2) * (nf - 3))
	term2 := (3 * (nf - 1) * (nf - 1)) / ((nf - 2) * (nf - 3))
	return term1*sum - term2, nil
}

// Covariance computes the population covariance between this dataset and another.
func (d *Dataset) Covariance(other *Dataset) (float64, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	other.mu.RLock()
	defer other.mu.RUnlock()
	if len(d.values) != len(other.values) {
		return 0, fmt.Errorf("datasets must have equal length: %d != %d", len(d.values), len(other.values))
	}
	if len(d.values) == 0 {
		return 0, fmt.Errorf("cannot compute covariance of empty datasets")
	}
	meanX, _ := d.meanUnsafe()
	meanY, _ := other.meanUnsafe()
	sum := 0.0
	for i := range d.values {
		sum += (d.values[i] - meanX) * (other.values[i] - meanY)
	}
	return sum / float64(len(d.values)), nil
}

// Correlation computes the Pearson correlation coefficient between two datasets.
func (d *Dataset) Correlation(other *Dataset) (float64, error) {
	cov, err := d.Covariance(other)
	if err != nil {
		return 0, err
	}
	sdX, err := d.StdDev()
	if err != nil {
		return 0, err
	}
	sdY, err := other.StdDev()
	if err != nil {
		return 0, err
	}
	if sdX == 0 || sdY == 0 {
		return 0, fmt.Errorf("cannot compute correlation when standard deviation is zero")
	}
	return cov / (sdX * sdY), nil
}

// ZScores returns the z-score normalized values of the dataset.
func (d *Dataset) ZScores() ([]float64, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	if len(d.values) == 0 {
		return nil, fmt.Errorf("cannot compute z-scores of empty dataset %q", d.name)
	}
	mean, _ := d.meanUnsafe()
	stddev, err := d.stddevUnsafe()
	if err != nil {
		return nil, err
	}
	if stddev == 0 {
		return nil, fmt.Errorf("cannot compute z-scores when standard deviation is zero")
	}
	scores := make([]float64, len(d.values))
	for i, v := range d.values {
		scores[i] = (v - mean) / stddev
	}
	return scores, nil
}

// Outliers returns values that are more than the given number of standard deviations
// from the mean. A typical threshold is 2.0 or 3.0.
func (d *Dataset) Outliers(threshold float64) ([]float64, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	if len(d.values) == 0 {
		return nil, fmt.Errorf("cannot detect outliers in empty dataset %q", d.name)
	}
	mean, _ := d.meanUnsafe()
	stddev, err := d.stddevUnsafe()
	if err != nil {
		return nil, err
	}
	var outliers []float64
	for _, v := range d.values {
		if math.Abs(v-mean) > threshold*stddev {
			outliers = append(outliers, v)
		}
	}
	return outliers, nil
}

// Histogram bins the data into the specified number of equal-width bins.
// Returns bin edges and counts.
func (d *Dataset) Histogram(bins int) ([]float64, []int, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	if len(d.values) == 0 {
		return nil, nil, fmt.Errorf("cannot create histogram of empty dataset %q", d.name)
	}
	if bins <= 0 {
		return nil, nil, fmt.Errorf("number of bins must be positive, got %d", bins)
	}
	minVal, maxVal := d.values[0], d.values[0]
	for _, v := range d.values[1:] {
		if v < minVal {
			minVal = v
		}
		if v > maxVal {
			maxVal = v
		}
	}
	if minVal == maxVal {
		edges := []float64{minVal, maxVal + 1}
		counts := []int{len(d.values)}
		return edges, counts, nil
	}
	width := (maxVal - minVal) / float64(bins)
	edges := make([]float64, bins+1)
	for i := 0; i <= bins; i++ {
		edges[i] = minVal + float64(i)*width
	}
	counts := make([]int, bins)
	for _, v := range d.values {
		idx := int((v - minVal) / width)
		if idx >= bins {
			idx = bins - 1
		}
		counts[idx]++
	}
	return edges, counts, nil
}

// Summary returns a formatted string with key statistical measures.
func (d *Dataset) Summary() string {
	n := d.Len()
	if n == 0 {
		return fmt.Sprintf("Dataset %q: empty", d.name)
	}
	mean, _ := d.Mean()
	median, _ := d.Median()
	stddev, _ := d.StdDev()
	min, max := d.MinMax()
	q1, _ := d.Percentile(25)
	q3, _ := d.Percentile(75)
	return fmt.Sprintf(
		"Dataset %q (n=%d):\n  Mean=%.4f  Median=%.4f  StdDev=%.4f\n  Min=%.4f  Q1=%.4f  Q3=%.4f  Max=%.4f",
		d.name, n, mean, median, stddev, min, q1, q3, max,
	)
}

// MinMax returns the minimum and maximum values in the dataset.
func (d *Dataset) MinMax() (float64, float64) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	if len(d.values) == 0 {
		return 0, 0
	}
	min, max := d.values[0], d.values[0]
	for _, v := range d.values[1:] {
		if v < min {
			min = v
		}
		if v > max {
			max = v
		}
	}
	return min, max
}

// Range returns the difference between the maximum and minimum values.
func (d *Dataset) Range() float64 {
	min, max := d.MinMax()
	return max - min
}

// Add appends new values to the dataset and invalidates the cache.
func (d *Dataset) Add(values ...float64) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.values = append(d.values, values...)
	d.cache = make(map[string]float64)
}

// Values returns a copy of the underlying data.
func (d *Dataset) Values() []float64 {
	d.mu.RLock()
	defer d.mu.RUnlock()
	copied := make([]float64, len(d.values))
	copy(copied, d.values)
	return copied
}

// internal helpers (caller must hold at least RLock)

func (d *Dataset) meanUnsafe() (float64, error) {
	if len(d.values) == 0 {
		return 0, fmt.Errorf("empty")
	}
	sum := 0.0
	for _, v := range d.values {
		sum += v
	}
	return sum / float64(len(d.values)), nil
}

func (d *Dataset) stddevUnsafe() (float64, error) {
	if len(d.values) == 0 {
		return 0, fmt.Errorf("empty")
	}
	mean, _ := d.meanUnsafe()
	sumSq := 0.0
	for _, v := range d.values {
		diff := v - mean
		sumSq += diff * diff
	}
	return math.Sqrt(sumSq / float64(len(d.values))), nil
}
