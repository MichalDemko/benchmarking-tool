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

func main() {
	fmt.Println("Starting benchmarking tool...")

	// Get config file from command line or use default
	configFile := "config.yaml"
	if len(os.Args) > 1 {
		configFile = os.Args[1]
	}

	cfg, err := config.LoadConfig(configFile)
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
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
		log.Fatalf("Error during benchmark execution: %v", err)
	}

	finalResults := metricsCollector.GetResults()
	
	// Generate report using the proper Reporter
	rep := reporter.NewReporter()
	rep.Generate(cfg, finalResults)

	fmt.Println("Benchmarking tool finished.")
}
