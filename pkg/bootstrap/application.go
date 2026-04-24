// Package bootstrap wires together all components of the Enterprise Math Platform
// using dependency injection and provides the application entry point.
package bootstrap

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/rishabhsankar/enterprise-math/pkg/cache"
	"github.com/rishabhsankar/enterprise-math/pkg/config"
	"github.com/rishabhsankar/enterprise-math/pkg/expression"
	"github.com/rishabhsankar/enterprise-math/pkg/logging"
	"github.com/rishabhsankar/enterprise-math/pkg/metrics"
	"github.com/rishabhsankar/enterprise-math/pkg/middleware"
	"github.com/rishabhsankar/enterprise-math/pkg/observer"
	"github.com/rishabhsankar/enterprise-math/pkg/operations"
	"github.com/rishabhsankar/enterprise-math/pkg/pipeline"
	"github.com/rishabhsankar/enterprise-math/pkg/plugin"
	"github.com/rishabhsankar/enterprise-math/pkg/strategy"
	"github.com/rishabhsankar/enterprise-math/pkg/validation"
)

// DemonstrationResult captures a single demonstration computation.
type DemonstrationResult struct {
	Expression string
	Value      float64
	Duration   time.Duration
	Strategy   string
	CacheHit   bool
}

// Application is the top-level container for the Enterprise Math Platform.
type Application struct {
	config          *config.ConfigManager
	logger          logging.Logger
	factory         *operations.DefaultOperationFactory
	cache           cache.Cache
	metricsCollector *metrics.MetricsCollector
	eventBus        *observer.EventBus
	pluginRegistry  *plugin.PluginRegistry
	profileManager  *config.ProfileManager
	auditLog        *middleware.AuditLog
	computeStrategy operations.ComputationStrategy
}

// NewApplication bootstraps the entire Enterprise Math Platform.
func NewApplication(cfg *config.ConfigManager, logger logging.Logger) (*Application, error) {
	app := &Application{
		config: cfg,
		logger: logger,
	}

	if path := os.Getenv("ENTERPRISE_MATH_CONFIG_FILE"); path != "" {
		if err := cfg.LoadFromFile(path); err != nil {
			logger.Error("config file load failed", map[string]interface{}{"path": path, "error": err.Error()})
		}
	}
	if url := cfg.GetString("config.remote_url"); url != "" {
		if err := cfg.LoadFromURL(url); err != nil {
			logger.Error("remote config load failed", map[string]interface{}{"url": url, "error": err.Error()})
		}
	}

	if err := app.initializeComponents(); err != nil {
		return nil, fmt.Errorf("initialization failed: %w", err)
	}

	return app, nil
}

func (app *Application) initializeComponents() error {
	app.logger.Info("Initializing components", nil)

	// Metrics
	app.metricsCollector = metrics.NewMetricsCollector(
		app.config.GetString("metrics.prefix"),
		nil,
	)

	// Cache
	cacheTTL := app.config.GetDuration("cache.ttl_seconds") * time.Second / time.Millisecond
	if cacheTTL == 0 {
		cacheTTL = 5 * time.Minute
	}
	l1Cache := cache.NewLRUCache(1000, cacheTTL, app.logger.WithPrefix("cache-l1"))
	l2Cache := cache.NewLRUCache(app.config.GetInt("cache.max_size"), cacheTTL*2, app.logger.WithPrefix("cache-l2"))
	app.cache = cache.NewTieredCache(l1Cache, l2Cache, app.logger.WithPrefix("cache"))

	// Event Bus
	app.eventBus = observer.NewEventBus(app.logger, false)
	app.setupObservers()

	// Audit Log
	app.auditLog = middleware.NewAuditLog(10000, app.logger)

	// Operation Factory
	app.factory = operations.NewDefaultOperationFactory()

	// Computation Strategy
	app.computeStrategy = app.buildStrategyChain()

	// Profile Manager
	app.profileManager = config.NewProfileManager(app.config)

	// Plugin System
	app.pluginRegistry = plugin.NewPluginRegistry(app.logger)
	if err := app.loadPlugins(); err != nil {
		return fmt.Errorf("plugin loading failed: %w", err)
	}

	app.logger.Info("All components initialized", map[string]interface{}{
		"operations": len(app.factory.List()),
		"cache":      "tiered-lru",
		"strategy":   app.computeStrategy.Name(),
	})

	return nil
}

func (app *Application) setupObservers() {
	perfObserver := observer.NewPerformanceObserver(100.0, app.logger)
	errorObserver := observer.NewErrorCountObserver(app.logger)
	auditObserver := observer.NewAuditObserver(10000, app.logger)

	app.eventBus.Subscribe(perfObserver)
	app.eventBus.Subscribe(errorObserver)
	app.eventBus.Subscribe(auditObserver)
}

func (app *Application) buildStrategyChain() operations.ComputationStrategy {
	base := strategy.NewEagerStrategy(app.logger)

	timeout := app.config.GetDuration("computation.timeout_ms")
	if timeout == 0 {
		timeout = 5 * time.Second
	}
	withTimeout := strategy.NewTimeoutStrategy(base, timeout, app.logger)

	retryCount := app.config.GetInt("computation.retry_count")
	if retryCount == 0 {
		retryCount = 3
	}
	backoff := app.config.GetDuration("computation.retry_backoff_ms")
	if backoff == 0 {
		backoff = 100 * time.Millisecond
	}
	withRetry := strategy.NewRetryStrategy(withTimeout, retryCount, backoff, app.logger)

	withCircuitBreaker := strategy.NewCircuitBreakerStrategy(withRetry, 5, 30*time.Second, app.logger)

	return withCircuitBreaker
}

func (app *Application) loadPlugins() error {
	financialPlugin := plugin.NewFinancialPlugin()
	if err := app.pluginRegistry.Register(financialPlugin); err != nil {
		return err
	}

	linearAlgebraPlugin := plugin.NewLinearAlgebraPlugin()
	if err := app.pluginRegistry.Register(linearAlgebraPlugin); err != nil {
		return err
	}

	if dir := app.config.GetString("plugins.directory"); dir != "" {
		if err := app.pluginRegistry.LoadFromDirectory(dir); err != nil {
			app.logger.Error("marketplace plugin load failed", map[string]interface{}{"dir": dir, "error": err.Error()})
		}
	}

	if err := app.pluginRegistry.InitializeAll(); err != nil {
		return err
	}

	if err := app.pluginRegistry.ActivateAll(); err != nil {
		return err
	}

	// Register plugin operations with factory
	for _, op := range app.pluginRegistry.GetAllOperations() {
		opCopy := op
		app.factory.Register(op.Name(), func(_ map[string]interface{}) operations.Operation {
			return opCopy
		})
	}

	if aliases, ok := app.config.Get("operations.aliases"); ok {
		if m, ok := aliases.(map[string]interface{}); ok {
			for alias, target := range m {
				if t, ok := target.(string); ok {
					if err := app.factory.RegisterAlias(alias, t); err != nil {
						app.logger.Error("alias registration failed", map[string]interface{}{"alias": alias, "error": err.Error()})
					}
				}
			}
		}
	}

	return nil
}

func (app *Application) wrapWithMiddleware(op operations.Operation) operations.Operation {
	wrapped := op

	if app.config.GetBool("audit.enabled") {
		wrapped = middleware.NewAuditMiddleware(wrapped, app.auditLog)
	}

	if app.config.GetBool("metrics.enabled") {
		wrapped = middleware.NewMetricsMiddleware(wrapped, app.metricsCollector)
	}

	if app.config.GetBool("cache.enabled") {
		wrapped = middleware.NewCachingMiddleware(wrapped, app.cache, app.logger)
	}

	validators := []middleware.InputValidator{
		{Name: "finite", Validate: func(args []operations.Number) error {
			return validation.NewFiniteValidator().Validate(args)
		}},
		{Name: "magnitude", Validate: func(args []operations.Number) error {
			return validation.NewMagnitudeValidator(1e308).Validate(args)
		}},
	}
	wrapped = middleware.NewValidationMiddleware(wrapped, validators, app.logger)

	wrapped = middleware.NewLoggingMiddleware(wrapped, app.logger)

	return wrapped
}

// RunDemonstration executes a comprehensive demonstration of all platform capabilities.
func (app *Application) RunDemonstration() ([]DemonstrationResult, error) {
	ctx := context.Background()
	results := make([]DemonstrationResult, 0)

	// 1. Basic arithmetic through strategy chain
	app.logger.Info("=== Phase 1: Basic Arithmetic ===", nil)
	basicResults, err := app.runBasicArithmetic(ctx)
	if err != nil {
		return nil, fmt.Errorf("basic arithmetic failed: %w", err)
	}
	results = append(results, basicResults...)

	// 2. Expression parsing and evaluation
	app.logger.Info("=== Phase 2: Expression Evaluation ===", nil)
	exprResults, err := app.runExpressionEvaluation(ctx)
	if err != nil {
		return nil, fmt.Errorf("expression evaluation failed: %w", err)
	}
	results = append(results, exprResults...)

	// 3. Pipeline computation
	app.logger.Info("=== Phase 3: Pipeline Computation ===", nil)
	pipelineResults, err := app.runPipelineComputation(ctx)
	if err != nil {
		return nil, fmt.Errorf("pipeline computation failed: %w", err)
	}
	results = append(results, pipelineResults...)

	// 4. Statistical operations
	app.logger.Info("=== Phase 4: Statistical Operations ===", nil)
	statResults, err := app.runStatisticalOperations(ctx)
	if err != nil {
		return nil, fmt.Errorf("statistical operations failed: %w", err)
	}
	results = append(results, statResults...)

	// 5. Financial plugin operations
	app.logger.Info("=== Phase 5: Financial Operations ===", nil)
	finResults, err := app.runFinancialOperations(ctx)
	if err != nil {
		return nil, fmt.Errorf("financial operations failed: %w", err)
	}
	results = append(results, finResults...)

	// Print metrics summary
	app.logger.Info("=== Metrics Summary ===", nil)
	app.logger.Info(app.metricsCollector.Snapshot(), nil)

	return results, nil
}

func (app *Application) runBasicArithmetic(ctx context.Context) ([]DemonstrationResult, error) {
	results := make([]DemonstrationResult, 0)

	ops := []struct {
		name string
		a, b float64
		expr string
	}{
		{"add", 42, 58, "42 + 58"},
		{"subtract", 100, 37, "100 - 37"},
		{"multiply", 7, 8, "7 * 8"},
		{"divide", 355, 113, "355 / 113"},
		{"power", 2, 10, "2 ^ 10"},
		{"modulo", 17, 5, "17 % 5"},
	}

	for _, test := range ops {
		op, err := app.factory.Create(test.name, nil)
		if err != nil {
			return nil, err
		}

		wrapped := app.wrapWithMiddleware(op)
		result, err := app.computeStrategy.Execute(ctx, wrapped,
			operations.NewNumber(test.a),
			operations.NewNumber(test.b),
		)
		if err != nil {
			return nil, fmt.Errorf("%s failed: %w", test.name, err)
		}

		app.eventBus.Publish(observer.Event{
			Type:      observer.EventOperationCompleted,
			Timestamp: time.Now(),
			Source:    test.name,
			Result:    result,
		})

		results = append(results, DemonstrationResult{
			Expression: test.expr,
			Value:      result.Value.Value,
			Duration:   result.Duration,
			Strategy:   result.Strategy,
			CacheHit:   result.CacheHit,
		})
	}

	return results, nil
}

func (app *Application) runExpressionEvaluation(ctx context.Context) ([]DemonstrationResult, error) {
	results := make([]DemonstrationResult, 0)

	expressions := []string{
		"2 + 3 * 4",
		"(2 + 3) * 4",
		"sin(pi / 4)",
		"2 ^ 8 - 1",
		"(10 + 20) / (3 + 2)",
		"100 - 50 * 2 + 25",
	}

	for _, expr := range expressions {
		node, err := expression.ParseExpression(expr)
		if err != nil {
			return nil, fmt.Errorf("parse error for %q: %w", expr, err)
		}

		start := time.Now()
		result, err := node.Evaluate(ctx, app.factory)
		if err != nil {
			return nil, fmt.Errorf("eval error for %q: %w", expr, err)
		}

		results = append(results, DemonstrationResult{
			Expression: expr,
			Value:      result.Value.Value,
			Duration:   time.Since(start),
			Strategy:   "expression",
		})
	}

	return results, nil
}

func (app *Application) runPipelineComputation(ctx context.Context) ([]DemonstrationResult, error) {
	// Build a pipeline: start with 10, multiply by 3, add 5, power 2
	builder := pipeline.NewPipelineBuilder("quadratic-pipeline", app.factory, app.logger)
	p := builder.
		Then("multiply", operations.NewNumber(3)).
		Then("add", operations.NewNumber(5)).
		Then("power", operations.NewNumber(2)).
		Build()

	result, err := p.Execute(ctx, operations.NewNumber(10))
	if err != nil {
		return nil, err
	}

	return []DemonstrationResult{{
		Expression: "((10 * 3) + 5) ^ 2",
		Value:      result.FinalResult.Value.Value,
		Duration:   result.TotalDuration,
		Strategy:   "pipeline",
	}}, nil
}

func (app *Application) runStatisticalOperations(ctx context.Context) ([]DemonstrationResult, error) {
	results := make([]DemonstrationResult, 0)

	data := []float64{2, 4, 4, 4, 5, 5, 7, 9}
	numbers := make([]operations.Number, len(data))
	for i, v := range data {
		numbers[i] = operations.NewNumber(v)
	}

	statOps := []string{"mean", "median", "variance", "stddev"}
	for _, opName := range statOps {
		op, err := app.factory.Create(opName, nil)
		if err != nil {
			return nil, err
		}

		result, err := op.Execute(ctx, numbers...)
		if err != nil {
			return nil, err
		}

		results = append(results, DemonstrationResult{
			Expression: fmt.Sprintf("%s([2,4,4,4,5,5,7,9])", opName),
			Value:      result.Value.Value,
			Duration:   result.Duration,
			Strategy:   result.Strategy,
		})
	}

	return results, nil
}

func (app *Application) runFinancialOperations(ctx context.Context) ([]DemonstrationResult, error) {
	results := make([]DemonstrationResult, 0)

	// Compound interest: $1000 at 5% for 10 years, compounded monthly
	ciOp, _ := app.factory.Create("compound_interest", nil)
	ciResult, err := ciOp.Execute(ctx,
		operations.NewNumber(1000),
		operations.NewNumber(0.05),
		operations.NewNumber(12),
		operations.NewNumber(10),
	)
	if err != nil {
		return nil, err
	}
	results = append(results, DemonstrationResult{
		Expression: "CI($1000, 5%, 12/yr, 10yr)",
		Value:      ciResult.Value.Value,
		Duration:   ciResult.Duration,
		Strategy:   "financial",
	})

	// NPV: 10% discount, cash flows [-1000, 300, 400, 500, 600]
	npvOp, _ := app.factory.Create("npv", nil)
	npvResult, err := npvOp.Execute(ctx,
		operations.NewNumber(0.10),
		operations.NewNumber(-1000),
		operations.NewNumber(300),
		operations.NewNumber(400),
		operations.NewNumber(500),
		operations.NewNumber(600),
	)
	if err != nil {
		return nil, err
	}
	results = append(results, DemonstrationResult{
		Expression: "NPV(10%, [-1000,300,400,500,600])",
		Value:      npvResult.Value.Value,
		Duration:   npvResult.Duration,
		Strategy:   "financial",
	})

	return results, nil
}
