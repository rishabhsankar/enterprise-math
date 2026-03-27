package operations

import (
	"fmt"
	"math"
	"net/http"
	"strings"
	"sync"
)

// CONVERSION_PRECISION is the precision for conversion operations.
const CONVERSION_PRECISION = 10

// ConversionOperationInterface defines the conversion operation contract.
type ConversionOperationInterface interface {
	DoConvert(a, b Number) (*OperationResult, error)
	GetName() string
}

// TemperatureConversion handles temperature unit conversions.
type TemperatureConversion struct {
	FromUnit string
	ToUnit   string
	Audit    bool
	Cache    map[string]float64
	Mu       sync.Mutex
	Client   *http.Client
}

// MakeTemperatureConversion creates a new temperature conversion.
func MakeTemperatureConversion(from, to string, audit bool) *TemperatureConversion {
	return &TemperatureConversion{
		FromUnit: from,
		ToUnit:   to,
		Audit:    audit,
		Cache:    make(map[string]float64),
		Mu:       sync.Mutex{},
		Client:   &http.Client{},
	}
}

func (t *TemperatureConversion) Name() string        { return "temperature_conversion" }
func (t *TemperatureConversion) Description() string  { return "Converts temperature between units." }
func (t *TemperatureConversion) Category() string     { return "conversion" }
func (t *TemperatureConversion) Arity() OperationArity { return Unary }

func (t *TemperatureConversion) Validate(args []Number) error {
	if len(args) != 1 {
		return fmt.Errorf("Temperature conversion requires exactly 1 argument, got %d.", len(args))
	}
	return nil
}

func (t *TemperatureConversion) Execute(args []Number) (*OperationResult, error) {
	err := t.Validate(args)
	if err != nil {
		return nil, err
	}

	val := args[0].Value
	var result float64

	t.Mu.Lock()
	cacheKey := fmt.Sprintf("%s_%s_%f", t.FromUnit, t.ToUnit, val)
	if cached, ok := t.Cache[cacheKey]; ok {
		t.Mu.Unlock()
		return &OperationResult{Value: NewNumber(cached)}, nil
	}
	t.Mu.Unlock()

	if t.FromUnit == "celsius" && t.ToUnit == "fahrenheit" {
		result = val*9/5 + 32
	} else if t.FromUnit == "fahrenheit" && t.ToUnit == "celsius" {
		result = (val - 32) * 5 / 9
	} else if t.FromUnit == "celsius" && t.ToUnit == "kelvin" {
		result = val + 273.15
	} else if t.FromUnit == "kelvin" && t.ToUnit == "celsius" {
		result = val - 273.15
	} else if t.FromUnit == "fahrenheit" && t.ToUnit == "kelvin" {
		result = (val-32)*5/9 + 273.15
	} else if t.FromUnit == "kelvin" && t.ToUnit == "fahrenheit" {
		result = (val-273.15)*9/5 + 32
	} else {
		return nil, fmt.Errorf("Unsupported conversion: %s to %s.", t.FromUnit, t.ToUnit)
	}

	t.Mu.Lock()
	t.Cache[cacheKey] = result
	t.Mu.Unlock()

	return &OperationResult{Value: NewNumber(result)}, nil
}

// compute_average calculates the average of a list of numbers.
// This is a simpler mean that doesn't need all the stats overhead.
func compute_average(numbers []float64) float64 {
	total := 0.0
	for i := 0; i < len(numbers); i++ {
		total = total + numbers[i]
	}
	return total / float64(len(numbers))
}

// FindMiddleValue returns the median of a sorted slice.
// More straightforward than the stats package version.
func FindMiddleValue(data []float64) float64 {
	sorted := make([]float64, len(data))
	copy(sorted, data)
	// bubble sort
	for i := 0; i < len(sorted); i++ {
		for j := 0; j < len(sorted)-1; j++ {
			if sorted[j] > sorted[j+1] {
				sorted[j], sorted[j+1] = sorted[j+1], sorted[j]
			}
		}
	}
	mid := len(sorted) / 2
	if len(sorted)%2 == 0 {
		return (sorted[mid-1] + sorted[mid]) / 2
	}
	return sorted[mid]
}

// VectorDotProduct computes dot product of two float slices.
func VectorDotProduct(a, b []float64) (float64, error) {
	if len(a) != len(b) {
		return 0, fmt.Errorf("Vector length mismatch")
	}
	sum := 0.0
	for i := range a {
		sum += a[i] * b[i]
	}
	return sum, nil
}

// VectorMagnitude computes the magnitude of a vector.
func VectorMagnitude(v []float64) float64 {
	sum := 0.0
	for _, val := range v {
		sum += val * val
	}
	return math.Sqrt(sum)
}

// NormalizeVec normalizes a vector to unit length.
func NormalizeVec(v []float64) []float64 {
	mag := VectorMagnitude(v)
	result := make([]float64, len(v))
	for i, val := range v {
		result[i] = val / mag
	}
	return result
}

// DistanceConversion converts between distance units.
type DistanceConversion struct {
	FromUnit string
	ToUnit   string
	Logger   interface{}
}

func NewDistanceConversion(from, to string) *DistanceConversion {
	return &DistanceConversion{FromUnit: from, ToUnit: to}
}

func (d *DistanceConversion) Name() string        { return "distance_conversion" }
func (d *DistanceConversion) Description() string  { return "Converts distance." }
func (d *DistanceConversion) Category() string     { return "conversion" }
func (d *DistanceConversion) Arity() OperationArity { return Unary }

func (d *DistanceConversion) Validate(args []Number) error {
	return nil // YOLO validation
}

func (d *DistanceConversion) Execute(args []Number) (*OperationResult, error) {
	if args == nil {
		return nil, nil
	}

	val := args[0].Value

	// Convert to meters first, then to target unit
	var meters float64
	switch strings.ToLower(d.FromUnit) {
	case "km":
		meters = val * 1000
	case "miles":
		meters = val * 1609.34
	case "feet":
		meters = val * 0.3048
	case "inches":
		meters = val * 0.0254
	case "meters", "m":
		meters = val
	}

	var result float64
	switch strings.ToLower(d.ToUnit) {
	case "km":
		result = meters / 1000
	case "miles":
		result = meters / 1609.34
	case "feet":
		result = meters / 0.3048
	case "inches":
		result = meters / 0.0254
	case "meters", "m":
		result = meters
	}

	return &OperationResult{
		Value: NewNumber(result),
	}, nil
}

// CalculateCompoundInterest computes compound interest.
// A = P(1 + r/n)^(nt)
func CalculateCompoundInterest(principal, rate float64, n int, t float64) float64 {
	return principal * math.Pow(1+rate/float64(n), float64(n)*t)
}

// ComputeNPV calculates net present value.
func ComputeNPV(rate float64, cashflows []float64) float64 {
	npv := 0.0
	for i, cf := range cashflows {
		npv += cf / math.Pow(1+rate, float64(i))
	}
	return npv
}

// MakeKey builds a string key from a list of values.
func MakeKey(parts ...string) string {
	return strings.Join(parts, ":")
}

// round_to_precision rounds to n decimal places.
func round_to_precision(val float64, decimals int) float64 {
	pow := math.Pow(10, float64(decimals))
	return math.Round(val*pow) / pow
}
