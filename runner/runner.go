package runner

import (
	"benchmarking-tool/config"
	"benchmarking-tool/metrics"
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math/big"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/time/rate"
)

// Runner executes the benchmark tests with the new configuration format
type Runner struct {
	cfg           *config.Config
	collector     *metrics.Collector
	client        *http.Client
	endpointNames []string // sorted keys of cfg.Endpoints (stable round-robin / weighted)
	endpointIdx   atomic.Uint64
	urlIdx        atomic.Uint64
	resultsCh     chan metrics.MetricDetail
}

// BenchmarkResult holds the outcome of a benchmark run
type BenchmarkResult struct {
	TotalRequestsMade        int64
	SuccessfulRequests       int64
	FailedRequests           int64
	DroppedDueToBackpressure int64
	RequestTimings           []time.Duration
	StatusCodes              map[int]int64
}

// NewRunner creates a new benchmark runner
func NewRunner(cfg *config.Config, collector *metrics.Collector) *Runner {
	requestTimeout := time.Duration(cfg.Execution.RequestTimeoutMs) * time.Millisecond

	epNames := make([]string, 0, len(cfg.Endpoints))
	for n := range cfg.Endpoints {
		epNames = append(epNames, n)
	}
	sort.Strings(epNames)

	mw := cfg.Execution.MaxWorkers
	if mw < 1 {
		mw = 1
	}
	nHosts := len(cfg.BaseUrls)
	if nHosts < 1 {
		nHosts = 1
	}
	tr := http.DefaultTransport.(*http.Transport).Clone()
	tr.MaxIdleConnsPerHost = mw
	tr.MaxIdleConns = mw * nHosts
	if tr.MaxIdleConns < mw {
		tr.MaxIdleConns = mw
	}

	return &Runner{
		cfg:           cfg,
		collector:     collector,
		endpointNames: epNames,
		client: &http.Client{
			Timeout:   requestTimeout,
			Transport: tr,
		},
		resultsCh: make(chan metrics.MetricDetail, 1000),
	}
}

// Run starts the benchmark based on the configuration
func (r *Runner) Run() (*BenchmarkResult, error) {
	log.Printf("Starting benchmark in '%s' mode.", r.cfg.Execution.Mode)

	switch r.cfg.Execution.Mode {
	case "fixed":
		return r.runFixedRPSMode()
	case "ramp":
		return nil, fmt.Errorf("ramp up mode not yet implemented")
	default:
		return nil, fmt.Errorf("unknown mode: %s", r.cfg.Execution.Mode)
	}
}

// runFixedRPSMode executes the benchmark at a fixed number of requests per second
func (r *Runner) runFixedRPSMode() (*BenchmarkResult, error) {
	log.Printf("Running in fixed RPS mode: %d RPS for %d seconds (workers=%d, queue=%d, burst=%d).",
		r.cfg.Execution.RequestsPerSecond, r.cfg.Execution.DurationSeconds,
		r.cfg.Execution.MaxWorkers, r.cfg.Execution.MaxQueueDepth, r.cfg.Execution.RateBurst)

	if r.cfg.Execution.RequestsPerSecond <= 0 {
		return nil, fmt.Errorf("requestsPerSecond must be positive for fixed mode")
	}
	if len(r.cfg.Endpoints) == 0 {
		return nil, fmt.Errorf("no endpoints configured")
	}
	if len(r.cfg.BaseUrls) == 0 {
		return nil, fmt.Errorf("no base URLs configured")
	}

	totalDuration := time.Duration(r.cfg.Execution.DurationSeconds) * time.Second
	ctx, cancel := context.WithTimeout(context.Background(), totalDuration)
	defer cancel()

	maxWorkers := r.cfg.Execution.MaxWorkers
	queueDepth := r.cfg.Execution.MaxQueueDepth
	jobs := make(chan struct{}, queueDepth)
	var dropped atomic.Int64

	collectorWg := sync.WaitGroup{}
	collectorWg.Add(1)
	go func() {
		defer collectorWg.Done()
		for detail := range r.resultsCh {
			r.collector.AppendDetail(detail)
		}
	}()

	var workerWg sync.WaitGroup
	for range maxWorkers {
		workerWg.Add(1)
		go func() {
			defer workerWg.Done()
			for range jobs {
				endpointName, endpoint := r.selectEndpoint()
				baseURL := r.selectBaseURL()
				detail := r.makeRequest(baseURL, endpointName, endpoint)
				r.resultsCh <- detail
			}
		}()
	}

	limiter := rate.NewLimiter(rate.Limit(r.cfg.Execution.RequestsPerSecond), r.cfg.Execution.RateBurst)

sched:
	for {
		if err := limiter.Wait(ctx); err != nil {
			break
		}
		select {
		case jobs <- struct{}{}:
		case <-ctx.Done():
			break sched
		default:
			dropped.Add(1)
		}
	}

	close(jobs)
	workerWg.Wait()
	close(r.resultsCh)
	collectorWg.Wait()

	droppedN := dropped.Load()
	if droppedN > 0 {
		log.Printf("Scheduler dropped %d scheduled requests (queue full; workers could not keep up).", droppedN)
	}
	log.Println("Benchmark duration reached.")

	agg := r.collector.GetResults()
	log.Printf("Fixed RPS mode finished. Total requests attempted: %d", agg.TotalRequests)

	return &BenchmarkResult{
		TotalRequestsMade:        agg.TotalRequests,
		SuccessfulRequests:       agg.SuccessfulRequests,
		FailedRequests:           agg.FailedRequests,
		DroppedDueToBackpressure: droppedN,
		StatusCodes:              agg.StatusCodesCount,
	}, nil
}

// selectEndpoint selects an endpoint based on the configured strategy
func (r *Runner) selectEndpoint() (string, config.EndpointConfig) {
	names := r.endpointNames
	var selectedName string

	switch strings.ToLower(r.cfg.EndpointSelection.Strategy) {
	case "weighted":
		selectedName = r.selectWeightedEndpoint(names)
	case "random":
		selectedName = r.selectRandomEndpoint(names)
	default: // roundRobin
		n := uint64(len(names))
		idx := r.endpointIdx.Add(1) - 1
		selectedName = names[idx%n]
	}

	return selectedName, r.cfg.Endpoints[selectedName]
}

// selectWeightedEndpoint selects an endpoint using weighted selection
func (r *Runner) selectWeightedEndpoint(names []string) string {
	if len(r.cfg.EndpointSelection.Weights) == 0 {
		n := uint64(len(names))
		idx := r.endpointIdx.Add(1) - 1
		return names[idx%n]
	}

	totalWeight := 0.0
	for _, name := range names {
		if weight, exists := r.cfg.EndpointSelection.Weights[name]; exists {
			totalWeight += weight
		}
	}

	if totalWeight == 0 {
		n := uint64(len(names))
		idx := r.endpointIdx.Add(1) - 1
		return names[idx%n]
	}

	ticketSpace := int64(totalWeight * 1000)
	if ticketSpace < 1 {
		ticketSpace = 1
	}
	num, err := rand.Int(rand.Reader, big.NewInt(ticketSpace))
	if err != nil {
		n := uint64(len(names))
		idx := r.endpointIdx.Add(1) - 1
		return names[idx%n]
	}

	target := float64(num.Int64()) / 1000.0
	cumulative := 0.0

	for _, name := range names {
		if weight, exists := r.cfg.EndpointSelection.Weights[name]; exists {
			cumulative += weight
			if target <= cumulative {
				return name
			}
		}
	}

	return names[len(names)-1]
}

// selectRandomEndpoint selects a random endpoint
func (r *Runner) selectRandomEndpoint(names []string) string {
	num, err := rand.Int(rand.Reader, big.NewInt(int64(len(names))))
	if err != nil {
		n := uint64(len(names))
		idx := r.endpointIdx.Add(1) - 1
		return names[idx%n]
	}
	return names[num.Int64()]
}

// selectBaseURL selects a base URL (round-robin for now)
func (r *Runner) selectBaseURL() string {
	n := uint64(len(r.cfg.BaseUrls))
	idx := r.urlIdx.Add(1) - 1
	return r.cfg.BaseUrls[idx%n]
}

// makeRequest creates and executes an HTTP request with parameter generation
func (r *Runner) makeRequest(baseURL, endpointName string, endpoint config.EndpointConfig) metrics.MetricDetail {
	reqStartTime := time.Now()

	// Build the full URL with path parameters
	fullURL, err := r.buildURL(baseURL, endpoint.Path, endpoint.PathParameters)
	if err != nil {
		return r.createErrorMetric(baseURL+endpoint.Path, endpoint.Method,
			fmt.Sprintf("URL building failed: %v", err), reqStartTime)
	}

	// Add query parameters
	if len(endpoint.QueryParameters) > 0 {
		fullURL, err = r.addQueryParams(fullURL, endpoint.QueryParameters)
		if err != nil {
			return r.createErrorMetric(fullURL, endpoint.Method,
				fmt.Sprintf("Query params failed: %v", err), reqStartTime)
		}
	}

	// Generate request body
	var body []byte
	if endpoint.BodyParameters != nil {
		body, err = r.generateRequestBody(endpoint.BodyParameters)
		if err != nil {
			return r.createErrorMetric(fullURL, endpoint.Method,
				fmt.Sprintf("Body generation failed: %v", err), reqStartTime)
		}
	}

	// Create HTTP request
	var req *http.Request
	if body != nil {
		req, err = http.NewRequest(endpoint.Method, fullURL, bytes.NewReader(body))
	} else {
		req, err = http.NewRequest(endpoint.Method, fullURL, nil)
	}

	if err != nil {
		return r.createErrorMetric(fullURL, endpoint.Method,
			fmt.Sprintf("Request creation failed: %v", err), reqStartTime)
	}

	// Set headers
	for name, value := range endpoint.Headers {
		req.Header.Set(name, value)
	}

	// Set default User-Agent if not present
	if req.Header.Get("User-Agent") == "" {
		req.Header.Set("User-Agent", "benchmarking-tool/2.0")
	}

	// Execute request
	resp, err := r.client.Do(req)
	duration := time.Since(reqStartTime)

	if err != nil {
		return r.createErrorMetric(fullURL, endpoint.Method, err.Error(), reqStartTime)
	}
	defer func() {
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
	}()

	return metrics.MetricDetail{
		URL:        fullURL,
		Method:     endpoint.Method,
		StatusCode: resp.StatusCode,
		Duration:   duration,
		IsError:    false,
		ErrorMsg:   "",
		Timestamp:  time.Now(),
	}
}

// buildURL constructs the full URL with path parameters
func (r *Runner) buildURL(baseURL, path string, pathParams map[string]interface{}) (string, error) {
	fullPath := path

	// Replace path parameters
	for paramName, paramDef := range pathParams {
		placeholder := fmt.Sprintf("{%s}", paramName)

		// Get or create parameter generator
		gen, err := r.cfg.GetParameterGenerator(paramDef)
		if err != nil {
			return "", fmt.Errorf("failed to get path parameter generator for %s: %w", paramName, err)
		}

		value, err := gen.Generate()
		if err != nil {
			return "", fmt.Errorf("failed to generate path parameter %s: %w", paramName, err)
		}

		fullPath = strings.ReplaceAll(fullPath, placeholder, fmt.Sprintf("%v", value))
	}

	// Combine base URL and path
	return fmt.Sprintf("%s%s", strings.TrimSuffix(baseURL, "/"), fullPath), nil
}

// addQueryParams adds query parameters to the URL
func (r *Runner) addQueryParams(fullURL string, queryParams map[string]interface{}) (string, error) {
	u, err := url.Parse(fullURL)
	if err != nil {
		return "", fmt.Errorf("failed to parse URL: %w", err)
	}

	query := u.Query()

	for paramName, paramDef := range queryParams {
		gen, err := r.cfg.GetParameterGenerator(paramDef)
		if err != nil {
			return "", fmt.Errorf("failed to get query parameter generator for %s: %w", paramName, err)
		}

		value, err := gen.Generate()
		if err != nil {
			return "", fmt.Errorf("failed to generate query parameter %s: %w", paramName, err)
		}

		query.Set(paramName, fmt.Sprintf("%v", value))
	}

	u.RawQuery = query.Encode()
	return u.String(), nil
}

// generateRequestBody generates the request body based on the body parameters
func (r *Runner) generateRequestBody(bodyParams interface{}) ([]byte, error) {
	gen, err := r.cfg.GetParameterGenerator(bodyParams)
	if err != nil {
		return nil, fmt.Errorf("failed to get body parameter generator: %w", err)
	}

	value, err := gen.Generate()
	if err != nil {
		return nil, fmt.Errorf("failed to generate body: %w", err)
	}

	// Convert to JSON
	return json.Marshal(value)
}

// createErrorMetric creates an error metric detail
func (r *Runner) createErrorMetric(url, method, errorMsg string, startTime time.Time) metrics.MetricDetail {
	return metrics.MetricDetail{
		URL:        url,
		Method:     method,
		StatusCode: 0,
		Duration:   time.Since(startTime),
		IsError:    true,
		ErrorMsg:   errorMsg,
		Timestamp:  time.Now(),
	}
}
