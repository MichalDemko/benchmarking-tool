package reporter

import (
	"benchmarking-tool/config"
	"benchmarking-tool/metrics"
	"fmt"
	"io"
	"os"
	"sort"
)

// Reporter generates and displays benchmark reports
type Reporter struct{}

// NewReporter creates a new Reporter
func NewReporter() *Reporter {
	return &Reporter{}
}

const metricLabelWidth = 22 // fits "Configured Duration", "Successful Requests"

func writeMetricRow(w io.Writer, label, value string) {
	fmt.Fprintf(w, "%-*s  %s\n", metricLabelWidth, label, value)
}

// Generate creates a report from benchmark results and metrics data
func (r *Reporter) Generate(cfg *config.Config, results metrics.AggregatedResults) {
	out := os.Stdout

	fmt.Fprintln(out, "\n--- Benchmark Report ---")

	writeMetricRow(out, "Metric", "Value")
	writeMetricRow(out, "------", "-----")

	writeMetricRow(out, "Test Mode", cfg.Execution.Mode)
	writeMetricRow(out, "Configured Duration", fmt.Sprintf("%ds", cfg.Execution.DurationSeconds))
	if cfg.Execution.Mode == "fixed" {
		writeMetricRow(out, "Configured RPS", fmt.Sprintf("%d", cfg.Execution.RequestsPerSecond))
	}

	writeMetricRow(out, "Total Requests", fmt.Sprintf("%d", results.TotalRequests))
	writeMetricRow(out, "Successful Requests", fmt.Sprintf("%d", results.SuccessfulRequests))
	writeMetricRow(out, "Failed Requests", fmt.Sprintf("%d", results.FailedRequests))

	if results.TotalRequests > 0 {
		errorRate := float64(results.FailedRequests) / float64(results.TotalRequests) * 100
		writeMetricRow(out, "Error Rate", fmt.Sprintf("%.2f%%", errorRate))
	}

	writeMetricRow(out, "Min Request Time", results.MinDuration.String())
	writeMetricRow(out, "Max Request Time", results.MaxDuration.String())
	writeMetricRow(out, "Avg Request Time", results.AvgDuration.String())

	fmt.Fprintln(out, "\nStatus Code Distribution:")

	var statusKeys []int
	for code := range results.StatusCodesCount {
		statusKeys = append(statusKeys, code)
	}
	sort.Ints(statusKeys)

	for _, code := range statusKeys {
		label := fmt.Sprintf("Status %d", code)
		writeMetricRow(out, label, fmt.Sprintf("%d", results.StatusCodesCount[code]))
	}

	if len(results.ErrorDetails) > 0 {
		fmt.Fprintln(out, "\nError Message Summary:")
		var errorKeys []string
		for msg := range results.ErrorDetails {
			errorKeys = append(errorKeys, msg)
		}
		sort.Strings(errorKeys)

		for _, msg := range errorKeys {
			label := fmt.Sprintf("'%s'", msg)
			writeMetricRow(out, label, fmt.Sprintf("%d times", results.ErrorDetails[msg]))
		}
	}

	fmt.Fprintln(out, "\n--- End of Report ---")
}
