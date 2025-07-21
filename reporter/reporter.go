package reporter

import (
	"benchmarking-tool/config"
	"benchmarking-tool/metrics"
	"fmt"
	"os"
	"sort"
	"text/tabwriter"
	// "time"
)

// Reporter generates and displays benchmark reports
type Reporter struct {
}

// NewReporter creates a new reporter
func NewReporter() *Reporter {
	return &Reporter{}
}

// Generate creates a report from benchmark results and metrics data
func (r *Reporter) Generate(cfg *config.Config, results metrics.AggregatedResults) {
	fmt.Println("\n--- Benchmark Report ---")

	// Example using tabwriter for formatted output
	const padding = 3
	writer := tabwriter.NewWriter(os.Stdout, 0, 0, padding, ' ', tabwriter.AlignRight|tabwriter.Debug)
	defer writer.Flush()

	fmt.Fprintf(writer, "Metric\tValue\n")
	fmt.Fprintf(writer, "------\t-----\n")

	fmt.Fprintf(writer, "Test Mode\t%s\n", cfg.Execution.Mode)
	fmt.Fprintf(writer, "Configured Duration\t%ds\n", cfg.Execution.DurationSeconds)
	if cfg.Execution.Mode == "fixed" {
		fmt.Fprintf(writer, "Configured RPS\t%d\n", cfg.Execution.RequestsPerSecond)
	}

	fmt.Fprintf(writer, "Total Requests\t%d\n", results.TotalRequests)
	fmt.Fprintf(writer, "Successful Requests\t%d\n", results.SuccessfulRequests)
	fmt.Fprintf(writer, "Failed Requests\t%d\n", results.FailedRequests)

	if results.TotalRequests > 0 {
		errorRate := float64(results.FailedRequests) / float64(results.TotalRequests) * 100
		fmt.Fprintf(writer, "Error Rate\t%.2f%%\n", errorRate)
	}

	fmt.Fprintf(writer, "Min Request Time\t%s\n", results.MinDuration)
	fmt.Fprintf(writer, "Max Request Time\t%s\n", results.MaxDuration)
	fmt.Fprintf(writer, "Avg Request Time\t%s\n", results.AvgDuration)

	// TODO: Calculate and display actual RPS achieved based on timestamps in MetricDetail
	// This would involve looking at the timestamps of requests in metrics.AllRequests
	// and calculating requests per second over the actual run duration.
	// For now, we can state the target RPS for fixed mode.

	fmt.Println("\nStatus Code Distribution:")
	// Sort status codes for consistent output
	var statusKeys []int
	for code := range results.StatusCodesCount {
		statusKeys = append(statusKeys, code)
	}
	sort.Ints(statusKeys)

	for _, code := range statusKeys {
		fmt.Fprintf(writer, "  Status %d\t%d\n", code, results.StatusCodesCount[code])
	}
	writer.Flush() // Flush before printing next section

	if len(results.ErrorDetails) > 0 {
		fmt.Println("\nError Message Summary:")
		// Sort error messages for consistent output
		var errorKeys []string
		for msg := range results.ErrorDetails {
			errorKeys = append(errorKeys, msg)
		}
		sort.Strings(errorKeys)

		for _, msg := range errorKeys {
			fmt.Fprintf(writer, "  '%s'\t%d times\n", msg, results.ErrorDetails[msg])
		}
		writer.Flush()
	}

	// TODO: Add latency percentile reporting (p50, p90, p95, p99)
	// This would require sorting all request durations from metrics.AllRequests
	// and then picking values at specific percentiles.

	fmt.Println("\n--- End of Report ---")
}
