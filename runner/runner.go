package runner

import (
	"benchmarking-tool/config"
	"benchmarking-tool/metrics"
	"bytes"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"log"
	"math/big"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// Runner executes the benchmark tests with the new configuration format
type Runner struct {
	cfg         *config.Config
	collector   *metrics.Collector
	client      *http.Client
	endpointIdx uint64
	urlIdx      uint64
	resultsCh   chan metrics.MetricDetail
	mu          sync.Mutex
}

// BenchmarkResult holds the outcome of a benchmark run
type BenchmarkResult struct {
	TotalRequestsMade  int64
	SuccessfulRequests int64
	FailedRequests     int64
	RequestTimings     []time.Duration
	StatusCodes        map[int]int64
}

// NewRunner creates a new benchmark runner
func NewRunner(cfg *config.Config, collector *metrics.Collector) *Runner {
	requestTimeout := time.Duration(cfg.Execution.RequestTimeoutMs) * time.Millisecond
	
	return &Runner{
		cfg:         cfg,
		collector:   collector,
		client: &http.Client{
			Timeout: requestTimeout,
		},
		endpointIdx: 0,
		urlIdx:      0,
		resultsCh:   make(chan metrics.MetricDetail, 1000),
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
	log.Printf("Running in fixed RPS mode: %d RPS for %d seconds.", 
		r.cfg.Execution.RequestsPerSecond, r.cfg.Execution.DurationSeconds)
	
	if r.cfg.Execution.RequestsPerSecond <= 0 {
		return nil, fmt.Errorf("requestsPerSecond must be positive for fixed mode")
	}
	if len(r.cfg.Endpoints) == 0 {
		return nil, fmt.Errorf("no endpoints configured")
	}
	if len(r.cfg.BaseUrls) == 0 {
		return nil, fmt.Errorf("no base URLs configured")
	}

	var wg sync.WaitGroup
	startTime := time.Now()
	totalDuration := time.Duration(r.cfg.Execution.DurationSeconds) * time.Second
	ticker := time.NewTicker(time.Second / time.Duration(r.cfg.Execution.RequestsPerSecond))
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
					
					// Select endpoint and base URL
					endpointName, endpoint := r.selectEndpoint()
					baseURL := r.selectBaseURL()
					
					detail := r.makeRequest(baseURL, endpointName, endpoint)
					r.resultsCh <- detail
					requestCount++
				}()
			case <-time.After(totalDuration - time.Since(startTime)):
				log.Println("Benchmark duration reached.")
				break loop
			}
		}
	
	wg.Wait()
	close(r.resultsCh)
	collectorWg.Wait()
	log.Printf("Fixed RPS mode finished. Total requests attempted: %d", requestCount)

	return &BenchmarkResult{}, nil
}

// selectEndpoint selects an endpoint based on the configured strategy
func (r *Runner) selectEndpoint() (string, config.EndpointConfig) {
	r.mu.Lock()
	defer r.mu.Unlock()
	
	var endpointNames []string
	for name := range r.cfg.Endpoints {
		endpointNames = append(endpointNames, name)
	}
	
	var selectedName string
	
	switch r.cfg.EndpointSelection.Strategy {
	case "weighted":
		selectedName = r.selectWeightedEndpoint(endpointNames)
	case "random":
		selectedName = r.selectRandomEndpoint(endpointNames)
	default: // "roundRobin"
		selectedName = endpointNames[r.endpointIdx%uint64(len(endpointNames))]
		r.endpointIdx++
	}
	
	return selectedName, r.cfg.Endpoints[selectedName]
}

// selectWeightedEndpoint selects an endpoint using weighted selection
func (r *Runner) selectWeightedEndpoint(names []string) string {
	if len(r.cfg.EndpointSelection.Weights) == 0 {
		// Fallback to round-robin
		selected := names[r.endpointIdx%uint64(len(names))]
		r.endpointIdx++
		return selected
	}
	
	totalWeight := 0.0
	for _, name := range names {
		if weight, exists := r.cfg.EndpointSelection.Weights[name]; exists {
			totalWeight += weight
		}
	}
	
	if totalWeight == 0 {
		// Fallback to round-robin
		selected := names[r.endpointIdx%uint64(len(names))]
		r.endpointIdx++
		return selected
	}
	
	// Generate random number
	num, err := rand.Int(rand.Reader, big.NewInt(int64(totalWeight*1000)))
	if err != nil {
		// Fallback to round-robin
		selected := names[r.endpointIdx%uint64(len(names))]
		r.endpointIdx++
		return selected
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
	
	// Fallback to last name
	return names[len(names)-1]
}

// selectRandomEndpoint selects a random endpoint
func (r *Runner) selectRandomEndpoint(names []string) string {
	num, err := rand.Int(rand.Reader, big.NewInt(int64(len(names))))
	if err != nil {
		// Fallback to round-robin
		selected := names[r.endpointIdx%uint64(len(names))]
		r.endpointIdx++
		return selected
	}
	return names[num.Int64()]
}

// selectBaseURL selects a base URL (round-robin for now)
func (r *Runner) selectBaseURL() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	
	selected := r.cfg.BaseUrls[r.urlIdx%uint64(len(r.cfg.BaseUrls))]
	r.urlIdx++
	return selected
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
	defer resp.Body.Close()
	
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
