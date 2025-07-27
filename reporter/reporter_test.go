package reporter

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"benchmarking-tool/config"
	"benchmarking-tool/metrics"
)

func TestNewReporter(t *testing.T) {
	reporter := NewReporter()

	if reporter == nil {
		t.Error("NewReporter() should not return nil")
	}
}

func TestReporter_Generate(t *testing.T) {
	testCases := []struct {
		name            string
		cfg             *config.Config
		results         metrics.AggregatedResults
		expectedStrings []string
	}{
		{
			name: "Standard report",
			cfg: &config.Config{
				Execution: config.ExecutionConfig{
					Mode:             "fixed",
					DurationSeconds:  60,
					RequestsPerSecond: 100,
				},
			},
			results: metrics.AggregatedResults{
				TotalRequests:      1000,
				SuccessfulRequests: 950,
				FailedRequests:     50,
				TotalDuration:      100 * time.Second,
				AvgDuration:        100 * time.Millisecond,
				MinDuration:        10 * time.Millisecond,
				MaxDuration:        500 * time.Millisecond,
				StatusCodesCount:   map[int]int64{200: 950, 500: 50},
				ErrorDetails:       map[string]int{"timeout": 30, "connection refused": 20},
			},
			expectedStrings: []string{"1000", "950", "50", "fixed", "60"},
		},
		{
			name: "Empty results",
			cfg: &config.Config{
				Execution: config.ExecutionConfig{
					Mode:            "fixed",
					DurationSeconds: 30,
				},
			},
			results: metrics.AggregatedResults{
				TotalRequests:      0,
				SuccessfulRequests: 0,
				FailedRequests:     0,
				TotalDuration:      0,
				AvgDuration:        0,
				MinDuration:        0,
				MaxDuration:        0,
				StatusCodesCount:   make(map[int]int64),
				ErrorDetails:       make(map[string]int),
			},
			expectedStrings: []string{"0", "Benchmark Report"},
		},
		{
			name: "Success rate",
			cfg: &config.Config{
				Execution: config.ExecutionConfig{
					Mode:            "fixed",
					DurationSeconds: 30,
				},
			},
			results: metrics.AggregatedResults{
				TotalRequests:      100,
				SuccessfulRequests: 95,
				FailedRequests:     5,
				StatusCodesCount:   map[int]int64{200: 95, 500: 5},
				ErrorDetails:       make(map[string]int),
			},
			expectedStrings: []string{"95", "5"},
		},
		{
			name: "Multiple status codes",
			cfg: &config.Config{
				Execution: config.ExecutionConfig{
					Mode:            "fixed",
					DurationSeconds: 60,
				},
			},
			results: metrics.AggregatedResults{
				TotalRequests:      1000,
				SuccessfulRequests: 800,
				FailedRequests:     200,
				StatusCodesCount:   map[int]int64{200: 700, 201: 100, 400: 50, 404: 75, 500: 75},
				ErrorDetails:       make(map[string]int),
			},
			expectedStrings: []string{"1000", "800", "200"},
		},
		{
			name: "Error details",
			cfg: &config.Config{
				Execution: config.ExecutionConfig{
					Mode:            "fixed",
					DurationSeconds: 30,
				},
			},
			results: metrics.AggregatedResults{
				TotalRequests:      100,
				SuccessfulRequests: 70,
				FailedRequests:     30,
				StatusCodesCount:   map[int]int64{200: 70, 0: 30},
				ErrorDetails:       map[string]int{"connection timeout": 15, "connection refused": 10, "dns resolution failed": 5},
			},
			expectedStrings: []string{"30", "70"},
		},
		{
			name: "Duration formatting",
			cfg: &config.Config{
				Execution: config.ExecutionConfig{
					Mode:            "fixed",
					DurationSeconds: 30,
				},
			},
			results: metrics.AggregatedResults{
				TotalRequests:      10,
				SuccessfulRequests: 10,
				FailedRequests:     0,
				TotalDuration:      15 * time.Second,
				AvgDuration:        1500 * time.Millisecond,
				MinDuration:        100 * time.Millisecond,
				MaxDuration:        3 * time.Second,
				StatusCodesCount:   map[int]int64{200: 10},
				ErrorDetails:       make(map[string]int),
			},
			expectedStrings: []string{"Min Request Time", "Max Request Time"},
		},
		{
			name: "Configuration info",
			cfg: &config.Config{
				Execution: config.ExecutionConfig{
					Mode:              "fixed",
					DurationSeconds:   120,
					RequestsPerSecond: 50,
				},
			},
			results: metrics.AggregatedResults{
				TotalRequests:      100,
				SuccessfulRequests: 95,
				FailedRequests:     5,
				StatusCodesCount:   map[int]int64{200: 95, 500: 5},
				ErrorDetails:       make(map[string]int),
			},
			expectedStrings: []string{"fixed", "120", "50"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			oldStdout := os.Stdout
			r, w, _ := os.Pipe()
			os.Stdout = w

			reporter := NewReporter()
			reporter.Generate(tc.cfg, tc.results)

			w.Close()
			os.Stdout = oldStdout

			var buf bytes.Buffer
			io.Copy(&buf, r)
			output := buf.String()

			for _, s := range tc.expectedStrings {
				if !strings.Contains(output, s) {
					t.Errorf("Report should contain '%s'", s)
				}
			}

			if tc.name == "Empty results" {
				if output == "" {
					t.Error("Report should generate output even for empty results")
				}
			}
		})
	}
}

func TestReporter_ReportStructure(t *testing.T) {
	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	reporter := NewReporter()

	cfg := &config.Config{
		Execution: config.ExecutionConfig{
			Mode:            "fixed",
			DurationSeconds: 60,
		},
	}

	results := metrics.AggregatedResults{
		TotalRequests:      100,
		SuccessfulRequests: 90,
		FailedRequests:     10,
		AvgDuration:        50 * time.Millisecond,
		MinDuration:        10 * time.Millisecond,
		MaxDuration:        200 * time.Millisecond,
		StatusCodesCount: map[int]int64{
			200: 90,
			500: 10,
		},
		ErrorDetails: map[string]int{
			"timeout": 10,
		},
	}

	reporter.Generate(cfg, results)

	// Restore stdout and capture output
	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	io.Copy(&buf, r)
	output := buf.String()

	// Check that the report has a reasonable structure
	lines := strings.Split(output, "\n")
	if len(lines) < 5 {
		t.Error("Report should have multiple lines with different sections")
	}

	// Should contain some form of header or title
	if !strings.Contains(output, "Benchmark Report") {
		t.Error("Report should contain benchmark report header")
	}
}
