package runner

import (
	"benchmarking-tool/config"
	"benchmarking-tool/metrics"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"
)

// Runner executes the benchmark tests
type Runner struct {
	cfg         *config.Config
	collector   *metrics.Collector
	httpClient  *http.Client
	endpointIdx uint64 // For deterministic endpoint selection
	mu          sync.Mutex // For thread-safe access to endpointIdx
	resultsCh   chan metrics.MetricDetail // Channel for sending results
}

// BenchmarkResult holds the outcome of a benchmark run
type BenchmarkResult struct {
	TotalRequestsMade  int64
	SuccessfulRequests int64
	FailedRequests     int64
	RequestTimings     []time.Duration
	StatusCodes        map[int]int64 // count per status code
	// TODO: Add more fields like error details per URL, etc.
}

// NewRunner creates a new benchmark runner
func NewRunner(cfg *config.Config, collector *metrics.Collector) *Runner {
	requestTimeout := 10 * time.Second // Default timeout
	if cfg.RequestTimeoutMs > 0 {
		requestTimeout = time.Duration(cfg.RequestTimeoutMs) * time.Millisecond
	}
	return &Runner{
		cfg:       cfg,
		collector: collector,
		httpClient: &http.Client{
			Timeout: requestTimeout,
		},
		endpointIdx: 0,
		resultsCh:   make(chan metrics.MetricDetail, 1000), // Buffer size can be tuned
	}
}

// Run starts the benchmark based on the configuration
func (r *Runner) Run() (*BenchmarkResult, error) {
	log.Printf("Starting benchmark in '%s' mode.", r.cfg.Mode)

	switch r.cfg.Mode {
	case "fixed":
		return r.runFixedRPSMode()
	case "ramp":
		return r.runRampUpMode() // We will implement this later
	default:
		return nil, fmt.Errorf("unknown mode: %s", r.cfg.Mode)
	}
}

// runFixedRPSMode executes the benchmark at a fixed number of requests per second
func (r *Runner) runFixedRPSMode() (*BenchmarkResult, error) {
	log.Printf("Running in fixed RPS mode: %d RPS for %d seconds.", r.cfg.RequestsPerSecond, r.cfg.DurationSeconds)
	if r.cfg.RequestsPerSecond <= 0 {
		return nil, fmt.Errorf("requestsPerSecond must be positive for fixed mode")
	}
	if len(r.cfg.Endpoints) == 0 {
		return nil, fmt.Errorf("no endpoints configured")
	}

		var wg sync.WaitGroup
		startTime := time.Now()
		totalDuration := time.Duration(r.cfg.DurationSeconds) * time.Second
		ticker := time.NewTicker(time.Second / time.Duration(r.cfg.RequestsPerSecond))
		defer ticker.Stop()
	
		// Start collector goroutine
		collectorWg := sync.WaitGroup{}
		collectorWg.Add(1)
		go func() {
			defer collectorWg.Done()
			for detail := range r.resultsCh {
				r.collector.AppendDetail(detail)
			}
		}()
	
		requestCount := 0
	
	loop:
		for time.Since(startTime) < totalDuration {
			select {
			case <-ticker.C:
				if time.Since(startTime) >= totalDuration {
					break loop
				}
				wg.Add(1)
				go func() {
					defer wg.Done()
					// Deterministic endpoint selection (round-robin)
					r.mu.Lock()
					endpoint := r.cfg.Endpoints[r.endpointIdx%uint64(len(r.cfg.Endpoints))]
					r.endpointIdx++
					r.mu.Unlock()
					detail := r.makeRequest(endpoint)
					r.resultsCh <- detail
					requestCount++
				}()
			case <-time.After(totalDuration - time.Since(startTime)):
				log.Println("Benchmark duration reached.")
				break loop
			}
		}
	
	wg.Wait() // Wait for all inflight requests to complete
	close(r.resultsCh) // Signal collector to finish
	collectorWg.Wait() // Wait for collector to finish
	log.Printf("Fixed RPS mode finished. Total requests attempted: %d", requestCount)

	// The actual results will be fetched from the collector in the main function
	// This BenchmarkResult is more of a summary from the runner's perspective if needed.
	// For now, we'll return an empty one as the collector holds the detailed metrics.
	return &BenchmarkResult{}, nil
}

// runRampUpMode executes the benchmark by gradually increasing RPS
func (r *Runner) runRampUpMode() (*BenchmarkResult, error) {
	log.Printf("Running in ramp-up mode: Initial %d RPS, increment %d RPS every %d seconds, for %d seconds total. Max RPS: %d",
		r.cfg.InitialRPS, r.cfg.RPSIncrement, r.cfg.RampUpPeriodSec, r.cfg.DurationSeconds, r.cfg.MaxRPS)
	// TODO: Implement logic for ramp-up RPS
	// - Loop for r.cfg.DurationSeconds
	// - Keep track of current RPS, starting at r.cfg.InitialRPS
	// - Every r.cfg.RampUpPeriodSec, increase current RPS by r.cfg.RPSIncrement, up to r.cfg.MaxRPS (if set)
	// - In each second, send currentRPS requests, distributed evenly.
	// - For each request:
	//   - Pick an endpoint
	//   - Make the HTTP request
	//   - Record metrics
	log.Println("Ramp-up mode is not yet implemented.")
	return &BenchmarkResult{}, nil // Placeholder
}

// makeRequest sends a single HTTP request and collects basic metrics
func (r *Runner) makeRequest(endpoint config.EndpointConfig) metrics.MetricDetail {
	reqStartTime := time.Now()
	var req *http.Request
	var err error

	if endpoint.Body != "" {
		req, err = http.NewRequest(endpoint.Method, endpoint.URL, strings.NewReader(endpoint.Body))
	} else {
		req, err = http.NewRequest(endpoint.Method, endpoint.URL, nil)
	}

	if err != nil {
		log.Printf("Error creating request for %s: %v", endpoint.URL, err)
		return metrics.MetricDetail{
			URL:        endpoint.URL,
			Method:     endpoint.Method,
			StatusCode: 0,
			Duration:   time.Since(reqStartTime),
			IsError:    true,
			ErrorMsg:   fmt.Sprintf("request creation failed: %v", err),
			Timestamp:  time.Now(),
		}
	}

	for key, value := range endpoint.Headers {
		req.Header.Set(key, value)
	}
	// Add a default User-Agent if not present
	if req.Header.Get("User-Agent") == "" {
		req.Header.Set("User-Agent", "benchmarking-tool/0.1")
	}

	resp, err := r.httpClient.Do(req)
	duration := time.Since(reqStartTime)

	if err != nil {
		log.Printf("Error making request to %s: %v", endpoint.URL, err)
		return metrics.MetricDetail{
			URL:        endpoint.URL,
			Method:     endpoint.Method,
			StatusCode: 0,
			Duration:   duration,
			IsError:    true,
			ErrorMsg:   err.Error(),
			Timestamp:  time.Now(),
		}
	}
	defer resp.Body.Close()

	return metrics.MetricDetail{
		URL:        endpoint.URL,
		Method:     endpoint.Method,
		StatusCode: resp.StatusCode,
		Duration:   duration,
		IsError:    false,
		ErrorMsg:   "",
		Timestamp:  time.Now(),
	}
}
