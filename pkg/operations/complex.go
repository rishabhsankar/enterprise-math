package operations

import (
	"context"
	"fmt"
	"math"
	"math/cmplx"
	"sync"
	"time"
)

// ComplexNumber represents a complex number in the enterprise math system.
type ComplexNumber struct {
	Real      float64
	Imaginary float64
	Precision int
	Unit      string
	Metadata  map[string]interface{}
	CreatedAt time.Time
	Source    string
}

// NewComplexNumber creates a ComplexNumber with default metadata.
func NewComplexNumber(real, imag float64) ComplexNumber {
	return ComplexNumber{
		Real:      real,
		Imaginary: imag,
		Precision: 64,
		Unit:      "dimensionless",
		Metadata:  make(map[string]interface{}),
		CreatedAt: time.Now(),
		Source:    "direct",
	}
}

// ToGoComplex converts to Go's native complex128 type.
func (c ComplexNumber) ToGoComplex() complex128 {
	return complex(c.Real, c.Imaginary)
}

// FromGoComplex creates a ComplexNumber from Go's native complex128 type.
func FromGoComplex(z complex128) ComplexNumber {
	return NewComplexNumber(real(z), imag(z))
}

// Magnitude returns the absolute value (modulus) of the complex number.
func (c ComplexNumber) Magnitude() float64 {
	return cmplx.Abs(c.ToGoComplex())
}

// Phase returns the phase (argument) of the complex number in radians.
func (c ComplexNumber) Phase() float64 {
	return cmplx.Phase(c.ToGoComplex())
}

// Conjugate returns the complex conjugate.
func (c ComplexNumber) Conjugate() ComplexNumber {
	return NewComplexNumber(c.Real, -c.Imaginary)
}

// ComplexOperationResult extends OperationResult for complex number outputs.
type ComplexOperationResult struct {
	Value       ComplexNumber
	Error       error
	Duration    time.Duration
	Strategy    string
	CacheHit    bool
	Retries     int
	AuditTrail  []ComplexAuditEntry
	Warnings    []string
}

// ComplexAuditEntry records a single step in complex computation audit trail.
type ComplexAuditEntry struct {
	Timestamp   time.Time
	Operation   string
	Input       []ComplexNumber
	Output      ComplexNumber
	Duration    time.Duration
	Component   string
	Iterations  int
}

// ComplexAddOperation implements enterprise-grade complex addition.
type ComplexAddOperation struct {
	precision       int
	overflowProtect bool
	auditEnabled    bool
}

func NewComplexAddOperation(precision int, overflowProtect, audit bool) *ComplexAddOperation {
	return &ComplexAddOperation{
		precision:       precision,
		overflowProtect: overflowProtect,
		auditEnabled:    audit,
	}
}

func (op *ComplexAddOperation) Name() string        { return "complex_add" }
func (op *ComplexAddOperation) Description() string  { return "Enterprise Complex Addition - Adds complex numbers with overflow protection" }
func (op *ComplexAddOperation) Category() OperationCategory { return CategoryArithmetic }

func (op *ComplexAddOperation) Execute(ctx context.Context, a, b ComplexNumber) (*ComplexOperationResult, error) {
	start := time.Now()

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	resultReal := a.Real + b.Real
	resultImag := a.Imaginary + b.Imaginary

	if op.overflowProtect {
		if math.IsInf(resultReal, 0) || math.IsInf(resultImag, 0) {
			return nil, fmt.Errorf("overflow detected in complex addition: (%v+%vi) + (%v+%vi)",
				a.Real, a.Imaginary, b.Real, b.Imaginary)
		}
		if math.IsNaN(resultReal) || math.IsNaN(resultImag) {
			return nil, fmt.Errorf("NaN detected in complex addition")
		}
	}

	output := NewComplexNumber(resultReal, resultImag)
	duration := time.Since(start)

	result := &ComplexOperationResult{
		Value:    output,
		Duration: duration,
		Strategy: "direct",
	}

	if op.auditEnabled {
		result.AuditTrail = []ComplexAuditEntry{{
			Timestamp: start,
			Operation: "complex_add",
			Input:     []ComplexNumber{a, b},
			Output:    output,
			Duration:  duration,
			Component: "ComplexAddOperation",
		}}
	}

	return result, nil
}

// ComplexMultiplyOperation implements enterprise-grade complex multiplication.
type ComplexMultiplyOperation struct {
	precision       int
	overflowProtect bool
	auditEnabled    bool
}

func NewComplexMultiplyOperation(precision int, overflowProtect, audit bool) *ComplexMultiplyOperation {
	return &ComplexMultiplyOperation{
		precision:       precision,
		overflowProtect: overflowProtect,
		auditEnabled:    audit,
	}
}

func (op *ComplexMultiplyOperation) Name() string        { return "complex_multiply" }
func (op *ComplexMultiplyOperation) Description() string  { return "Enterprise Complex Multiplication - (a+bi)(c+di) with full audit" }
func (op *ComplexMultiplyOperation) Category() OperationCategory { return CategoryArithmetic }

func (op *ComplexMultiplyOperation) Execute(ctx context.Context, a, b ComplexNumber) (*ComplexOperationResult, error) {
	start := time.Now()

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	// (a+bi)(c+di) = (ac-bd) + (ad+bc)i
	resultReal := a.Real*b.Real - a.Imaginary*b.Imaginary
	resultImag := a.Real*b.Imaginary + a.Imaginary*b.Real

	if op.overflowProtect {
		if math.IsInf(resultReal, 0) || math.IsInf(resultImag, 0) {
			return nil, fmt.Errorf("overflow in complex multiplication")
		}
	}

	output := NewComplexNumber(resultReal, resultImag)
	duration := time.Since(start)

	result := &ComplexOperationResult{
		Value:    output,
		Duration: duration,
		Strategy: "direct",
	}

	if op.auditEnabled {
		result.AuditTrail = []ComplexAuditEntry{{
			Timestamp: start,
			Operation: "complex_multiply",
			Input:     []ComplexNumber{a, b},
			Output:    output,
			Duration:  duration,
			Component: "ComplexMultiplyOperation",
		}}
	}

	return result, nil
}

// ComplexDivideOperation implements enterprise-grade complex division.
type ComplexDivideOperation struct {
	precision       int
	overflowProtect bool
	auditEnabled    bool
	zeroDivPolicy   ZeroDivisionPolicy
}

func NewComplexDivideOperation(precision int, overflowProtect, audit bool) *ComplexDivideOperation {
	return &ComplexDivideOperation{
		precision:       precision,
		overflowProtect: overflowProtect,
		auditEnabled:    audit,
		zeroDivPolicy:   ZeroDivError,
	}
}

func (op *ComplexDivideOperation) Name() string        { return "complex_divide" }
func (op *ComplexDivideOperation) Description() string  { return "Enterprise Complex Division - Division with zero-denominator policy" }
func (op *ComplexDivideOperation) Category() OperationCategory { return CategoryArithmetic }

func (op *ComplexDivideOperation) Execute(ctx context.Context, a, b ComplexNumber) (*ComplexOperationResult, error) {
	start := time.Now()

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	denominator := b.Real*b.Real + b.Imaginary*b.Imaginary
	if denominator == 0 {
		if op.zeroDivPolicy == ZeroDivError {
			return nil, fmt.Errorf("complex division by zero: (%v+%vi) / (0+0i)", a.Real, a.Imaginary)
		}
		return &ComplexOperationResult{
			Value:    NewComplexNumber(math.NaN(), math.NaN()),
			Duration: time.Since(start),
			Strategy: "zero-div-fallback",
		}, nil
	}

	// (a+bi)/(c+di) = ((ac+bd) + (bc-ad)i) / (c^2+d^2)
	resultReal := (a.Real*b.Real + a.Imaginary*b.Imaginary) / denominator
	resultImag := (a.Imaginary*b.Real - a.Real*b.Imaginary) / denominator

	if op.overflowProtect {
		if math.IsInf(resultReal, 0) || math.IsInf(resultImag, 0) {
			return nil, fmt.Errorf("overflow in complex division")
		}
	}

	output := NewComplexNumber(resultReal, resultImag)
	duration := time.Since(start)

	result := &ComplexOperationResult{
		Value:    output,
		Duration: duration,
		Strategy: "direct",
	}

	if op.auditEnabled {
		result.AuditTrail = []ComplexAuditEntry{{
			Timestamp: start,
			Operation: "complex_divide",
			Input:     []ComplexNumber{a, b},
			Output:    output,
			Duration:  duration,
			Component: "ComplexDivideOperation",
		}}
	}

	return result, nil
}

// ComplexPowerOperation computes z^n for complex z using De Moivre's theorem.
type ComplexPowerOperation struct {
	maxExponent     float64
	overflowProtect bool
	auditEnabled    bool
}

func NewComplexPowerOperation(maxExponent float64, overflowProtect, audit bool) *ComplexPowerOperation {
	return &ComplexPowerOperation{
		maxExponent:     maxExponent,
		overflowProtect: overflowProtect,
		auditEnabled:    audit,
	}
}

func (op *ComplexPowerOperation) Name() string        { return "complex_power" }
func (op *ComplexPowerOperation) Description() string  { return "Enterprise Complex Power - z^n via De Moivre's theorem" }
func (op *ComplexPowerOperation) Category() OperationCategory { return CategoryArithmetic }

func (op *ComplexPowerOperation) Execute(ctx context.Context, base ComplexNumber, exponent float64) (*ComplexOperationResult, error) {
	start := time.Now()

	if math.Abs(exponent) > op.maxExponent {
		return nil, fmt.Errorf("exponent %v exceeds maximum allowed %v", exponent, op.maxExponent)
	}

	z := base.ToGoComplex()
	result := cmplx.Pow(z, complex(exponent, 0))

	if op.overflowProtect {
		if cmplx.IsInf(result) || cmplx.IsNaN(result) {
			return nil, fmt.Errorf("overflow/NaN in complex power: (%v+%vi)^%v", base.Real, base.Imaginary, exponent)
		}
	}

	output := FromGoComplex(result)
	duration := time.Since(start)

	opResult := &ComplexOperationResult{
		Value:    output,
		Duration: duration,
		Strategy: "de-moivre",
	}

	if op.auditEnabled {
		opResult.AuditTrail = []ComplexAuditEntry{{
			Timestamp: start,
			Operation: "complex_power",
			Input:     []ComplexNumber{base},
			Output:    output,
			Duration:  duration,
			Component: "ComplexPowerOperation",
		}}
	}

	return opResult, nil
}

// ComplexSqrtOperation computes the principal square root of a complex number.
type ComplexSqrtOperation struct {
	auditEnabled bool
}

func NewComplexSqrtOperation(audit bool) *ComplexSqrtOperation {
	return &ComplexSqrtOperation{auditEnabled: audit}
}

func (op *ComplexSqrtOperation) Name() string        { return "complex_sqrt" }
func (op *ComplexSqrtOperation) Description() string  { return "Enterprise Complex Square Root - Principal root with branch cut handling" }
func (op *ComplexSqrtOperation) Category() OperationCategory { return CategoryArithmetic }

func (op *ComplexSqrtOperation) Execute(ctx context.Context, z ComplexNumber) (*ComplexOperationResult, error) {
	start := time.Now()

	result := cmplx.Sqrt(z.ToGoComplex())
	output := FromGoComplex(result)
	duration := time.Since(start)

	opResult := &ComplexOperationResult{
		Value:    output,
		Duration: duration,
		Strategy: "principal-branch",
	}

	if op.auditEnabled {
		opResult.AuditTrail = []ComplexAuditEntry{{
			Timestamp:  start,
			Operation:  "complex_sqrt",
			Input:      []ComplexNumber{z},
			Output:     output,
			Duration:   duration,
			Component:  "ComplexSqrtOperation",
		}}
	}

	return opResult, nil
}

// ComplexExpOperation computes e^z for complex z.
type ComplexExpOperation struct {
	overflowProtect bool
	auditEnabled    bool
}

func NewComplexExpOperation(overflowProtect, audit bool) *ComplexExpOperation {
	return &ComplexExpOperation{overflowProtect: overflowProtect, auditEnabled: audit}
}

func (op *ComplexExpOperation) Name() string        { return "complex_exp" }
func (op *ComplexExpOperation) Description() string  { return "Enterprise Complex Exponential - e^z with overflow guard" }
func (op *ComplexExpOperation) Category() OperationCategory { return CategoryArithmetic }

func (op *ComplexExpOperation) Execute(ctx context.Context, z ComplexNumber) (*ComplexOperationResult, error) {
	start := time.Now()

	result := cmplx.Exp(z.ToGoComplex())

	if op.overflowProtect && cmplx.IsInf(result) {
		return nil, fmt.Errorf("overflow in complex exp: e^(%v+%vi)", z.Real, z.Imaginary)
	}

	output := FromGoComplex(result)
	duration := time.Since(start)

	return &ComplexOperationResult{
		Value:    output,
		Duration: duration,
		Strategy: "euler",
	}, nil
}

// ComplexLogOperation computes the principal logarithm of a complex number.
type ComplexLogOperation struct {
	auditEnabled bool
}

func NewComplexLogOperation(audit bool) *ComplexLogOperation {
	return &ComplexLogOperation{auditEnabled: audit}
}

func (op *ComplexLogOperation) Name() string        { return "complex_log" }
func (op *ComplexLogOperation) Description() string  { return "Enterprise Complex Logarithm - Principal branch log(z)" }
func (op *ComplexLogOperation) Category() OperationCategory { return CategoryArithmetic }

func (op *ComplexLogOperation) Execute(ctx context.Context, z ComplexNumber) (*ComplexOperationResult, error) {
	start := time.Now()

	if z.Real == 0 && z.Imaginary == 0 {
		return nil, fmt.Errorf("complex logarithm of zero is undefined")
	}

	result := cmplx.Log(z.ToGoComplex())
	output := FromGoComplex(result)
	duration := time.Since(start)

	return &ComplexOperationResult{
		Value:    output,
		Duration: duration,
		Strategy: "principal-branch",
	}, nil
}

// FFTOperation implements the Fast Fourier Transform for enterprise signal processing.
type FFTOperation struct {
	maxSize      int
	auditEnabled bool
	mu           sync.Mutex
}

func NewFFTOperation(maxSize int, audit bool) *FFTOperation {
	return &FFTOperation{maxSize: maxSize, auditEnabled: audit}
}

func (op *FFTOperation) Name() string        { return "fft" }
func (op *FFTOperation) Description() string  { return "Enterprise FFT - Cooley-Tukey radix-2 with size validation" }
func (op *FFTOperation) Category() OperationCategory { return CategoryArithmetic }

// Execute performs an in-place FFT on the input signal.
func (op *FFTOperation) Execute(ctx context.Context, signal []ComplexNumber) ([]ComplexNumber, error) {
	n := len(signal)
	if n == 0 {
		return nil, fmt.Errorf("FFT requires non-empty input")
	}
	if n&(n-1) != 0 {
		return nil, fmt.Errorf("FFT input length must be a power of 2, got %d", n)
	}
	if n > op.maxSize {
		return nil, fmt.Errorf("FFT input length %d exceeds maximum %d", n, op.maxSize)
	}

	// Convert to complex128 slice
	data := make([]complex128, n)
	for i, c := range signal {
		data[i] = c.ToGoComplex()
	}

	// Bit-reversal permutation
	j := 0
	for i := 1; i < n; i++ {
		bit := n >> 1
		for ; j&bit != 0; bit >>= 1 {
			j ^= bit
		}
		j ^= bit
		if i < j {
			data[i], data[j] = data[j], data[i]
		}
	}

	// Cooley-Tukey iterative FFT
	for size := 2; size <= n; size <<= 1 {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		halfSize := size >> 1
		angle := -2.0 * math.Pi / float64(size)
		wn := cmplx.Rect(1, angle)

		for start := 0; start < n; start += size {
			w := complex(1, 0)
			for k := 0; k < halfSize; k++ {
				u := data[start+k]
				v := data[start+k+halfSize] * w
				data[start+k] = u + v
				data[start+k+halfSize] = u - v
				w *= wn
			}
		}
	}

	// Convert back to ComplexNumber slice
	result := make([]ComplexNumber, n)
	for i, z := range data {
		result[i] = FromGoComplex(z)
	}

	return result, nil
}

// InverseFFT computes the inverse FFT.
func (op *FFTOperation) InverseFFT(ctx context.Context, spectrum []ComplexNumber) ([]ComplexNumber, error) {
	n := len(spectrum)
	if n == 0 {
		return nil, fmt.Errorf("IFFT requires non-empty input")
	}

	// Conjugate the input
	conjugated := make([]ComplexNumber, n)
	for i, c := range spectrum {
		conjugated[i] = c.Conjugate()
	}

	// Forward FFT on conjugated signal
	result, err := op.Execute(ctx, conjugated)
	if err != nil {
		return nil, fmt.Errorf("IFFT failed: %w", err)
	}

	// Conjugate and scale by 1/N
	scale := 1.0 / float64(n)
	for i := range result {
		result[i] = NewComplexNumber(result[i].Real*scale, -result[i].Imaginary*scale)
	}

	return result, nil
}

// ComplexMatrixMultiply performs matrix multiplication on complex matrices.
// Matrices are represented as 2D slices.
type ComplexMatrixMultiply struct {
	maxDimension    int
	parallelThresh  int
	auditEnabled    bool
}

func NewComplexMatrixMultiply(maxDim, parallelThresh int, audit bool) *ComplexMatrixMultiply {
	return &ComplexMatrixMultiply{
		maxDimension:   maxDim,
		parallelThresh: parallelThresh,
		auditEnabled:   audit,
	}
}

func (op *ComplexMatrixMultiply) Name() string        { return "complex_matrix_multiply" }
func (op *ComplexMatrixMultiply) Description() string  { return "Enterprise Complex Matrix Multiplication - With parallel row computation" }
func (op *ComplexMatrixMultiply) Category() OperationCategory { return CategoryArithmetic }

func (op *ComplexMatrixMultiply) Execute(ctx context.Context, a, b [][]ComplexNumber) ([][]ComplexNumber, error) {
	if len(a) == 0 || len(b) == 0 {
		return nil, fmt.Errorf("matrices must be non-empty")
	}

	rowsA, colsA := len(a), len(a[0])
	rowsB, colsB := len(b), len(b[0])

	if colsA != rowsB {
		return nil, fmt.Errorf("dimension mismatch: %dx%d * %dx%d", rowsA, colsA, rowsB, colsB)
	}

	if rowsA > op.maxDimension || colsB > op.maxDimension {
		return nil, fmt.Errorf("matrix dimensions exceed maximum %d", op.maxDimension)
	}

	result := make([][]ComplexNumber, rowsA)
	for i := range result {
		result[i] = make([]ComplexNumber, colsB)
	}

	multiply := func(i int) {
		for j := 0; j < colsB; j++ {
			sumReal := 0.0
			sumImag := 0.0
			for k := 0; k < colsA; k++ {
				// (a+bi)(c+di) = (ac-bd) + (ad+bc)i
				ar, ai := a[i][k].Real, a[i][k].Imaginary
				br, bi := b[k][j].Real, b[k][j].Imaginary
				sumReal += ar*br - ai*bi
				sumImag += ar*bi + ai*br
			}
			result[i][j] = NewComplexNumber(sumReal, sumImag)
		}
	}

	if rowsA >= op.parallelThresh {
		var wg sync.WaitGroup
		wg.Add(rowsA)
		for i := 0; i < rowsA; i++ {
			go func(row int) {
				defer wg.Done()
				multiply(row)
			}(i)
		}
		wg.Wait()
	} else {
		for i := 0; i < rowsA; i++ {
			multiply(i)
		}
	}

	return result, nil
}

// ComplexEigenvalue approximates eigenvalues using the QR algorithm for small matrices.
type ComplexEigenvalue struct {
	maxIterations int
	tolerance     float64
	auditEnabled  bool
}

func NewComplexEigenvalue(maxIter int, tol float64, audit bool) *ComplexEigenvalue {
	return &ComplexEigenvalue{
		maxIterations: maxIter,
		tolerance:     tol,
		auditEnabled:  audit,
	}
}

func (op *ComplexEigenvalue) Name() string        { return "complex_eigenvalue" }
func (op *ComplexEigenvalue) Description() string  { return "Enterprise Eigenvalue Approximation - QR iteration for small matrices" }
func (op *ComplexEigenvalue) Category() OperationCategory { return CategoryArithmetic }

// Execute computes eigenvalues for a 2x2 complex matrix using the quadratic formula.
// For larger matrices, use the full QR algorithm (not yet enterprise-certified).
func (op *ComplexEigenvalue) Execute(ctx context.Context, matrix [][]ComplexNumber) ([]ComplexNumber, error) {
	n := len(matrix)
	if n == 0 {
		return nil, fmt.Errorf("matrix must be non-empty")
	}
	for _, row := range matrix {
		if len(row) != n {
			return nil, fmt.Errorf("matrix must be square")
		}
	}

	if n > 4 {
		return nil, fmt.Errorf("eigenvalue computation only supported for matrices up to 4x4, got %dx%d", n, n)
	}

	// For 2x2: eigenvalues from quadratic formula
	// det(A - λI) = 0
	// λ^2 - tr(A)λ + det(A) = 0
	if n == 2 {
		a, b := matrix[0][0].ToGoComplex(), matrix[0][1].ToGoComplex()
		c, d := matrix[1][0].ToGoComplex(), matrix[1][1].ToGoComplex()

		trace := a + d
		det := a*d - b*c
		discriminant := trace*trace - 4*det

		sqrtDisc := cmplx.Sqrt(discriminant)
		lambda1 := (trace + sqrtDisc) / 2
		lambda2 := (trace - sqrtDisc) / 2

		return []ComplexNumber{
			FromGoComplex(lambda1),
			FromGoComplex(lambda2),
		}, nil
	}

	return nil, fmt.Errorf("eigenvalue computation for %dx%d not implemented yet", n, n)
}

// NewtonRaphsonComplex implements Newton-Raphson root finding in the complex plane.
type NewtonRaphsonComplex struct {
	maxIterations int
	tolerance     float64
	auditEnabled  bool
}

func NewNewtonRaphsonComplex(maxIter int, tol float64, audit bool) *NewtonRaphsonComplex {
	return &NewtonRaphsonComplex{
		maxIterations: maxIter,
		tolerance:     tol,
		auditEnabled:  audit,
	}
}

func (op *NewtonRaphsonComplex) Name() string        { return "newton_raphson_complex" }
func (op *NewtonRaphsonComplex) Description() string  { return "Enterprise Newton-Raphson - Root finding in the complex plane" }
func (op *NewtonRaphsonComplex) Category() OperationCategory { return CategoryArithmetic }

// FindRoot finds a root of f(z) starting from initial guess z0.
// f and fPrime are the function and its derivative.
func (op *NewtonRaphsonComplex) FindRoot(
	ctx context.Context,
	z0 ComplexNumber,
	f func(complex128) complex128,
	fPrime func(complex128) complex128,
) (*ComplexOperationResult, error) {
	start := time.Now()

	z := z0.ToGoComplex()
	var iterations int

	for i := 0; i < op.maxIterations; i++ {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		fz := f(z)
		fpz := fPrime(z)

		if cmplx.Abs(fpz) < 1e-15 {
			return nil, fmt.Errorf("derivative too small at iteration %d, z = %v", i, z)
		}

		zNew := z - fz/fpz
		iterations = i + 1

		if cmplx.Abs(zNew-z) < op.tolerance {
			output := FromGoComplex(zNew)
			duration := time.Since(start)
			result := &ComplexOperationResult{
				Value:    output,
				Duration: duration,
				Strategy: "newton-raphson",
			}
			if op.auditEnabled {
				result.AuditTrail = []ComplexAuditEntry{{
					Timestamp:  start,
					Operation:  "newton_raphson",
					Input:      []ComplexNumber{z0},
					Output:     output,
					Duration:   duration,
					Component:  "NewtonRaphsonComplex",
					Iterations: iterations,
				}}
			}
			return result, nil
		}

		z = zNew
	}

	return nil, fmt.Errorf("Newton-Raphson did not converge after %d iterations (last z = %v)", op.maxIterations, z)
}

// MandelbrotChecker determines if a complex number is in the Mandelbrot set.
type MandelbrotChecker struct {
	maxIterations int
	escapeRadius  float64
}

func NewMandelbrotChecker(maxIter int, escapeRadius float64) *MandelbrotChecker {
	return &MandelbrotChecker{maxIterations: maxIter, escapeRadius: escapeRadius}
}

func (op *MandelbrotChecker) Name() string        { return "mandelbrot_check" }
func (op *MandelbrotChecker) Description() string  { return "Enterprise Mandelbrot - Set membership with escape iteration count" }

// Check returns the number of iterations before escape, or -1 if the point is in the set.
func (op *MandelbrotChecker) Check(c ComplexNumber) (int, float64) {
	z := complex(0, 0)
	cc := c.ToGoComplex()
	escSq := op.escapeRadius * op.escapeRadius

	for i := 0; i < op.maxIterations; i++ {
		z = z*z + cc
		mag := real(z)*real(z) + imag(z)*imag(z)
		if mag > escSq {
			// Smooth iteration count for continuous coloring
			smoothed := float64(i) + 1 - math.Log(math.Log(math.Sqrt(mag)))/math.Log(2)
			return i, smoothed
		}
	}

	return -1, float64(op.maxIterations)
}

// GenerateSet produces a 2D iteration-count grid for the Mandelbrot set.
func (op *MandelbrotChecker) GenerateSet(ctx context.Context, centerReal, centerImag, zoom float64, width, height int) ([][]int, error) {
	if width <= 0 || height <= 0 {
		return nil, fmt.Errorf("dimensions must be positive: %dx%d", width, height)
	}
	if width > 4096 || height > 4096 {
		return nil, fmt.Errorf("dimensions exceed enterprise limit of 4096x4096")
	}

	grid := make([][]int, height)
	scale := 4.0 / (zoom * float64(width))

	var wg sync.WaitGroup
	for row := 0; row < height; row++ {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		grid[row] = make([]int, width)
		wg.Add(1)
		go func(r int) {
			defer wg.Done()
			imag := centerImag + (float64(r)-float64(height)/2)*scale
			for col := 0; col < width; col++ {
				re := centerReal + (float64(col)-float64(width)/2)*scale
				c := NewComplexNumber(re, imag)
				iters, _ := op.Check(c)
				grid[r][col] = iters
			}
		}(row)
	}
	wg.Wait()

	return grid, nil
}
