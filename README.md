# Enterprise Math™

> Because `2 + 2` deserves a strategy pattern, circuit breaker, audit trail, and plugin system.

A magnificently over-engineered mathematical computation platform featuring:

- **Operation Abstractions** - Every math op is an interface with validation, audit trails, and metadata
- **Strategy Patterns** - Eager, retry, timeout, and circuit breaker execution strategies
- **Middleware Chain** - Logging, metrics, caching, validation, and audit middleware for every computation
- **Plugin System** - Financial math and linear algebra as dependency-resolved plugins
- **Expression Parser** - Recursive-descent parser with AST evaluation
- **Computation Pipelines** - Chainable, forkable pipeline stages with hooks
- **Observer Pattern** - Event bus with performance monitoring, error tracking, and audit observers
- **Tiered LRU Cache** - Two-level cache with TTL expiration and eviction callbacks
- **Configuration Management** - Hierarchical config with profiles, validation, and hot-reload
- **Metrics Collection** - Counters, gauges, histograms with quantile calculation

## Architecture

```
main.go
└── bootstrap.Application
    ├── config.ConfigManager + ProfileManager
    ├── logging.EnterpriseLogger + Middleware chain
    ├── operations.DefaultOperationFactory
    │   ├── Arithmetic (add, subtract, multiply, divide, modulo, power)
    │   ├── Trigonometry (sin, cos, tan, asin, atan2, sinh)
    │   ├── Statistical (mean, median, variance, stddev, correlation, percentile)
    │   └── Bitwise (and, or, xor, shift, popcount)
    ├── strategy chain (eager → timeout → retry → circuit breaker)
    ├── middleware chain (logging → validation → caching → metrics → audit)
    ├── cache.TieredCache (L1 + L2 LRU)
    ├── metrics.MetricsCollector
    ├── observer.EventBus
    │   ├── PerformanceObserver
    │   ├── ErrorCountObserver
    │   └── AuditObserver
    ├── plugin.PluginRegistry
    │   ├── FinancialPlugin (compound_interest, NPV, IRR, PV, FV)
    │   └── LinearAlgebraPlugin (dot_product, magnitude, normalize)
    ├── expression.Parser (lexer → tokens → recursive descent → AST)
    ├── pipeline.Pipeline (stages → hooks → fork/join)
    └── validation.ValidatorChain
```

## Run

```bash
go run .
```

## Why?

Testing knowledge graph indexing. That's why.
