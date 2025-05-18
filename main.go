package main

import (
	"fmt"
	"log"
	"os"

	"benchmarking-tool/config"
	"benchmarking-tool/metrics" // Added import
	"benchmarking-tool/reporter"
	"benchmarking-tool/runner"
)

func main() {
	fmt.Println("Starting benchmarking tool...")

	// Initialize Go Modules if you haven't: go mod init benchmarking-tool
	// Then: go mod tidy

	cfg, err := config.LoadConfig("config.yaml")
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
		os.Exit(1)
	}

	fmt.Printf("Configuration loaded: Mode='%s', Duration=%ds, RPS (Fixed)=%d\n", cfg.Mode, cfg.DurationSeconds, cfg.RequestsPerSecond)
	fmt.Printf("Endpoints to test: %d\n", len(cfg.Endpoints))
	for _, ep := range cfg.Endpoints {
		fmt.Printf("  - %s %s\n", ep.Method, ep.URL)
	}

	metricsCollector := metrics.NewCollector()

	benchmarkRunner := runner.NewRunner(cfg, metricsCollector)

	_, err = benchmarkRunner.Run() // Runner itself doesn't return detailed results directly
	if err != nil {
		log.Fatalf("Error during benchmark execution: %v", err)
	}

	// Get aggregated results from the collector
	finalResults := metricsCollector.GetResults()

	// Initialize reporter and generate report
	reportGenerator := reporter.NewReporter()
	reportGenerator.Generate(cfg, finalResults) // Pass config for context if needed by reporter

	fmt.Println("Benchmarking tool finished.")
}
