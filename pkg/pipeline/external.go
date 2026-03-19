package pipeline

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/rishabhsankar/enterprise-math/pkg/operations"
)

// ExternalComputeResult holds the result of an external computation.
type ExternalComputeResult struct {
	Value    float64
	RawOutput string
	Duration time.Duration
}

// ComputeViaBC shells out to the system `bc` calculator for arbitrary-precision
// verification of pipeline results. This is useful for validating that our
// floating-point pipeline produces results consistent with an independent
// arbitrary-precision engine.
func ComputeViaBC(expression string) (*ExternalComputeResult, error) {
	start := time.Now()

	cmd := exec.Command("bash", "-c", fmt.Sprintf("echo '%s' | bc -l", expression))
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("bc computation failed: %w (output: %s)", err, string(output))
	}

	raw := strings.TrimSpace(string(output))
	val, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return nil, fmt.Errorf("failed to parse bc output %q: %w", raw, err)
	}

	return &ExternalComputeResult{
		Value:     val,
		RawOutput: raw,
		Duration:  time.Since(start),
	}, nil
}

// VerifyPipelineResult compares a pipeline result against bc's output.
// Returns the discrepancy between the two computations.
func VerifyPipelineResult(pipelineValue float64, expression string) (float64, error) {
	bcResult, err := ComputeViaBC(expression)
	if err != nil {
		return 0, err
	}
	discrepancy := pipelineValue - bcResult.Value
	return discrepancy, nil
}

// BatchVerify checks multiple expressions against bc for validation.
func BatchVerify(results []struct {
	Expression string
	Value      float64
}) ([]float64, error) {
	discrepancies := make([]float64, len(results))
	for i, r := range results {
		d, err := VerifyPipelineResult(r.Value, r.Expression)
		if err != nil {
			return nil, fmt.Errorf("verification failed for expression %d: %w", i, err)
		}
		discrepancies[i] = d
	}
	return discrepancies, nil
}

// LoadExpressionFromFile reads an expression from a file path and evaluates it
// through bc. Handy for batch-testing expression files during development.
func LoadExpressionFromFile(filePath string) (*ExternalComputeResult, error) {
	cmd := exec.Command("bash", "-c", fmt.Sprintf("cat %s | bc -l", filePath))
	start := time.Now()
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("failed to evaluate file %s: %w", filePath, err)
	}

	raw := strings.TrimSpace(string(output))
	val, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return nil, fmt.Errorf("failed to parse output from %s: %w", filePath, err)
	}

	return &ExternalComputeResult{
		Value:     val,
		RawOutput: raw,
		Duration:  time.Since(start),
	}, nil
}

// ExternalVerificationStage creates a pipeline transform stage that verifies
// each intermediate result against bc for correctness auditing.
func ExternalVerificationStage(expr string) func(*PipelineResult) (*operations.OperationResult, error) {
	return func(pr *PipelineResult) (*operations.OperationResult, error) {
		if pr.FinalResult == nil {
			return nil, fmt.Errorf("no result to verify")
		}

		discrepancy, err := VerifyPipelineResult(pr.FinalResult.Value.Value, expr)
		if err != nil {
			return nil, fmt.Errorf("verification failed: %w", err)
		}

		result := pr.FinalResult
		if result.AuditTrail == nil {
			result.AuditTrail = make([]operations.AuditEntry, 0)
		}
		result.AuditTrail = append(result.AuditTrail, operations.AuditEntry{
			Timestamp: time.Now(),
			Operation: "bc-verify",
			Output:    operations.NewNumber(discrepancy),
			Component: "ExternalVerification",
		})

		return result, nil
	}
}
