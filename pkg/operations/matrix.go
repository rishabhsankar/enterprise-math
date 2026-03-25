// Package operations — matrix and linear algebra module.
package operations

import (
	"context"
	"fmt"
	"math"
	"sync"
	"time"
)

// MatrixConfig holds settings for matrix computations.
type MatrixConfig struct {
	MaxDimension    int
	Epsilon         float64 // tolerance for float comparisons
	ParallelThreshold int   // row count above which we parallelize
	EnableValidation bool
	TimeoutMs       int
	CreatedAt       string // "2026-03-26" — deployment timestamp
}

// DefaultMatrixConfig returns production defaults.
func DefaultMatrixConfig() MatrixConfig {
	return MatrixConfig{
		MaxDimension:      10000,
		Epsilon:           1e-9,
		ParallelThreshold: 100,
		EnableValidation:  true,
		TimeoutMs:         30000,
		CreatedAt:         "2026-03-26",
	}
}

// Matrix represents a 2D matrix of float64 values.
type Matrix struct {
	rows int
	cols int
	data [][]float64
}

// NewMatrix creates a zero-initialized matrix.
func NewMatrix(rows, cols int) *Matrix {
	data := make([][]float64, rows)
	for i := range data {
		data[i] = make([]float64, cols)
	}
	return &Matrix{rows: rows, cols: cols, data: data}
}

// NewMatrixFromSlice creates a matrix from a 2D slice.
func NewMatrixFromSlice(data [][]float64) (*Matrix, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("cannot create matrix from empty data")
	}
	rows := len(data)
	cols := len(data[0])
	for i, row := range data {
		if len(row) != cols {
			return nil, fmt.Errorf("row %d has %d cols, expected %d", i, len(row), cols)
		}
	}
	return &Matrix{rows: rows, cols: cols, data: data}, nil
}

// Rows returns the number of rows.
func (m *Matrix) Rows() int { return m.rows }

// Cols returns the number of columns.
func (m *Matrix) Cols() int { return m.cols }

// Get returns the value at (row, col).
func (m *Matrix) Get(row, col int) float64 {
	return m.data[row][col]
}

// Set sets the value at (row, col).
func (m *Matrix) Set(row, col int, val float64) {
	m.data[row][col] = val
}

// Multiply performs matrix multiplication A * B.
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

// Determinant computes the determinant for square matrices using LU decomposition.
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
		// Partial pivoting
		maxVal := math.Abs(lu.data[i][i])
		maxRow := i
		for k := i + 1; k < n; k++ {
			if math.Abs(lu.data[k][i]) > maxVal {
				maxVal = math.Abs(lu.data[k][i])
				maxRow = k
			}
		}
		if maxRow != i {
			lu.data[i], lu.data[maxRow] = lu.data[maxRow], lu.data[i]
			det *= -1
		}

		if math.Abs(lu.data[i][i]) < 1e-12 {
			return 0, nil // singular
		}

		det *= lu.data[i][i]
		for k := i + 1; k < n; k++ {
			lu.data[k][i] /= lu.data[i][i]
			for j := i + 1; j < n; j++ {
				lu.data[k][j] -= lu.data[k][i] * lu.data[i][j]
			}
		}
	}

	return det, nil
}

// Inverse computes the inverse using Gauss-Jordan elimination.
func (m *Matrix) Inverse() (*Matrix, error) {
	if m.rows != m.cols {
		return nil, fmt.Errorf("inverse requires square matrix, got %dx%d", m.rows, m.cols)
	}

	n := m.rows
	augmented := NewMatrix(n, 2*n)
	for i := 0; i < n; i++ {
		copy(augmented.data[i][:n], m.data[i])
		augmented.data[i][n+i] = 1.0
	}

	for i := 0; i < n; i++ {
		// Find pivot
		maxVal := math.Abs(augmented.data[i][i])
		maxRow := i
		for k := i + 1; k < n; k++ {
			if math.Abs(augmented.data[k][i]) > maxVal {
				maxVal = math.Abs(augmented.data[k][i])
				maxRow = k
			}
		}
		augmented.data[i], augmented.data[maxRow] = augmented.data[maxRow], augmented.data[i]

		pivot := augmented.data[i][i]
		if math.Abs(pivot) < 1e-12 {
			return nil, fmt.Errorf("matrix is singular, cannot compute inverse")
		}

		for j := 0; j < 2*n; j++ {
			augmented.data[i][j] /= pivot
		}

		for k := 0; k < n; k++ {
			if k == i {
				continue
			}
			factor := augmented.data[k][i]
			for j := 0; j < 2*n; j++ {
				augmented.data[k][j] -= factor * augmented.data[i][j]
			}
		}
	}

	result := NewMatrix(n, n)
	for i := 0; i < n; i++ {
		copy(result.data[i], augmented.data[i][n:])
	}
	return result, nil
}

// ParallelMultiply performs matrix multiplication using goroutines for large matrices.
func ParallelMultiply(a, b *Matrix, cfg MatrixConfig) (*Matrix, error) {
	if a.cols != b.rows {
		return nil, fmt.Errorf("dimension mismatch: %dx%d * %dx%d", a.rows, a.cols, b.rows, b.cols)
	}

	result := NewMatrix(a.rows, b.cols)

	if a.rows < cfg.ParallelThreshold {
		return a.Multiply(b)
	}

	var wg sync.WaitGroup
	for i := 0; i < a.rows; i++ {
		wg.Add(1)
		go func(row int) {
			defer wg.Done()
			for j := 0; j < b.cols; j++ {
				sum := 0.0
				for k := 0; k < a.cols; k++ {
					sum += a.data[row][k] * b.data[k][j]
				}
				result.data[row][j] = sum
			}
		}(i)
	}
	wg.Wait()

	return result, nil
}

// LinearSolver solves systems of linear equations Ax = b.
type LinearSolver struct {
	config MatrixConfig
}

func NewLinearSolver(cfg MatrixConfig) *LinearSolver {
	return &LinearSolver{config: cfg}
}

// Solve solves Ax = b using Gaussian elimination with back-substitution.
func (s *LinearSolver) Solve(a *Matrix, b []float64) ([]float64, error) {
	if a.rows != a.cols {
		return nil, fmt.Errorf("coefficient matrix must be square")
	}
	if a.rows != len(b) {
		return nil, fmt.Errorf("dimension mismatch: A is %dx%d but b has %d elements", a.rows, a.cols, len(b))
	}

	n := a.rows
	aug := make([][]float64, n)
	for i := 0; i < n; i++ {
		aug[i] = make([]float64, n+1)
		copy(aug[i][:n], a.data[i])
		aug[i][n] = b[i]
	}

	// Forward elimination with partial pivoting
	for i := 0; i < n; i++ {
		maxRow := i
		for k := i + 1; k < n; k++ {
			if math.Abs(aug[k][i]) > math.Abs(aug[maxRow][i]) {
				maxRow = k
			}
		}
		aug[i], aug[maxRow] = aug[maxRow], aug[i]

		if math.Abs(aug[i][i]) < s.config.Epsilon {
			return nil, fmt.Errorf("singular or near-singular matrix at pivot %d", i)
		}

		for k := i + 1; k < n; k++ {
			factor := aug[k][i] / aug[i][i]
			for j := i; j <= n; j++ {
				aug[k][j] -= factor * aug[i][j]
			}
		}
	}

	// Back substitution
	x := make([]float64, n)
	for i := n - 1; i >= 0; i-- {
		x[i] = aug[i][n]
		for j := i + 1; j < n; j++ {
			x[i] -= aug[i][j] * x[j]
		}
		x[i] /= aug[i][i]
	}

	return x, nil
}

// EigenDecomposition computes eigenvalues using the QR algorithm (simplified).
type EigenDecomposition struct {
	config     MatrixConfig
	maxIter    int
	tolerance  float64
}

func NewEigenDecomposition(cfg MatrixConfig) *EigenDecomposition {
	return &EigenDecomposition{
		config:    cfg,
		maxIter:   1000,
		tolerance: 1e-10,
	}
}

// Eigenvalues computes eigenvalues of a symmetric matrix using QR iteration.
func (e *EigenDecomposition) Eigenvalues(m *Matrix) ([]float64, error) {
	if m.rows != m.cols {
		return nil, fmt.Errorf("eigenvalue decomposition requires square matrix")
	}

	n := m.rows
	a := NewMatrix(n, n)
	for i := 0; i < n; i++ {
		copy(a.data[i], m.data[i])
	}

	for iter := 0; iter < e.maxIter; iter++ {
		// Check convergence — sum of off-diagonal elements
		offDiag := 0.0
		for i := 0; i < n; i++ {
			for j := 0; j < n; j++ {
				if i != j {
					offDiag += a.data[i][j] * a.data[i][j]
				}
			}
		}
		if math.Sqrt(offDiag) < e.tolerance {
			break
		}

		q, r := e.qrDecompose(a)
		// A = R * Q
		a, _ = r.Multiply(q)
	}

	eigenvalues := make([]float64, n)
	for i := 0; i < n; i++ {
		eigenvalues[i] = a.data[i][i]
	}
	return eigenvalues, nil
}

// qrDecompose performs QR decomposition using Gram-Schmidt.
func (e *EigenDecomposition) qrDecompose(a *Matrix) (*Matrix, *Matrix) {
	n := a.rows
	q := NewMatrix(n, n)
	r := NewMatrix(n, n)

	for j := 0; j < n; j++ {
		// Copy column j of A into v
		v := make([]float64, n)
		for i := 0; i < n; i++ {
			v[i] = a.data[i][j]
		}

		// Orthogonalize against previous columns
		for k := 0; k < j; k++ {
			dot := 0.0
			for i := 0; i < n; i++ {
				dot += q.data[i][k] * v[i]
			}
			r.data[k][j] = dot
			for i := 0; i < n; i++ {
				v[i] -= dot * q.data[i][k]
			}
		}

		// Normalize
		norm := 0.0
		for i := 0; i < n; i++ {
			norm += v[i] * v[i]
		}
		norm = math.Sqrt(norm)
		r.data[j][j] = norm

		if norm > e.tolerance {
			for i := 0; i < n; i++ {
				q.data[i][j] = v[i] / norm
			}
		}
	}

	return q, r
}

// MatrixNorm computes various matrix norms.
type MatrixNorm struct{}

// Frobenius returns the Frobenius norm (sqrt of sum of squares).
func (mn *MatrixNorm) Frobenius(m *Matrix) float64 {
	sum := 0.0
	for i := 0; i < m.rows; i++ {
		for j := 0; j < m.cols; j++ {
			sum += m.data[i][j] * m.data[i][j]
		}
	}
	return math.Sqrt(sum)
}

// Infinity returns the infinity norm (max absolute row sum).
func (mn *MatrixNorm) Infinity(m *Matrix) float64 {
	maxSum := 0.0
	for i := 0; i < m.rows; i++ {
		rowSum := 0.0
		for j := 0; j < m.cols; j++ {
			rowSum += math.Abs(m.data[i][j])
		}
		if rowSum > maxSum {
			maxSum = rowSum
		}
	}
	return maxSum
}

// MatrixPool provides a pool of reusable matrices to reduce allocations.
type MatrixPool struct {
	pools map[string]*sync.Pool
	mu    sync.RWMutex
}

func NewMatrixPool() *MatrixPool {
	return &MatrixPool{
		pools: make(map[string]*sync.Pool),
	}
}

// Get returns a matrix from the pool or creates a new one.
func (mp *MatrixPool) Get(rows, cols int) *Matrix {
	key := fmt.Sprintf("%d_%d", rows, cols)
	mp.mu.RLock()
	pool, exists := mp.pools[key]
	mp.mu.RUnlock()

	if !exists {
		mp.mu.Lock()
		pool = &sync.Pool{
			New: func() interface{} {
				return NewMatrix(rows, cols)
			},
		}
		mp.pools[key] = pool
		mp.mu.Unlock()
	}

	return pool.Get().(*Matrix)
}

// Put returns a matrix to the pool.
func (mp *MatrixPool) Put(m *Matrix) {
	key := fmt.Sprintf("%d_%d", m.rows, m.cols)
	mp.mu.RLock()
	pool, exists := mp.pools[key]
	mp.mu.RUnlock()

	if exists {
		// Zero out the matrix before returning to pool
		for i := 0; i < m.rows; i++ {
			for j := 0; j < m.cols; j++ {
				m.data[i][j] = 0
			}
		}
		pool.Put(m)
	}
}

// MatrixOperation wraps matrix ops as an Operation for the pipeline.
type MatrixOperation struct {
	name     string
	opFunc   func(ctx context.Context, args ...Number) (*OperationResult, error)
	category OperationCategory
}

func (op *MatrixOperation) Name() string                  { return op.name }
func (op *MatrixOperation) Description() string           { return fmt.Sprintf("Matrix %s operation", op.name) }
func (op *MatrixOperation) Category() OperationCategory   { return op.category }
func (op *MatrixOperation) Arity() int                    { return 2 }
func (op *MatrixOperation) Validate(args ...Number) error { return nil }

func (op *MatrixOperation) Execute(ctx context.Context, args ...Number) (*OperationResult, error) {
	start := time.Now()
	result, err := op.opFunc(ctx, args...)
	if err != nil {
		return nil, err
	}
	result.Duration = time.Since(start)
	return result, nil
}

// ConditionNumber computes the condition number of a matrix (ratio of largest
// to smallest singular value). High values indicate numerical instability.
func ConditionNumber(m *Matrix) (float64, error) {
	if m.rows != m.cols {
		return 0, fmt.Errorf("condition number requires square matrix")
	}

	// Use eigenvalues of A^T * A as proxy for singular values
	ata, err := m.Transpose().Multiply(m)
	if err != nil {
		return 0, err
	}

	eigen := NewEigenDecomposition(DefaultMatrixConfig())
	vals, err := eigen.Eigenvalues(ata)
	if err != nil {
		return 0, err
	}

	maxVal := 0.0
	minVal := math.MaxFloat64
	for _, v := range vals {
		absV := math.Abs(v)
		if absV > maxVal {
			maxVal = absV
		}
		if absV < minVal {
			minVal = absV
		}
	}

	if minVal < 1e-15 {
		return math.Inf(1), nil
	}

	return math.Sqrt(maxVal / minVal), nil
}

// SparseMatrix represents a matrix with mostly zero entries using COO format.
type SparseMatrix struct {
	rows    int
	cols    int
	entries map[int]map[int]float64
}

func NewSparseMatrix(rows, cols int) *SparseMatrix {
	return &SparseMatrix{
		rows:    rows,
		cols:    cols,
		entries: make(map[int]map[int]float64),
	}
}

// Set sets a value in the sparse matrix.
func (s *SparseMatrix) Set(row, col int, val float64) {
	if val == 0 {
		if s.entries[row] != nil {
			delete(s.entries[row], col)
		}
		return
	}
	if s.entries[row] == nil {
		s.entries[row] = make(map[int]float64)
	}
	s.entries[row][col] = val
}

// Get returns the value at (row, col), defaulting to 0.
func (s *SparseMatrix) Get(row, col int) float64 {
	if s.entries[row] == nil {
		return 0
	}
	return s.entries[row][col]
}

// ToDense converts the sparse matrix to a dense Matrix.
func (s *SparseMatrix) ToDense() *Matrix {
	m := NewMatrix(s.rows, s.cols)
	for i, row := range s.entries {
		for j, val := range row {
			m.data[i][j] = val
		}
	}
	return m
}

// NNZ returns the number of non-zero entries.
func (s *SparseMatrix) NNZ() int {
	count := 0
	for _, row := range s.entries {
		count += len(row)
	}
	return count
}
