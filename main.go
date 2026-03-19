// Enterprise Math - Because simple arithmetic deserves enterprise architecture.
//
// This application demonstrates the power of over-engineering by implementing
// basic mathematical operations through a sophisticated multi-layered architecture
// featuring dependency injection, strategy patterns, observer patterns, middleware
// chains, plugin systems, and distributed-ready abstractions.
package main

import (
	"fmt"
	"os"

	"github.com/rishabhsankar/enterprise-math/pkg/bootstrap"
	"github.com/rishabhsankar/enterprise-math/pkg/config"
	"github.com/rishabhsankar/enterprise-math/pkg/logging"
)

func main() {
	logger := logging.NewEnterpriseLogger(logging.LogConfig{
		Level:      logging.INFO,
		Format:     logging.JSON,
		OutputPath: "stdout",
		EnableCaller: true,
	})

	logger.Info("Initializing Enterprise Math Platform™", map[string]interface{}{
		"version": "1.0.0-enterprise",
		"edition": "platinum",
	})

	cfg := config.NewConfigManager(config.WithDefaults(), config.WithEnvOverrides("ENTERPRISE_MATH_"))
	if err := cfg.Validate(); err != nil {
		logger.Fatal("Configuration validation failed", map[string]interface{}{"error": err.Error()})
		os.Exit(1)
	}

	app, err := bootstrap.NewApplication(cfg, logger)
	if err != nil {
		logger.Fatal("Application bootstrap failed", map[string]interface{}{"error": err.Error()})
		os.Exit(1)
	}

	results, err := app.RunDemonstration()
	if err != nil {
		logger.Error("Demonstration failed", map[string]interface{}{"error": err.Error()})
		os.Exit(1)
	}

	for _, r := range results {
		fmt.Printf("[%s] %s = %v (computed in %v)\n", r.Strategy, r.Expression, r.Value, r.Duration)
	}

	logger.Info("Enterprise Math Platform™ completed successfully", nil)
}
