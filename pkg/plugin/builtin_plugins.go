package plugin

import (
	"context"
	"fmt"
	"math"
	"time"

	"github.com/rishabhsankar/enterprise-math/pkg/operations"
)

// FinancialPlugin provides financial math operations.
type FinancialPlugin struct{}

func NewFinancialPlugin() *FinancialPlugin { return &FinancialPlugin{} }

func (p *FinancialPlugin) Name() string        { return "financial" }
func (p *FinancialPlugin) Version() string     { return "1.0.0" }
func (p *FinancialPlugin) Description() string { return "Financial mathematics operations" }
func (p *FinancialPlugin) Dependencies() []string { return nil }
func (p *FinancialPlugin) Initialize(_ *PluginRegistry) error { return nil }
func (p *FinancialPlugin) Shutdown() error     { return nil }
func (p *FinancialPlugin) Middleware() []operations.OperationMiddleware { return nil }

func (p *FinancialPlugin) Operations() []operations.Operation {
	return []operations.Operation{
		&CompoundInterestOperation{},
		&PresentValueOperation{},
		&FutureValueOperation{},
		&IRROperation{},
		&NPVOperation{},
	}
}

// CompoundInterestOperation calculates compound interest.
type CompoundInterestOperation struct{}

func (op *CompoundInterestOperation) Name() string                  { return "compound_interest" }
func (op *CompoundInterestOperation) Description() string           { return "Enterprise Compound Interest™ - A = P(1+r/n)^(nt)" }
func (op *CompoundInterestOperation) Category() operations.OperationCategory { return operations.CategoryCustom }
func (op *CompoundInterestOperation) Arity() int                    { return 4 }

func (op *CompoundInterestOperation) Validate(args ...operations.Number) error {
	if len(args) != 4 {
		return fmt.Errorf("compound_interest requires 4 args (principal, rate, compounds_per_year, years), got %d", len(args))
	}
	if args[0].Value < 0 {
		return fmt.Errorf("principal must be non-negative")
	}
	if args[2].Value <= 0 {
		return fmt.Errorf("compounds per year must be positive")
	}
	return nil
}

func (op *CompoundInterestOperation) Execute(ctx context.Context, args ...operations.Number) (*operations.OperationResult, error) {
	if err := op.Validate(args...); err != nil {
		return nil, err
	}
	start := time.Now()
	principal := args[0].Value
	rate := args[1].Value
	n := args[2].Value
	t := args[3].Value
	amount := principal * math.Pow(1+rate/n, n*t)
	return &operations.OperationResult{
		Value:    operations.NewNumberWithUnit(amount, "currency"),
		Duration: time.Since(start),
		Strategy: "direct",
	}, nil
}

// PresentValueOperation calculates present value of a future sum.
type PresentValueOperation struct{}

func (op *PresentValueOperation) Name() string                  { return "present_value" }
func (op *PresentValueOperation) Description() string           { return "Enterprise Present Value™ - PV = FV / (1+r)^n" }
func (op *PresentValueOperation) Category() operations.OperationCategory { return operations.CategoryCustom }
func (op *PresentValueOperation) Arity() int                    { return 3 }

func (op *PresentValueOperation) Validate(args ...operations.Number) error {
	if len(args) != 3 {
		return fmt.Errorf("present_value requires 3 args (future_value, rate, periods), got %d", len(args))
	}
	return nil
}

func (op *PresentValueOperation) Execute(ctx context.Context, args ...operations.Number) (*operations.OperationResult, error) {
	if err := op.Validate(args...); err != nil {
		return nil, err
	}
	start := time.Now()
	fv := args[0].Value
	rate := args[1].Value
	periods := args[2].Value
	pv := fv / math.Pow(1+rate, periods)
	return &operations.OperationResult{
		Value:    operations.NewNumberWithUnit(pv, "currency"),
		Duration: time.Since(start),
		Strategy: "direct",
	}, nil
}

// FutureValueOperation calculates future value of a present sum.
type FutureValueOperation struct{}

func (op *FutureValueOperation) Name() string                  { return "future_value" }
func (op *FutureValueOperation) Description() string           { return "Enterprise Future Value™ - FV = PV * (1+r)^n" }
func (op *FutureValueOperation) Category() operations.OperationCategory { return operations.CategoryCustom }
func (op *FutureValueOperation) Arity() int                    { return 3 }

func (op *FutureValueOperation) Validate(args ...operations.Number) error {
	if len(args) != 3 {
		return fmt.Errorf("future_value requires 3 args (present_value, rate, periods), got %d", len(args))
	}
	return nil
}

func (op *FutureValueOperation) Execute(ctx context.Context, args ...operations.Number) (*operations.OperationResult, error) {
	if err := op.Validate(args...); err != nil {
		return nil, err
	}
	start := time.Now()
	pv := args[0].Value
	rate := args[1].Value
	periods := args[2].Value
	fv := pv * math.Pow(1+rate, periods)
	return &operations.OperationResult{
		Value:    operations.NewNumberWithUnit(fv, "currency"),
		Duration: time.Since(start),
		Strategy: "direct",
	}, nil
}

// IRROperation calculates internal rate of return using Newton-Raphson.
type IRROperation struct{}

func (op *IRROperation) Name() string                  { return "irr" }
func (op *IRROperation) Description() string           { return "Enterprise IRR™ - Newton-Raphson internal rate of return" }
func (op *IRROperation) Category() operations.OperationCategory { return operations.CategoryCustom }
func (op *IRROperation) Arity() int                    { return -1 }

func (op *IRROperation) Validate(args ...operations.Number) error {
	if len(args) < 2 {
		return fmt.Errorf("IRR requires at least 2 cash flows, got %d", len(args))
	}
	return nil
}

func (op *IRROperation) Execute(ctx context.Context, args ...operations.Number) (*operations.OperationResult, error) {
	if err := op.Validate(args...); err != nil {
		return nil, err
	}
	start := time.Now()

	cashFlows := make([]float64, len(args))
	for i, a := range args {
		cashFlows[i] = a.Value
	}

	// Newton-Raphson method
	rate := 0.1
	for iter := 0; iter < 1000; iter++ {
		npv := 0.0
		dnpv := 0.0
		for i, cf := range cashFlows {
			t := float64(i)
			npv += cf / math.Pow(1+rate, t)
			dnpv -= t * cf / math.Pow(1+rate, t+1)
		}

		if math.Abs(npv) < 1e-10 {
			break
		}
		if dnpv == 0 {
			break
		}
		rate -= npv / dnpv
	}

	return &operations.OperationResult{
		Value:    operations.NewNumberWithUnit(rate, "rate"),
		Duration: time.Since(start),
		Strategy: "newton-raphson",
	}, nil
}

// NPVOperation calculates net present value.
type NPVOperation struct{}

func (op *NPVOperation) Name() string                  { return "npv" }
func (op *NPVOperation) Description() string           { return "Enterprise NPV™ - Net present value of cash flows" }
func (op *NPVOperation) Category() operations.OperationCategory { return operations.CategoryCustom }
func (op *NPVOperation) Arity() int                    { return -1 }

func (op *NPVOperation) Validate(args ...operations.Number) error {
	if len(args) < 2 {
		return fmt.Errorf("NPV requires discount rate + at least 1 cash flow, got %d args", len(args))
	}
	return nil
}

func (op *NPVOperation) Execute(ctx context.Context, args ...operations.Number) (*operations.OperationResult, error) {
	if err := op.Validate(args...); err != nil {
		return nil, err
	}
	start := time.Now()

	rate := args[0].Value
	npv := 0.0
	for i := 1; i < len(args); i++ {
		t := float64(i - 1)
		npv += args[i].Value / math.Pow(1+rate, t)
	}

	return &operations.OperationResult{
		Value:    operations.NewNumberWithUnit(npv, "currency"),
		Duration: time.Since(start),
		Strategy: "direct",
	}, nil
}

// LinearAlgebraPlugin provides vector and matrix operations.
type LinearAlgebraPlugin struct{}

func NewLinearAlgebraPlugin() *LinearAlgebraPlugin { return &LinearAlgebraPlugin{} }

func (p *LinearAlgebraPlugin) Name() string        { return "linear_algebra" }
func (p *LinearAlgebraPlugin) Version() string     { return "1.0.0" }
func (p *LinearAlgebraPlugin) Description() string { return "Linear algebra operations" }
func (p *LinearAlgebraPlugin) Dependencies() []string { return nil }
func (p *LinearAlgebraPlugin) Initialize(_ *PluginRegistry) error { return nil }
func (p *LinearAlgebraPlugin) Shutdown() error     { return nil }
func (p *LinearAlgebraPlugin) Middleware() []operations.OperationMiddleware { return nil }

func (p *LinearAlgebraPlugin) Operations() []operations.Operation {
	return []operations.Operation{
		&DotProductOperation{},
		&MagnitudeOperation{},
		&NormalizeOperation{},
	}
}

// DotProductOperation computes the dot product of two vectors.
type DotProductOperation struct{}

func (op *DotProductOperation) Name() string                  { return "dot_product" }
func (op *DotProductOperation) Description() string           { return "Enterprise Dot Product™ - Vector inner product" }
func (op *DotProductOperation) Category() operations.OperationCategory { return operations.CategoryCustom }
func (op *DotProductOperation) Arity() int                    { return -1 }

func (op *DotProductOperation) Validate(args ...operations.Number) error {
	if len(args)%2 != 0 {
		return fmt.Errorf("dot product requires even number of args (paired vectors), got %d", len(args))
	}
	if len(args) < 2 {
		return fmt.Errorf("dot product requires at least 2 elements")
	}
	return nil
}

func (op *DotProductOperation) Execute(ctx context.Context, args ...operations.Number) (*operations.OperationResult, error) {
	if err := op.Validate(args...); err != nil {
		return nil, err
	}
	start := time.Now()
	n := len(args) / 2
	result := 0.0
	for i := 0; i < n; i++ {
		result += args[i].Value * args[n+i].Value
	}
	return &operations.OperationResult{
		Value:    operations.NewNumber(result),
		Duration: time.Since(start),
		Strategy: "direct",
	}, nil
}

// MagnitudeOperation computes the magnitude (L2 norm) of a vector.
type MagnitudeOperation struct{}

func (op *MagnitudeOperation) Name() string                  { return "magnitude" }
func (op *MagnitudeOperation) Description() string           { return "Enterprise Magnitude™ - L2 vector norm" }
func (op *MagnitudeOperation) Category() operations.OperationCategory { return operations.CategoryCustom }
func (op *MagnitudeOperation) Arity() int                    { return -1 }

func (op *MagnitudeOperation) Validate(args ...operations.Number) error {
	if len(args) == 0 {
		return fmt.Errorf("magnitude requires at least 1 element")
	}
	return nil
}

func (op *MagnitudeOperation) Execute(ctx context.Context, args ...operations.Number) (*operations.OperationResult, error) {
	if err := op.Validate(args...); err != nil {
		return nil, err
	}
	start := time.Now()
	sumSq := 0.0
	for _, a := range args {
		sumSq += a.Value * a.Value
	}
	return &operations.OperationResult{
		Value:    operations.NewNumber(math.Sqrt(sumSq)),
		Duration: time.Since(start),
		Strategy: "direct",
	}, nil
}

// NormalizeOperation normalizes a vector to unit length.
type NormalizeOperation struct{}

func (op *NormalizeOperation) Name() string                  { return "normalize" }
func (op *NormalizeOperation) Description() string           { return "Enterprise Normalize™ - Unit vector normalization" }
func (op *NormalizeOperation) Category() operations.OperationCategory { return operations.CategoryCustom }
func (op *NormalizeOperation) Arity() int                    { return -1 }

func (op *NormalizeOperation) Validate(args ...operations.Number) error {
	if len(args) == 0 {
		return fmt.Errorf("normalize requires at least 1 element")
	}
	return nil
}

func (op *NormalizeOperation) Execute(ctx context.Context, args ...operations.Number) (*operations.OperationResult, error) {
	if err := op.Validate(args...); err != nil {
		return nil, err
	}
	start := time.Now()
	sumSq := 0.0
	for _, a := range args {
		sumSq += a.Value * a.Value
	}
	mag := math.Sqrt(sumSq)
	if mag == 0 {
		return nil, fmt.Errorf("cannot normalize zero vector")
	}
	// Return the first normalized component as representative
	return &operations.OperationResult{
		Value:    operations.NewNumber(args[0].Value / mag),
		Duration: time.Since(start),
		Strategy: "direct",
	}, nil
}
