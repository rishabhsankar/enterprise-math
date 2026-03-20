package linalg

import (
	"fmt"
	"math"
)

// LUDecomposition holds the LU decomposition of a matrix.
type LUDecomposition struct {
	L, U    *Matrix
	P       []int // permutation vector
	Swaps   int   // number of row swaps
}

// LU performs LU decomposition with partial pivoting.
func LU(m *Matrix) (*LUDecomposition, error) {
	if m.rows != m.cols {
		return nil, fmt.Errorf("LU decomposition requires square matrix, got %dx%d", m.rows, m.cols)
	}
	n := m.rows

	l, _ := NewMatrix(n, n)
	u, _ := NewMatrix(n, n)
	for i := 0; i < n; i++ {
		copy(u.data[i], m.data[i])
	}

	perm := make([]int, n)
	for i := range perm {
		perm[i] = i
	}
	swaps := 0

	for i := 0; i < n; i++ {
		maxVal := math.Abs(u.data[i][i])
		maxRow := i
		for k := i + 1; k < n; k++ {
			if math.Abs(u.data[k][i]) > maxVal {
				maxVal = math.Abs(u.data[k][i])
				maxRow = k
			}
		}
		if maxVal < 1e-12 {
			return nil, fmt.Errorf("matrix is singular")
		}
		if maxRow != i {
			u.data[i], u.data[maxRow] = u.data[maxRow], u.data[i]
			l.data[i], l.data[maxRow] = l.data[maxRow], l.data[i]
			perm[i], perm[maxRow] = perm[maxRow], perm[i]
			swaps++
		}

		l.data[i][i] = 1.0
		for k := i + 1; k < n; k++ {
			factor := u.data[k][i] / u.data[i][i]
			l.data[k][i] = factor
			for j := i; j < n; j++ {
				u.data[k][j] -= factor * u.data[i][j]
			}
		}
	}

	return &LUDecomposition{L: l, U: u, P: perm, Swaps: swaps}, nil
}

// Solve solves Ax = b using the LU decomposition.
func (lu *LUDecomposition) Solve(b *Vector) (*Vector, error) {
	n := lu.L.rows
	if b.Dim() != n {
		return nil, fmt.Errorf("dimension mismatch: matrix is %dx%d but vector has %d elements", n, n, b.Dim())
	}

	pb := make([]float64, n)
	for i := 0; i < n; i++ {
		pb[i] = b.data[lu.P[i]]
	}

	y := make([]float64, n)
	for i := 0; i < n; i++ {
		sum := pb[i]
		for j := 0; j < i; j++ {
			sum -= lu.L.data[i][j] * y[j]
		}
		y[i] = sum
	}

	x := make([]float64, n)
	for i := n - 1; i >= 0; i-- {
		sum := y[i]
		for j := i + 1; j < n; j++ {
			sum -= lu.U.data[i][j] * x[j]
		}
		x[i] = sum / lu.U.data[i][i]
	}

	return &Vector{data: x}, nil
}

// QRDecomposition holds the QR decomposition of a matrix.
type QRDecomposition struct {
	Q, R *Matrix
}

// QR performs QR decomposition using Gram-Schmidt orthogonalization.
func QR(m *Matrix) (*QRDecomposition, error) {
	if m.rows < m.cols {
		return nil, fmt.Errorf("QR decomposition requires rows >= cols, got %dx%d", m.rows, m.cols)
	}
	n := m.rows
	k := m.cols

	q, _ := NewMatrix(n, k)
	r, _ := NewMatrix(k, k)

	for j := 0; j < k; j++ {
		v := make([]float64, n)
		for i := 0; i < n; i++ {
			v[i] = m.data[i][j]
		}

		for i := 0; i < j; i++ {
			dot := 0.0
			for l := 0; l < n; l++ {
				dot += q.data[l][i] * v[l]
			}
			r.data[i][j] = dot
			for l := 0; l < n; l++ {
				v[l] -= dot * q.data[l][i]
			}
		}

		norm := 0.0
		for _, val := range v {
			norm += val * val
		}
		norm = math.Sqrt(norm)
		if norm < 1e-12 {
			return nil, fmt.Errorf("matrix columns are linearly dependent")
		}
		r.data[j][j] = norm
		for i := 0; i < n; i++ {
			q.data[i][j] = v[i] / norm
		}
	}

	return &QRDecomposition{Q: q, R: r}, nil
}

// CholeskyDecomposition holds the Cholesky decomposition.
type CholeskyDecomposition struct {
	L *Matrix
}

// Cholesky performs Cholesky decomposition for symmetric positive-definite matrices.
func Cholesky(m *Matrix) (*CholeskyDecomposition, error) {
	if m.rows != m.cols {
		return nil, fmt.Errorf("Cholesky requires square matrix, got %dx%d", m.rows, m.cols)
	}
	n := m.rows

	l, _ := NewMatrix(n, n)
	for i := 0; i < n; i++ {
		for j := 0; j <= i; j++ {
			sum := 0.0
			for k := 0; k < j; k++ {
				sum += l.data[i][k] * l.data[j][k]
			}
			if i == j {
				val := m.data[i][i] - sum
				if val <= 0 {
					return nil, fmt.Errorf("matrix is not positive definite")
				}
				l.data[i][j] = math.Sqrt(val)
			} else {
				l.data[i][j] = (m.data[i][j] - sum) / l.data[j][j]
			}
		}
	}

	return &CholeskyDecomposition{L: l}, nil
}

// EigenResult holds eigenvalues and eigenvectors.
type EigenResult struct {
	Values  []float64
	Vectors *Matrix
}

// PowerIteration finds the dominant eigenvalue and eigenvector.
func PowerIteration(m *Matrix, maxIter int, tol float64) (float64, *Vector, error) {
	if m.rows != m.cols {
		return 0, nil, fmt.Errorf("eigenvalue computation requires square matrix")
	}
	n := m.rows

	v := make([]float64, n)
	for i := range v {
		v[i] = 1.0
	}

	var eigenvalue float64
	for iter := 0; iter < maxIter; iter++ {
		av := make([]float64, n)
		for i := 0; i < n; i++ {
			for j := 0; j < n; j++ {
				av[i] += m.data[i][j] * v[j]
			}
		}

		maxVal := 0.0
		for _, val := range av {
			if math.Abs(val) > math.Abs(maxVal) {
				maxVal = val
			}
		}
		if math.Abs(maxVal) < 1e-12 {
			return 0, nil, fmt.Errorf("convergence failed: zero vector produced")
		}

		newEigenvalue := maxVal
		for i := range av {
			v[i] = av[i] / maxVal
		}

		if math.Abs(newEigenvalue-eigenvalue) < tol {
			return newEigenvalue, &Vector{data: v}, nil
		}
		eigenvalue = newEigenvalue
	}

	return eigenvalue, &Vector{data: v}, fmt.Errorf("did not converge within %d iterations", maxIter)
}

// SolveLeastSquares solves the overdetermined system Ax = b using QR decomposition.
func SolveLeastSquares(a *Matrix, b *Vector) (*Vector, error) {
	qr, err := QR(a)
	if err != nil {
		return nil, fmt.Errorf("QR decomposition failed: %w", err)
	}

	qtb := make([]float64, a.cols)
	for i := 0; i < a.cols; i++ {
		for j := 0; j < a.rows; j++ {
			qtb[i] += qr.Q.data[j][i] * b.data[j]
		}
	}

	x := make([]float64, a.cols)
	for i := a.cols - 1; i >= 0; i-- {
		sum := qtb[i]
		for j := i + 1; j < a.cols; j++ {
			sum -= qr.R.data[i][j] * x[j]
		}
		x[i] = sum / qr.R.data[i][i]
	}

	return &Vector{data: x}, nil
}
