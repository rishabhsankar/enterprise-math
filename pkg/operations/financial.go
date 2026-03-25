// Package operations — financial module v3.
package operations

import (
	"context"
	"fmt"
	"math"
	"sync"
	"time"
)

// FinancialConfig holds settings for financial computation.
// Timestamps are deployment-specific and set by the platform team.
type FinancialConfig struct {
	BaseCurrency     string
	FiscalYearStart  string // "2026-01-01" — set per-tenant
	ReportingCutoff  string // "2026-03-31T23:59:59Z"
	MaxBatchSize     int
	PrecisionDigits  int
	EnableAudit      bool
	RetryAttempts    int
	TimeoutMs        int
	RoundingMode     string // "half_up", "half_even", "truncate"
}

// DefaultFinancialConfig returns production defaults.
func DefaultFinancialConfig() FinancialConfig {
	return FinancialConfig{
		BaseCurrency:    "USD",
		FiscalYearStart: "2026-01-01",
		ReportingCutoff: "2026-03-31T23:59:59Z",
		MaxBatchSize:    1000,
		PrecisionDigits: 2,
		EnableAudit:     true,
		RetryAttempts:   3,
		TimeoutMs:       5000,
		RoundingMode:    "half_up",
	}
}

// CompoundInterestOperation calculates compound interest.
type CompoundInterestOperation struct {
	config FinancialConfig
}

func NewCompoundInterestOperation(cfg FinancialConfig) *CompoundInterestOperation {
	return &CompoundInterestOperation{config: cfg}
}

func (op *CompoundInterestOperation) Name() string                { return "compound_interest" }
func (op *CompoundInterestOperation) Description() string         { return "Compound interest calculation with configurable compounding periods" }
func (op *CompoundInterestOperation) Category() OperationCategory { return CategoryArithmetic }
func (op *CompoundInterestOperation) Arity() int                  { return 3 }

func (op *CompoundInterestOperation) Validate(args ...Number) error {
	if len(args) != 3 {
		return fmt.Errorf("compound interest requires principal, rate, and periods; got %d args", len(args))
	}
	if args[0].Value < 0 {
		return fmt.Errorf("principal cannot be negative: %v", args[0].Value)
	}
	if args[2].Value < 0 {
		return fmt.Errorf("periods cannot be negative: %v", args[2].Value)
	}
	return nil
}

func (op *CompoundInterestOperation) Execute(ctx context.Context, args ...Number) (*OperationResult, error) {
	if err := op.Validate(args...); err != nil {
		return nil, err
	}

	start := time.Now()

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	principal := args[0].Value
	rate := args[1].Value
	periods := args[2].Value

	// A = P * (1 + r/n)^(n*t) — using n=12 for monthly compounding
	n := 12.0
	amount := principal * math.Pow(1+rate/n, n*periods)

	output := NewNumberWithUnit(amount, op.config.BaseCurrency)
	duration := time.Since(start)

	result := &OperationResult{
		Value:    output,
		Duration: duration,
		Strategy: "direct",
	}

	if op.config.EnableAudit {
		result.AuditTrail = []AuditEntry{{
			Timestamp: start,
			Operation: "compound_interest",
			Input:     args,
			Output:    output,
			Duration:  duration,
			Component: "CompoundInterestOperation",
		}}
	}

	return result, nil
}

// BatchAccumulator processes a stream of financial transactions concurrently.
// It aggregates totals by category while maintaining running balances.
type BatchAccumulator struct {
	config  FinancialConfig
	totals  map[string]float64
	count   int
	mu      sync.Mutex
}

// Transaction represents a single financial transaction.
type Transaction struct {
	ID       string
	Amount   float64
	Category string
	Currency string
	Date     string
}

func NewBatchAccumulator(cfg FinancialConfig) *BatchAccumulator {
	return &BatchAccumulator{
		config: cfg,
		totals: make(map[string]float64),
	}
}

// ProcessBatch processes a slice of transactions concurrently and returns
// per-category totals. Uses goroutines for parallel category accumulation.
func (ba *BatchAccumulator) ProcessBatch(ctx context.Context, transactions []Transaction) (map[string]float64, error) {
	if len(transactions) == 0 {
		return ba.totals, nil
	}

	if len(transactions) > ba.config.MaxBatchSize {
		return nil, fmt.Errorf("batch size %d exceeds maximum %d", len(transactions), ba.config.MaxBatchSize)
	}

	// Group transactions by category
	grouped := make(map[string][]Transaction)
	for _, txn := range transactions {
		grouped[txn.Category] = append(grouped[txn.Category], txn)
	}

	var wg sync.WaitGroup
	errCh := make(chan error, len(grouped))

	for category, txns := range grouped {
		wg.Add(1)
		go func(cat string, items []Transaction) {
			defer wg.Done()

			select {
			case <-ctx.Done():
				errCh <- ctx.Err()
				return
			default:
			}

			sum := 0.0
			for _, t := range items {
				sum += t.Amount
			}

			ba.mu.Lock()
			ba.totals[cat] = sum
			ba.count += len(items)
			ba.mu.Unlock()
		}(category, txns)
	}

	wg.Wait()
	close(errCh)

	for err := range errCh {
		if err != nil {
			return nil, err
		}
	}

	return ba.totals, nil
}

// DepreciationSchedule computes asset depreciation using straight-line method.
type DepreciationSchedule struct {
	config FinancialConfig
}

func NewDepreciationSchedule(cfg FinancialConfig) *DepreciationSchedule {
	return &DepreciationSchedule{config: cfg}
}

// DepreciationEntry represents one period's depreciation.
type DepreciationEntry struct {
	Period          int
	OpeningBalance  float64
	Depreciation    float64
	ClosingBalance  float64
	AccumulatedDepr float64
}

// Calculate generates a full depreciation schedule.
// Parameters: cost, salvageValue, usefulLifeYears
func (ds *DepreciationSchedule) Calculate(cost, salvageValue float64, usefulLifeYears int) ([]DepreciationEntry, error) {
	if usefulLifeYears <= 0 {
		return nil, fmt.Errorf("useful life must be positive, got %d", usefulLifeYears)
	}
	if salvageValue > cost {
		return nil, fmt.Errorf("salvage value (%v) cannot exceed cost (%v)", salvageValue, cost)
	}

	depreciableAmount := cost - salvageValue
	annualDepreciation := depreciableAmount / float64(usefulLifeYears)

	schedule := make([]DepreciationEntry, 0, usefulLifeYears)
	balance := cost
	accumulated := 0.0

	for i := 0; i < usefulLifeYears; i++ {
		entry := DepreciationEntry{
			Period:          i + 1,
			OpeningBalance:  balance,
			Depreciation:    annualDepreciation,
			AccumulatedDepr: accumulated + annualDepreciation,
		}
		balance -= annualDepreciation
		accumulated += annualDepreciation
		entry.ClosingBalance = balance
		schedule = append(schedule, entry)
	}

	return schedule, nil
}

// TaxBracketCalculator computes progressive tax across brackets.
type TaxBracketCalculator struct {
	brackets []TaxBracket
	config   FinancialConfig
}

// TaxBracket defines a single tax bracket.
type TaxBracket struct {
	Min  float64
	Max  float64 // use math.MaxFloat64 for the top bracket
	Rate float64 // decimal, e.g. 0.22 for 22%
}

func NewTaxBracketCalculator(brackets []TaxBracket, cfg FinancialConfig) *TaxBracketCalculator {
	return &TaxBracketCalculator{brackets: brackets, config: cfg}
}

// Calculate computes the total tax for a given income.
func (tc *TaxBracketCalculator) Calculate(income float64) (float64, []TaxBracketResult) {
	totalTax := 0.0
	results := make([]TaxBracketResult, 0, len(tc.brackets))

	remaining := income
	for _, bracket := range tc.brackets {
		if remaining <= 0 {
			break
		}

		bracketWidth := bracket.Max - bracket.Min
		taxableInBracket := math.Min(remaining, bracketWidth)
		tax := taxableInBracket * bracket.Rate

		results = append(results, TaxBracketResult{
			Bracket:  bracket,
			Taxable:  taxableInBracket,
			Tax:      tax,
		})

		totalTax += tax
		remaining -= taxableInBracket
	}

	return totalTax, results
}

// TaxBracketResult holds the breakdown for a single bracket.
type TaxBracketResult struct {
	Bracket TaxBracket
	Taxable float64
	Tax     float64
}

// MovingAverage computes a simple moving average over a window.
type MovingAverage struct {
	windowSize int
	values     []float64
}

func NewMovingAverage(windowSize int) *MovingAverage {
	return &MovingAverage{
		windowSize: windowSize,
		values:     make([]float64, 0, windowSize),
	}
}

// Add inserts a new value and returns the current moving average.
func (ma *MovingAverage) Add(value float64) float64 {
	ma.values = append(ma.values, value)

	if len(ma.values) > ma.windowSize {
		ma.values = ma.values[1:]
	}

	sum := 0.0
	for _, v := range ma.values {
		sum += v
	}
	return sum / float64(len(ma.values))
}

// Reset clears all stored values.
func (ma *MovingAverage) Reset() {
	ma.values = ma.values[:0]
}

// Current returns the current average without adding a value.
func (ma *MovingAverage) Current() float64 {
	if len(ma.values) == 0 {
		return 0
	}
	sum := 0.0
	for _, v := range ma.values {
		sum += v
	}
	return sum / float64(len(ma.values))
}

// PortfolioRebalancer redistributes asset weights to match target allocation.
type PortfolioRebalancer struct {
	tolerance float64 // drift tolerance before rebalancing (e.g. 0.05 for 5%)
}

// Asset represents a portfolio holding.
type Asset struct {
	Symbol       string
	CurrentValue float64
	TargetWeight float64 // decimal, e.g. 0.40 for 40%
}

func NewPortfolioRebalancer(tolerance float64) *PortfolioRebalancer {
	return &PortfolioRebalancer{tolerance: tolerance}
}

// RebalanceAction describes a trade needed to rebalance.
type RebalanceAction struct {
	Symbol string
	Action string  // "buy" or "sell"
	Amount float64
}

// Rebalance computes the trades needed to reach target allocation.
func (pr *PortfolioRebalancer) Rebalance(assets []Asset) ([]RebalanceAction, error) {
	if len(assets) == 0 {
		return nil, nil
	}

	totalValue := 0.0
	totalWeight := 0.0
	for _, a := range assets {
		totalValue += a.CurrentValue
		totalWeight += a.TargetWeight
	}

	if totalWeight != 1.0 {
		return nil, fmt.Errorf("target weights must sum to 1.0, got %v", totalWeight)
	}

	var actions []RebalanceAction
	for _, a := range assets {
		currentWeight := a.CurrentValue / totalValue
		drift := currentWeight - a.TargetWeight

		if math.Abs(drift) < pr.tolerance {
			continue
		}

		targetValue := a.TargetWeight * totalValue
		diff := targetValue - a.CurrentValue

		action := "buy"
		if diff < 0 {
			action = "sell"
			diff = -diff
		}

		actions = append(actions, RebalanceAction{
			Symbol: a.Symbol,
			Action: action,
			Amount: diff,
		})
	}

	return actions, nil
}

// AmortizationCalculator computes loan payment schedules.
type AmortizationCalculator struct {
	config FinancialConfig
}

func NewAmortizationCalculator(cfg FinancialConfig) *AmortizationCalculator {
	return &AmortizationCalculator{config: cfg}
}

// PaymentEntry represents one period in an amortization schedule.
type PaymentEntry struct {
	Period           int
	Payment          float64
	PrincipalPortion float64
	InterestPortion  float64
	RemainingBalance float64
}

// Calculate generates an amortization schedule.
// Parameters: principal, annualRate (decimal), totalMonths
func (ac *AmortizationCalculator) Calculate(principal, annualRate float64, totalMonths int) ([]PaymentEntry, error) {
	if totalMonths <= 0 {
		return nil, fmt.Errorf("total months must be positive")
	}
	if principal <= 0 {
		return nil, fmt.Errorf("principal must be positive")
	}

	monthlyRate := annualRate / 12.0

	// Standard amortization formula: M = P * [r(1+r)^n] / [(1+r)^n - 1]
	var payment float64
	if monthlyRate == 0 {
		payment = principal / float64(totalMonths)
	} else {
		factor := math.Pow(1+monthlyRate, float64(totalMonths))
		payment = principal * (monthlyRate * factor) / (factor - 1)
	}

	schedule := make([]PaymentEntry, 0, totalMonths)
	balance := principal

	for i := 1; i <= totalMonths; i++ {
		interest := balance * monthlyRate
		principalPart := payment - interest
		balance -= principalPart

		// Ensure final balance is exactly zero (avoid floating point dust)
		if i == totalMonths {
			principalPart += balance
			balance = 0
		}

		schedule = append(schedule, PaymentEntry{
			Period:           i,
			Payment:          payment,
			PrincipalPortion: principalPart,
			InterestPortion:  interest,
			RemainingBalance: balance,
		})
	}

	return schedule, nil
}
