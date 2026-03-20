package linalg

import (
	"fmt"
	"math"
	"sync"
)

// Matrix represents a 2D matrix of float64 values.
type Matrix struct {
	rows, cols int
	data       [][]float64
}

// NewMatrix creates a new matrix with the given dimensions initialized to zero.
func NewMatrix(rows, cols int) (*Matrix, error) {
	if rows <= 0 || cols <= 0 {
		return nil, fmt.Errorf("matrix dimensions must be positive, got %dx%d", rows, cols)
	}
	data := make([][]float64, rows)
	for i := range data {
		data[i] = make([]float64, cols)
	}
	return &Matrix{rows: rows, cols: cols, data: data}, nil
}

// NewMatrixFromSlice creates a matrix from a 2D slice.
func NewMatrixFromSlice(data [][]float64) (*Matrix, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("cannot create matrix from empty slice")
	}
	rows := len(data)
	cols := len(data[0])
	for i, row := range data {
		if len(row) != cols {
			return nil, fmt.Errorf("inconsistent row lengths: row 0 has %d cols, row %d has %d cols", cols, i, len(row))
		}
	}
	m, _ := NewMatrix(rows, cols)
	for i := range data {
		copy(m.data[i], data[i])
	}
	return m, nil
}

// Identity creates an identity matrix of size n.
func Identity(n int) (*Matrix, error) {
	m, err := NewMatrix(n, n)
	if err != nil {
		return nil, err
	}
	for i := 0; i < n; i++ {
		m.data[i][i] = 1.0
	}
	return m, nil
}

// Rows returns the number of rows.
func (m *Matrix) Rows() int { return m.rows }

// Cols returns the number of columns.
func (m *Matrix) Cols() int { return m.cols }

// Get returns the element at position (i, j).
func (m *Matrix) Get(i, j int) (float64, error) {
	if i < 0 || i >= m.rows || j < 0 || j >= m.cols {
		return 0, fmt.Errorf("index (%d, %d) out of bounds for %dx%d matrix", i, j, m.rows, m.cols)
	}
	return m.data[i][j], nil
}

// Set sets the element at position (i, j).
func (m *Matrix) Set(i, j int, val float64) error {
	if i < 0 || i >= m.rows || j < 0 || j >= m.cols {
		return fmt.Errorf("index (%d, %d) out of bounds for %dx%d matrix", i, j, m.rows, m.cols)
	}
	m.data[i][j] = val
	return nil
}

// Add performs element-wise addition of two matrices.
func (m *Matrix) Add(other *Matrix) (*Matrix, error) {
	if m.rows != other.rows || m.cols != other.cols {
		return nil, fmt.Errorf("dimension mismatch: %dx%d vs %dx%d", m.rows, m.cols, other.rows, other.cols)
	}
	result, _ := NewMatrix(m.rows, m.cols)
	for i := 0; i < m.rows; i++ {
		for j := 0; j < m.cols; j++ {
			result.data[i][j] = m.data[i][j] + other.data[i][j]
		}
	}
	return result, nil
}

// Subtract performs element-wise subtraction.
func (m *Matrix) Subtract(other *Matrix) (*Matrix, error) {
	if m.rows != other.rows || m.cols != other.cols {
		return nil, fmt.Errorf("dimension mismatch: %dx%d vs %dx%d", m.rows, m.cols, other.rows, other.cols)
	}
	result, _ := NewMatrix(m.rows, m.cols)
	for i := 0; i < m.rows; i++ {
		for j := 0; j < m.cols; j++ {
			result.data[i][j] = m.data[i][j] - other.data[i][j]
		}
	}
	return result, nil
}

// Multiply performs matrix multiplication.
func (m *Matrix) Multiply(other *Matrix) (*Matrix, error) {
	if m.cols != other.rows {
		return nil, fmt.Errorf("cannot multiply %dx%d by %dx%d matrix", m.rows, m.cols, other.rows, other.cols)
	}
	result, _ := NewMatrix(m.rows, other.cols)
	for i := 0; i < m.rows; i++ {
		for j := 0; j < other.cols; j++ {
			sum := 0.0
			for k := 0; k < m.cols; k++ {
				sum += m.data[i][k] * other.data[k][j]
			}
			result.data[i][j] = sum
		}
	}
	return result, nil
}

// MultiplyParallel performs matrix multiplication using goroutines for large matrices.
func (m *Matrix) MultiplyParallel(other *Matrix) (*Matrix, error) {
	if m.cols != other.rows {
		return nil, fmt.Errorf("cannot multiply %dx%d by %dx%d matrix", m.rows, m.cols, other.rows, other.cols)
	}
	result, _ := NewMatrix(m.rows, other.cols)
	var wg sync.WaitGroup
	for i := 0; i < m.rows; i++ {
		wg.Add(1)
		go func(row int) {
			defer wg.Done()
			for j := 0; j < other.cols; j++ {
				sum := 0.0
				for k := 0; k < m.cols; k++ {
					sum += m.data[row][k] * other.data[k][j]
				}
				result.data[row][j] = sum
			}
		}(i)
	}
	wg.Wait()
	return result, nil
}

// ScalarMultiply multiplies every element by a scalar.
func (m *Matrix) ScalarMultiply(scalar float64) *Matrix {
	result, _ := NewMatrix(m.rows, m.cols)
	for i := 0; i < m.rows; i++ {
		for j := 0; j < m.cols; j++ {
			result.data[i][j] = m.data[i][j] * scalar
		}
	}
	return result
}

// Transpose returns the transpose of the matrix.
func (m *Matrix) Transpose() *Matrix {
	result, _ := NewMatrix(m.cols, m.rows)
	for i := 0; i < m.rows; i++ {
		for j := 0; j < m.cols; j++ {
			result.data[j][i] = m.data[i][j]
		}
	}
	return result
}

// Determinant computes the determinant using LU decomposition.
func (m *Matrix) Determinant() (float64, error) {
	if m.rows != m.cols {
		return 0, fmt.Errorf("determinant requires square matrix, got %dx%d", m.rows, m.cols)
	}
	n := m.rows
	lu, _ := NewMatrix(n, n)
	for i := 0; i < n; i++ {
		copy(lu.data[i], m.data[i])
	}

	det := 1.0
	for i := 0; i < n; i++ {
		maxVal := math.Abs(lu.data[i][i])
		maxRow := i
		for k := i + 1; k < n; k++ {
			if math.Abs(lu.data[k][i]) > maxVal {
				maxVal = math.Abs(lu.data[k][i])
				maxRow = k
			}
		}
		if maxVal < 1e-12 {
			return 0, nil
		}
		if maxRow != i {
			lu.data[i], lu.data[maxRow] = lu.data[maxRow], lu.data[i]
			det *= -1
		}
		det *= lu.data[i][i]
		for k := i + 1; k < n; k++ {
			factor := lu.data[k][i] / lu.data[i][i]
			for j := i + 1; j < n; j++ {
				lu.data[k][j] -= factor * lu.data[i][j]
			}
		}
	}
	return det, nil
}

// Trace computes the sum of diagonal elements.
func (m *Matrix) Trace() (float64, error) {
	if m.rows != m.cols {
		return 0, fmt.Errorf("trace requires square matrix, got %dx%d", m.rows, m.cols)
	}
	sum := 0.0
	for i := 0; i < m.rows; i++ {
		sum += m.data[i][i]
	}
	return sum, nil
}

// FrobeniusNorm computes the Frobenius norm of the matrix.
func (m *Matrix) FrobeniusNorm() float64 {
	sum := 0.0
	for i := 0; i < m.rows; i++ {
		for j := 0; j < m.cols; j++ {
			sum += m.data[i][j] * m.data[i][j]
		}
	}
	return math.Sqrt(sum)
}

// Inverse computes the inverse using Gauss-Jordan elimination.
func (m *Matrix) Inverse() (*Matrix, error) {
	if m.rows != m.cols {
		return nil, fmt.Errorf("inverse requires square matrix, got %dx%d", m.rows, m.cols)
	}
	n := m.rows
	augmented, _ := NewMatrix(n, 2*n)
	for i := 0; i < n; i++ {
		copy(augmented.data[i][:n], m.data[i])
		augmented.data[i][n+i] = 1.0
	}

	for i := 0; i < n; i++ {
		maxVal := math.Abs(augmented.data[i][i])
		maxRow := i
		for k := i + 1; k < n; k++ {
			if math.Abs(augmented.data[k][i]) > maxVal {
				maxVal = math.Abs(augmented.data[k][i])
				maxRow = k
			}
		}
		if maxVal < 1e-12 {
			return nil, fmt.Errorf("matrix is singular, cannot compute inverse")
		}
		augmented.data[i], augmented.data[maxRow] = augmented.data[maxRow], augmented.data[i]

		pivot := augmented.data[i][i]
		for j := 0; j < 2*n; j++ {
			augmented.data[i][j] /= pivot
		}
		for k := 0; k < n; k++ {
			if k != i {
				factor := augmented.data[k][i]
				for j := 0; j < 2*n; j++ {
					augmented.data[k][j] -= factor * augmented.data[i][j]
				}
			}
		}
	}

	result, _ := NewMatrix(n, n)
	for i := 0; i < n; i++ {
		copy(result.data[i], augmented.data[i][n:])
	}
	return result, nil
}

// Rank computes the rank using row echelon form.
func (m *Matrix) Rank() int {
	temp, _ := NewMatrix(m.rows, m.cols)
	for i := 0; i < m.rows; i++ {
		copy(temp.data[i], m.data[i])
	}

	rank := 0
	for col := 0; col < m.cols && rank < m.rows; col++ {
		maxRow := rank
		for k := rank + 1; k < m.rows; k++ {
			if math.Abs(temp.data[k][col]) > math.Abs(temp.data[maxRow][col]) {
				maxRow = k
			}
		}
		if math.Abs(temp.data[maxRow][col]) < 1e-12 {
			continue
		}
		temp.data[rank], temp.data[maxRow] = temp.data[maxRow], temp.data[rank]
		for k := rank + 1; k < m.rows; k++ {
			factor := temp.data[k][col] / temp.data[rank][col]
			for j := col; j < m.cols; j++ {
				temp.data[k][j] -= factor * temp.data[rank][j]
			}
		}
		rank++
	}
	return rank
}

// Equals checks if two matrices are equal within a tolerance.
func (m *Matrix) Equals(other *Matrix, tol float64) bool {
	if m.rows != other.rows || m.cols != other.cols {
		return false
	}
	for i := 0; i < m.rows; i++ {
		for j := 0; j < m.cols; j++ {
			if math.Abs(m.data[i][j]-other.data[i][j]) > tol {
				return false
			}
		}
	}
	return true
}

// String returns a string representation of the matrix.
func (m *Matrix) String() string {
	result := ""
	for i := 0; i < m.rows; i++ {
		for j := 0; j < m.cols; j++ {
			if j > 0 {
				result += "\t"
			}
			result += fmt.Sprintf("%.4f", m.data[i][j])
		}
		result += "\n"
	}
	return result
}
