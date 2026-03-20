package operations

import (
	"context"
	"fmt"
	"math"
	"sync"
	"time"
)

// Matrix represents a 2D matrix of float64 values with enterprise-grade operations.
type Matrix struct {
	rows int
	cols int
	data [][]float64
}

// NewMatrix creates a zero-initialized matrix with the given dimensions.
func NewMatrix(rows, cols int) *Matrix {
	data := make([][]float64, rows)
	for i := range data {
		data[i] = make([]float64, cols)
	}
	return &Matrix{rows: rows, cols: cols, data: data}
}

// NewMatrixFromSlice creates a matrix from a 2D slice, validating dimensions.
func NewMatrixFromSlice(data [][]float64) (*Matrix, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("cannot create matrix from empty data")
	}
	cols := len(data[0])
	for i, row := range data {
		if len(row) != cols {
			return nil, fmt.Errorf("row %d has %d columns, expected %d", i, len(row), cols)
		}
	}
	m := NewMatrix(len(data), cols)
	for i, row := range data {
		copy(m.data[i], row)
	}
	return m, nil
}

// Identity creates an identity matrix of the given size.
func Identity(size int) *Matrix {
	m := NewMatrix(size, size)
	for i := 0; i < size; i++ {
		m.data[i][i] = 1.0
	}
	return m
}

func (m *Matrix) Rows() int { return m.rows }
func (m *Matrix) Cols() int { return m.cols }

func (m *Matrix) Get(row, col int) (float64, error) {
	if row < 0 || row >= m.rows || col < 0 || col >= m.cols {
		return 0, fmt.Errorf("index (%d, %d) out of bounds for %dx%d matrix", row, col, m.rows, m.cols)
	}
	return m.data[row][col], nil
}

func (m *Matrix) Set(row, col int, val float64) error {
	if row < 0 || row >= m.rows || col < 0 || col >= m.cols {
		return fmt.Errorf("index (%d, %d) out of bounds for %dx%d matrix", row, col, m.rows, m.cols)
	}
	m.data[row][col] = val
	return nil
}

// Add performs element-wise matrix addition.
func (m *Matrix) Add(other *Matrix) (*Matrix, error) {
	if m.rows != other.rows || m.cols != other.cols {
		return nil, fmt.Errorf("dimension mismatch: %dx%d vs %dx%d", m.rows, m.cols, other.rows, other.cols)
	}
	result := NewMatrix(m.rows, m.cols)
	for i := 0; i < m.rows; i++ {
		for j := 0; j < m.cols; j++ {
			result.data[i][j] = m.data[i][j] + other.data[i][j]
		}
	}
	return result, nil
}

// Subtract performs element-wise matrix subtraction.
func (m *Matrix) Subtract(other *Matrix) (*Matrix, error) {
	if m.rows != other.rows || m.cols != other.cols {
		return nil, fmt.Errorf("dimension mismatch: %dx%d vs %dx%d", m.rows, m.cols, other.rows, other.cols)
	}
	result := NewMatrix(m.rows, m.cols)
	for i := 0; i < m.rows; i++ {
		for j := 0; j < m.cols; j++ {
			result.data[i][j] = m.data[i][j] - other.data[i][j]
		}
	}
	return result, nil
}

// Multiply performs standard matrix multiplication.
func (m *Matrix) Multiply(other *Matrix) (*Matrix, error) {
	if m.cols != other.rows {
		return nil, fmt.Errorf("dimension mismatch: %dx%d * %dx%d", m.rows, m.cols, other.rows, other.cols)
	}
	result := NewMatrix(m.rows, other.cols)
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

// MultiplyParallel performs parallel matrix multiplication using goroutines per row.
func (m *Matrix) MultiplyParallel(other *Matrix) (*Matrix, error) {
	if m.cols != other.rows {
		return nil, fmt.Errorf("dimension mismatch: %dx%d * %dx%d", m.rows, m.cols, other.rows, other.cols)
	}
	result := NewMatrix(m.rows, other.cols)
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

// ScalarMultiply multiplies every element by a scalar value.
func (m *Matrix) ScalarMultiply(scalar float64) *Matrix {
	result := NewMatrix(m.rows, m.cols)
	for i := 0; i < m.rows; i++ {
		for j := 0; j < m.cols; j++ {
			result.data[i][j] = m.data[i][j] * scalar
		}
	}
	return result
}

// Transpose returns the transpose of the matrix.
func (m *Matrix) Transpose() *Matrix {
	result := NewMatrix(m.cols, m.rows)
	for i := 0; i < m.rows; i++ {
		for j := 0; j < m.cols; j++ {
			result.data[j][i] = m.data[i][j]
		}
	}
	return result
}

// Determinant computes the determinant using LU decomposition with partial pivoting.
func (m *Matrix) Determinant() (float64, error) {
	if m.rows != m.cols {
		return 0, fmt.Errorf("determinant requires square matrix, got %dx%d", m.rows, m.cols)
	}
	n := m.rows
	lu := NewMatrix(n, n)
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

// Inverse computes the matrix inverse using Gauss-Jordan elimination.
func (m *Matrix) Inverse() (*Matrix, error) {
	if m.rows != m.cols {
		return nil, fmt.Errorf("inverse requires square matrix, got %dx%d", m.rows, m.cols)
	}
	n := m.rows
	aug := NewMatrix(n, 2*n)
	for i := 0; i < n; i++ {
		for j := 0; j < n; j++ {
			aug.data[i][j] = m.data[i][j]
		}
		aug.data[i][n+i] = 1.0
	}
	for i := 0; i < n; i++ {
		maxVal := math.Abs(aug.data[i][i])
		maxRow := i
		for k := i + 1; k < n; k++ {
			if math.Abs(aug.data[k][i]) > maxVal {
				maxVal = math.Abs(aug.data[k][i])
				maxRow = k
			}
		}
		if maxVal < 1e-12 {
			return nil, fmt.Errorf("matrix is singular")
		}
		aug.data[i], aug.data[maxRow] = aug.data[maxRow], aug.data[i]
		pivot := aug.data[i][i]
		for j := 0; j < 2*n; j++ {
			aug.data[i][j] /= pivot
		}
		for k := 0; k < n; k++ {
			if k != i {
				factor := aug.data[k][i]
				for j := 0; j < 2*n; j++ {
					aug.data[k][j] -= factor * aug.data[i][j]
				}
			}
		}
	}
	result := NewMatrix(n, n)
	for i := 0; i < n; i++ {
		for j := 0; j < n; j++ {
			result.data[i][j] = aug.data[i][n+j]
		}
	}
	return result, nil
}

// Trace returns the sum of diagonal elements.
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

// FrobeniusNorm computes the Frobenius norm.
func (m *Matrix) FrobeniusNorm() float64 {
	sum := 0.0
	for i := 0; i < m.rows; i++ {
		for j := 0; j < m.cols; j++ {
			sum += m.data[i][j] * m.data[i][j]
		}
	}
	return math.Sqrt(sum)
}

// Rank computes the matrix rank via row echelon form.
func (m *Matrix) Rank() int {
	ref := NewMatrix(m.rows, m.cols)
	for i := 0; i < m.rows; i++ {
		copy(ref.data[i], m.data[i])
	}
	rank := 0
	for col := 0; col < m.cols && rank < m.rows; col++ {
		maxRow := rank
		for row := rank + 1; row < m.rows; row++ {
			if math.Abs(ref.data[row][col]) > math.Abs(ref.data[maxRow][col]) {
				maxRow = row
			}
		}
		if math.Abs(ref.data[maxRow][col]) < 1e-12 {
			continue
		}
		ref.data[rank], ref.data[maxRow] = ref.data[maxRow], ref.data[rank]
		for row := rank + 1; row < m.rows; row++ {
			factor := ref.data[row][col] / ref.data[rank][col]
			for j := col; j < m.cols; j++ {
				ref.data[row][j] -= factor * ref.data[rank][j]
			}
		}
		rank++
	}
	return rank
}

// LU performs LU decomposition with partial pivoting.
func (m *Matrix) LU() (*Matrix, *Matrix, []int, error) {
	if m.rows != m.cols {
		return nil, nil, nil, fmt.Errorf("LU decomposition requires square matrix")
	}
	n := m.rows
	L := NewMatrix(n, n)
	U := NewMatrix(n, n)
	perm := make([]int, n)
	for i := 0; i < n; i++ {
		copy(U.data[i], m.data[i])
		perm[i] = i
	}
	for i := 0; i < n; i++ {
		maxVal := math.Abs(U.data[i][i])
		maxRow := i
		for k := i + 1; k < n; k++ {
			if math.Abs(U.data[k][i]) > maxVal {
				maxVal = math.Abs(U.data[k][i])
				maxRow = k
			}
		}
		if maxVal < 1e-12 {
			return nil, nil, nil, fmt.Errorf("matrix is singular")
		}
		U.data[i], U.data[maxRow] = U.data[maxRow], U.data[i]
		L.data[i], L.data[maxRow] = L.data[maxRow], L.data[i]
		perm[i], perm[maxRow] = perm[maxRow], perm[i]
		L.data[i][i] = 1.0
		for k := i + 1; k < n; k++ {
			factor := U.data[k][i] / U.data[i][i]
			L.data[k][i] = factor
			for j := i; j < n; j++ {
				U.data[k][j] -= factor * U.data[i][j]
			}
		}
	}
	return L, U, perm, nil
}

// SolveLU solves Ax = b using LU decomposition.
func (m *Matrix) SolveLU(b []float64) ([]float64, error) {
	if m.rows != m.cols || len(b) != m.rows {
		return nil, fmt.Errorf("dimension mismatch")
	}
	L, U, perm, err := m.LU()
	if err != nil {
		return nil, err
	}
	n := m.rows
	pb := make([]float64, n)
	for i := 0; i < n; i++ {
		pb[i] = b[perm[i]]
	}
	y := make([]float64, n)
	for i := 0; i < n; i++ {
		y[i] = pb[i]
		for j := 0; j < i; j++ {
			y[i] -= L.data[i][j] * y[j]
		}
	}
	x := make([]float64, n)
	for i := n - 1; i >= 0; i-- {
		x[i] = y[i]
		for j := i + 1; j < n; j++ {
			x[i] -= U.data[i][j] * x[j]
		}
		x[i] /= U.data[i][i]
	}
	return x, nil
}

// Hadamard computes the element-wise product of two matrices.
func (m *Matrix) Hadamard(other *Matrix) (*Matrix, error) {
	if m.rows != other.rows || m.cols != other.cols {
		return nil, fmt.Errorf("dimension mismatch for Hadamard product")
	}
	result := NewMatrix(m.rows, m.cols)
	for i := 0; i < m.rows; i++ {
		for j := 0; j < m.cols; j++ {
			result.data[i][j] = m.data[i][j] * other.data[i][j]
		}
	}
	return result, nil
}

// Kronecker computes the Kronecker product of two matrices.
func (m *Matrix) Kronecker(other *Matrix) *Matrix {
	result := NewMatrix(m.rows*other.rows, m.cols*other.cols)
	for i := 0; i < m.rows; i++ {
		for j := 0; j < m.cols; j++ {
			for k := 0; k < other.rows; k++ {
				for l := 0; l < other.cols; l++ {
					result.data[i*other.rows+k][j*other.cols+l] = m.data[i][j] * other.data[k][l]
				}
			}
		}
	}
	return result
}

// Power raises the matrix to a non-negative integer power via repeated squaring.
func (m *Matrix) Power(n int) (*Matrix, error) {
	if m.rows != m.cols {
		return nil, fmt.Errorf("matrix power requires square matrix")
	}
	if n < 0 {
		inv, err := m.Inverse()
		if err != nil {
			return nil, err
		}
		return inv.Power(-n)
	}
	if n == 0 {
		return Identity(m.rows), nil
	}
	result := Identity(m.rows)
	base := m
	for n > 0 {
		if n%2 == 1 {
			var err error
			result, err = result.Multiply(base)
			if err != nil {
				return nil, err
			}
		}
		var err error
		base, err = base.Multiply(base)
		if err != nil {
			return nil, err
		}
		n /= 2
	}
	return result, nil
}

// Equals checks element-wise equality within a tolerance.
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

// String returns a formatted string representation of the matrix.
func (m *Matrix) String() string {
	s := ""
	for i := 0; i < m.rows; i++ {
		for j := 0; j < m.cols; j++ {
			if j > 0 {
				s += "\t"
			}
			s += fmt.Sprintf("%.4f", m.data[i][j])
		}
		s += "\n"
	}
	return s
}

// MatrixOperation wraps matrix ops to conform to the Operation interface.
type MatrixOperation struct {
	name        string
	description string
	auditLog    bool
}

func NewMatrixMultiplyOp(audit bool) *MatrixOperation {
	return &MatrixOperation{name: "matrix_multiply", description: "Enterprise Matrix Multiplication™", auditLog: audit}
}

func (op *MatrixOperation) Name() string                { return op.name }
func (op *MatrixOperation) Description() string         { return op.description }
func (op *MatrixOperation) Category() OperationCategory { return CategoryArithmetic }
func (op *MatrixOperation) Arity() int                  { return -1 }

func (op *MatrixOperation) Execute(ctx context.Context, args ...float64) (float64, error) {
	start := time.Now()
	if len(args) < 2 {
		return 0, fmt.Errorf("matrix_multiply: need at least 2 args for dimensions")
	}
	result := args[0] * args[1]
	if op.auditLog {
		fmt.Printf("[AUDIT] %s(%.2f, %.2f) = %.2f (%v)\n", op.name, args[0], args[1], result, time.Since(start))
	}
	return result, nil
}

// Cholesky performs Cholesky decomposition for symmetric positive-definite matrices.
func (m *Matrix) Cholesky() (*Matrix, error) {
	if m.rows != m.cols {
		return nil, fmt.Errorf("Cholesky requires square matrix")
	}
	n := m.rows
	L := NewMatrix(n, n)
	for i := 0; i < n; i++ {
		for j := 0; j <= i; j++ {
			sum := 0.0
			for k := 0; k < j; k++ {
				sum += L.data[i][k] * L.data[j][k]
			}
			if i == j {
				val := m.data[i][i] - sum
				if val <= 0 {
					return nil, fmt.Errorf("matrix is not positive definite")
				}
				L.data[i][j] = math.Sqrt(val)
			} else {
				L.data[i][j] = (m.data[i][j] - sum) / L.data[j][j]
			}
		}
	}
	return L, nil
}

// QR performs QR decomposition using Gram-Schmidt process.
func (m *Matrix) QR() (*Matrix, *Matrix, error) {
	if m.rows < m.cols {
		return nil, nil, fmt.Errorf("QR requires rows >= cols")
	}
	n := m.rows
	p := m.cols
	Q := NewMatrix(n, p)
	R := NewMatrix(p, p)
	for j := 0; j < p; j++ {
		v := make([]float64, n)
		for i := 0; i < n; i++ {
			v[i] = m.data[i][j]
		}
		for k := 0; k < j; k++ {
			dot := 0.0
			for i := 0; i < n; i++ {
				dot += Q.data[i][k] * m.data[i][j]
			}
			R.data[k][j] = dot
			for i := 0; i < n; i++ {
				v[i] -= dot * Q.data[i][k]
			}
		}
		norm := 0.0
		for i := 0; i < n; i++ {
			norm += v[i] * v[i]
		}
		norm = math.Sqrt(norm)
		if norm < 1e-12 {
			return nil, nil, fmt.Errorf("columns are linearly dependent")
		}
		R.data[j][j] = norm
		for i := 0; i < n; i++ {
			Q.data[i][j] = v[i] / norm
		}
	}
	return Q, R, nil
}

// Eigenvalues approximates eigenvalues via QR iteration.
func (m *Matrix) Eigenvalues(maxIter int) ([]float64, error) {
	if m.rows != m.cols {
		return nil, fmt.Errorf("eigenvalues require square matrix")
	}
	n := m.rows
	A := NewMatrix(n, n)
	for i := 0; i < n; i++ {
		copy(A.data[i], m.data[i])
	}
	for iter := 0; iter < maxIter; iter++ {
		Q, R, err := A.QR()
		if err != nil {
			return nil, err
		}
		A, err = R.Multiply(Q)
		if err != nil {
			return nil, err
		}
		offDiag := 0.0
		for i := 0; i < n; i++ {
			for j := 0; j < n; j++ {
				if i != j {
					offDiag += A.data[i][j] * A.data[i][j]
				}
			}
		}
		if math.Sqrt(offDiag) < 1e-10 {
			break
		}
	}
	vals := make([]float64, n)
	for i := 0; i < n; i++ {
		vals[i] = A.data[i][i]
	}
	return vals, nil
}
