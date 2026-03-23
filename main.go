package main

import (
	"fmt"
	"log"
	"os"

	"benchmarking-tool/config"
	"benchmarking-tool/metrics"
	"benchmarking-tool/reporter"
	"benchmarking-tool/runner"
)

func run(args []string) error {
	fmt.Println("Starting benchmarking tool...")

	configFile := "config.yaml"
	if len(args) > 1 {
		configFile = args[1]
	}

	cfg, err := config.LoadConfig(configFile)
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	fmt.Printf("Configuration loaded: Mode='%s', Duration=%ds, RPS=%d\n",
		cfg.Execution.Mode, cfg.Execution.DurationSeconds, cfg.Execution.RequestsPerSecond)
	fmt.Printf("Base URLs: %v\n", cfg.BaseUrls)
	fmt.Printf("Endpoints to test: %d\n", len(cfg.Endpoints))
	for name, ep := range cfg.Endpoints {
		fmt.Printf("  - %s: %s %s\n", name, ep.Method, ep.Path)
	}
	fmt.Printf("Parameter generators defined: %d\n", len(cfg.ParameterGenerators))

	metricsCollector := metrics.NewCollector()
	benchmarkRunner := runner.NewRunner(cfg, metricsCollector)

	_, err = benchmarkRunner.Run()
	if err != nil {
		return fmt.Errorf("error during benchmark execution: %w", err)
	}

	finalResults := metricsCollector.GetResults()

	// Generate report using the proper Reporter
	rep := reporter.NewReporter()
	rep.Generate(cfg, finalResults)

	fmt.Println("Benchmarking tool finished.")
	return nil
}

func main() {
	if err := run(os.Args); err != nil {
		log.Fatalf("Application error: %v", err)
	}
}
