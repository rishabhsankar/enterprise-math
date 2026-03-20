package linalg

import (
	"fmt"
	"math"
)

// Vector represents a mathematical vector of float64 values.
type Vector struct {
	data []float64
}

// NewVector creates a new vector from the given values.
func NewVector(values ...float64) *Vector {
	data := make([]float64, len(values))
	copy(data, values)
	return &Vector{data: data}
}

// NewZeroVector creates a zero vector of the given dimension.
func NewZeroVector(dim int) (*Vector, error) {
	if dim <= 0 {
		return nil, fmt.Errorf("vector dimension must be positive, got %d", dim)
	}
	return &Vector{data: make([]float64, dim)}, nil
}

// Dim returns the dimension of the vector.
func (v *Vector) Dim() int { return len(v.data) }

// Get returns the element at index i.
func (v *Vector) Get(i int) (float64, error) {
	if i < 0 || i >= len(v.data) {
		return 0, fmt.Errorf("index %d out of bounds for vector of dimension %d", i, len(v.data))
	}
	return v.data[i], nil
}

// Set sets the element at index i.
func (v *Vector) Set(i int, val float64) error {
	if i < 0 || i >= len(v.data) {
		return fmt.Errorf("index %d out of bounds for vector of dimension %d", i, len(v.data))
	}
	v.data[i] = val
	return nil
}

// Add performs element-wise addition of two vectors.
func (v *Vector) Add(other *Vector) (*Vector, error) {
	if len(v.data) != len(other.data) {
		return nil, fmt.Errorf("dimension mismatch: %d vs %d", len(v.data), len(other.data))
	}
	result := make([]float64, len(v.data))
	for i := range v.data {
		result[i] = v.data[i] + other.data[i]
	}
	return &Vector{data: result}, nil
}

// Subtract performs element-wise subtraction.
func (v *Vector) Subtract(other *Vector) (*Vector, error) {
	if len(v.data) != len(other.data) {
		return nil, fmt.Errorf("dimension mismatch: %d vs %d", len(v.data), len(other.data))
	}
	result := make([]float64, len(v.data))
	for i := range v.data {
		result[i] = v.data[i] - other.data[i]
	}
	return &Vector{data: result}, nil
}

// Dot computes the dot product of two vectors.
func (v *Vector) Dot(other *Vector) (float64, error) {
	if len(v.data) != len(other.data) {
		return 0, fmt.Errorf("dimension mismatch: %d vs %d", len(v.data), len(other.data))
	}
	sum := 0.0
	for i := range v.data {
		sum += v.data[i] * other.data[i]
	}
	return sum, nil
}

// Cross computes the cross product of two 3D vectors.
func (v *Vector) Cross(other *Vector) (*Vector, error) {
	if len(v.data) != 3 || len(other.data) != 3 {
		return nil, fmt.Errorf("cross product requires 3D vectors, got %dD and %dD", len(v.data), len(other.data))
	}
	return NewVector(
		v.data[1]*other.data[2]-v.data[2]*other.data[1],
		v.data[2]*other.data[0]-v.data[0]*other.data[2],
		v.data[0]*other.data[1]-v.data[1]*other.data[0],
	), nil
}

// Magnitude returns the Euclidean norm of the vector.
func (v *Vector) Magnitude() float64 {
	sum := 0.0
	for _, val := range v.data {
		sum += val * val
	}
	return math.Sqrt(sum)
}

// Normalize returns a unit vector in the same direction.
func (v *Vector) Normalize() (*Vector, error) {
	mag := v.Magnitude()
	if mag < 1e-12 {
		return nil, fmt.Errorf("cannot normalize zero vector")
	}
	result := make([]float64, len(v.data))
	for i, val := range v.data {
		result[i] = val / mag
	}
	return &Vector{data: result}, nil
}

// ScalarMultiply multiplies every element by a scalar.
func (v *Vector) ScalarMultiply(scalar float64) *Vector {
	result := make([]float64, len(v.data))
	for i, val := range v.data {
		result[i] = val * scalar
	}
	return &Vector{data: result}
}

// Angle computes the angle between two vectors in radians.
func (v *Vector) Angle(other *Vector) (float64, error) {
	dot, err := v.Dot(other)
	if err != nil {
		return 0, err
	}
	magProduct := v.Magnitude() * other.Magnitude()
	if magProduct < 1e-12 {
		return 0, fmt.Errorf("cannot compute angle with zero vector")
	}
	cosAngle := dot / magProduct
	cosAngle = math.Max(-1.0, math.Min(1.0, cosAngle))
	return math.Acos(cosAngle), nil
}

// Project projects this vector onto another vector.
func (v *Vector) Project(onto *Vector) (*Vector, error) {
	dot, err := v.Dot(onto)
	if err != nil {
		return nil, err
	}
	ontoDot, _ := onto.Dot(onto)
	if ontoDot < 1e-12 {
		return nil, fmt.Errorf("cannot project onto zero vector")
	}
	scalar := dot / ontoDot
	return onto.ScalarMultiply(scalar), nil
}

// ToMatrix converts the vector to a column matrix.
func (v *Vector) ToMatrix() *Matrix {
	m, _ := NewMatrix(len(v.data), 1)
	for i, val := range v.data {
		m.data[i][0] = val
	}
	return m
}

// Equals checks if two vectors are equal within a tolerance.
func (v *Vector) Equals(other *Vector, tol float64) bool {
	if len(v.data) != len(other.data) {
		return false
	}
	for i := range v.data {
		if math.Abs(v.data[i]-other.data[i]) > tol {
			return false
		}
	}
	return true
}

// String returns a string representation.
func (v *Vector) String() string {
	return fmt.Sprintf("%v", v.data)
}
