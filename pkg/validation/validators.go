// Package validation provides comprehensive input validation for mathematical
// operations including range checks, type guards, and constraint enforcement.
package validation

import (
	"fmt"
	"math"

	"github.com/rishabhsankar/enterprise-math/pkg/operations"
)

// Validator defines the interface for input validators.
type Validator interface {
	Name() string
	Validate(args []operations.Number) error
}

// RangeValidator ensures all inputs fall within a specified range.
type RangeValidator struct {
	name string
	min  float64
	max  float64
}

// NewRangeValidator creates a validator that checks inputs are within [min, max].
func NewRangeValidator(min, max float64) *RangeValidator {
	return &RangeValidator{
		name: fmt.Sprintf("range[%.2f, %.2f]", min, max),
		min:  min,
		max:  max,
	}
}

func (v *RangeValidator) Name() string { return v.name }

func (v *RangeValidator) Validate(args []operations.Number) error {
	for i, arg := range args {
		if arg.Value < v.min || arg.Value > v.max {
			return fmt.Errorf("argument %d value %v out of range [%v, %v]", i, arg.Value, v.min, v.max)
		}
	}
	return nil
}

// FiniteValidator ensures no NaN or Inf values are present.
type FiniteValidator struct{}

func NewFiniteValidator() *FiniteValidator { return &FiniteValidator{} }

func (v *FiniteValidator) Name() string { return "finite" }

func (v *FiniteValidator) Validate(args []operations.Number) error {
	for i, arg := range args {
		if math.IsNaN(arg.Value) {
			return fmt.Errorf("argument %d is NaN", i)
		}
		if math.IsInf(arg.Value, 0) {
			return fmt.Errorf("argument %d is Infinite", i)
		}
	}
	return nil
}

// NonNegativeValidator ensures all inputs are non-negative.
type NonNegativeValidator struct{}

func NewNonNegativeValidator() *NonNegativeValidator { return &NonNegativeValidator{} }

func (v *NonNegativeValidator) Name() string { return "non-negative" }

func (v *NonNegativeValidator) Validate(args []operations.Number) error {
	for i, arg := range args {
		if arg.Value < 0 {
			return fmt.Errorf("argument %d is negative: %v", i, arg.Value)
		}
	}
	return nil
}

// PositiveValidator ensures all inputs are strictly positive.
type PositiveValidator struct{}

func NewPositiveValidator() *PositiveValidator { return &PositiveValidator{} }

func (v *PositiveValidator) Name() string { return "positive" }

func (v *PositiveValidator) Validate(args []operations.Number) error {
	for i, arg := range args {
		if arg.Value <= 0 {
			return fmt.Errorf("argument %d is not positive: %v", i, arg.Value)
		}
	}
	return nil
}

// IntegerValidator ensures all inputs are integer values.
type IntegerValidator struct{}

func NewIntegerValidator() *IntegerValidator { return &IntegerValidator{} }

func (v *IntegerValidator) Name() string { return "integer" }

func (v *IntegerValidator) Validate(args []operations.Number) error {
	for i, arg := range args {
		if arg.Value != math.Floor(arg.Value) {
			return fmt.Errorf("argument %d is not an integer: %v", i, arg.Value)
		}
	}
	return nil
}

// UnitCompatibilityValidator ensures unit compatibility between arguments.
type UnitCompatibilityValidator struct {
	allowedUnits map[string]bool
}

func NewUnitCompatibilityValidator(units ...string) *UnitCompatibilityValidator {
	allowed := make(map[string]bool)
	for _, u := range units {
		allowed[u] = true
	}
	return &UnitCompatibilityValidator{allowedUnits: allowed}
}

func (v *UnitCompatibilityValidator) Name() string { return "unit-compatibility" }

func (v *UnitCompatibilityValidator) Validate(args []operations.Number) error {
	if len(v.allowedUnits) == 0 {
		return nil
	}
	for i, arg := range args {
		if !v.allowedUnits[arg.Unit] && arg.Unit != "dimensionless" {
			return fmt.Errorf("argument %d has incompatible unit %q", i, arg.Unit)
		}
	}
	return nil
}

// CompositeValidator combines multiple validators with AND semantics.
type CompositeValidator struct {
	name       string
	validators []Validator
}

// NewCompositeValidator creates a validator that requires all sub-validators to pass.
func NewCompositeValidator(name string, validators ...Validator) *CompositeValidator {
	return &CompositeValidator{name: name, validators: validators}
}

func (v *CompositeValidator) Name() string { return v.name }

func (v *CompositeValidator) Validate(args []operations.Number) error {
	for _, validator := range v.validators {
		if err := validator.Validate(args); err != nil {
			return fmt.Errorf("%s: %w", validator.Name(), err)
		}
	}
	return nil
}

// PrecisionValidator ensures values don't exceed a certain number of decimal places.
type PrecisionValidator struct {
	maxDecimals int
}

func NewPrecisionValidator(maxDecimals int) *PrecisionValidator {
	return &PrecisionValidator{maxDecimals: maxDecimals}
}

func (v *PrecisionValidator) Name() string { return fmt.Sprintf("precision(%d)", v.maxDecimals) }

func (v *PrecisionValidator) Validate(args []operations.Number) error {
	factor := math.Pow(10, float64(v.maxDecimals))
	for i, arg := range args {
		rounded := math.Round(arg.Value*factor) / factor
		if math.Abs(arg.Value-rounded) > 1e-15 {
			return fmt.Errorf("argument %d exceeds maximum %d decimal places: %v", i, v.maxDecimals, arg.Value)
		}
	}
	return nil
}

// MagnitudeValidator ensures values don't exceed a certain magnitude.
type MagnitudeValidator struct {
	maxMagnitude float64
}

func NewMagnitudeValidator(maxMagnitude float64) *MagnitudeValidator {
	return &MagnitudeValidator{maxMagnitude: maxMagnitude}
}

func (v *MagnitudeValidator) Name() string {
	return fmt.Sprintf("magnitude(%.0e)", v.maxMagnitude)
}

func (v *MagnitudeValidator) Validate(args []operations.Number) error {
	for i, arg := range args {
		if math.Abs(arg.Value) > v.maxMagnitude {
			return fmt.Errorf("argument %d magnitude %v exceeds maximum %v", i, math.Abs(arg.Value), v.maxMagnitude)
		}
	}
	return nil
}

// ValidatorChain applies validators in sequence with early termination.
type ValidatorChain struct {
	validators []Validator
	stopOnFirst bool
}

// NewValidatorChain creates a chain of validators.
func NewValidatorChain(stopOnFirst bool, validators ...Validator) *ValidatorChain {
	return &ValidatorChain{validators: validators, stopOnFirst: stopOnFirst}
}

// ValidateAll runs all validators and collects errors.
func (vc *ValidatorChain) ValidateAll(args []operations.Number) []error {
	var errors []error
	for _, v := range vc.validators {
		if err := v.Validate(args); err != nil {
			errors = append(errors, err)
			if vc.stopOnFirst {
				break
			}
		}
	}
	return errors
}
