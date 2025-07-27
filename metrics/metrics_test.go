package metrics

import (
	"sync"
	"testing"
	"time"
)

func TestNewCollector(t *testing.T) {
	collector := NewCollector()

	if collector == nil {
		t.Error("NewCollector() should not return nil")
	}

	// Test that it starts with empty results
	results := collector.GetResults()
	if results.TotalRequests != 0 {
		t.Error("New collector should start with 0 total requests")
	}
}

func TestCollector_RecordRequest(t *testing.T) {
	testCases := []struct {
		name                 string
		url                  string
		method               string
		statusCode           int
		duration             time.Duration
		isError              bool
		errorMsg             string
		expectedTotal        int64
		expectedSuccessful   int64
		expectedFailed       int64
		expectedStatusCount  int64
		expectedErrorCount   int
		expectedAvgDuration  time.Duration
	}{
		{
			name:                 "Successful request",
			url:                  "https://api.example.com/test",
			method:               "GET",
			statusCode:           200,
			duration:             100 * time.Millisecond,
			isError:              false,
			errorMsg:             "",
			expectedTotal:        1,
			expectedSuccessful:   1,
			expectedFailed:       0,
			expectedStatusCount:  1,
			expectedAvgDuration:  100 * time.Millisecond,
		},
		{
			name:                 "Request with network error",
			url:                  "https://api.example.com/test",
			method:               "GET",
			statusCode:           0,
			duration:             5 * time.Second,
			isError:              true,
			errorMsg:             "connection timeout",
			expectedTotal:        1,
			expectedSuccessful:   0,
			expectedFailed:       1,
			expectedStatusCount:  1,
			expectedErrorCount:   1,
			expectedAvgDuration:  5 * time.Second,
		},
		{
			name:                 "Request with HTTP error status",
			url:                  "https://api.example.com/notfound",
			method:               "GET",
			statusCode:           404,
			duration:             50 * time.Millisecond,
			isError:              false,
			errorMsg:             "",
			expectedTotal:        1,
			expectedSuccessful:   0,
			expectedFailed:       1,
			expectedStatusCount:  1,
			expectedAvgDuration:  50 * time.Millisecond,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			collector := NewCollector()
			collector.RecordRequest(tc.url, tc.method, tc.statusCode, tc.duration, tc.isError, tc.errorMsg)
			results := collector.GetResults()

			if results.TotalRequests != tc.expectedTotal {
				t.Errorf("Expected %d total requests, got %d", tc.expectedTotal, results.TotalRequests)
			}
			if results.SuccessfulRequests != tc.expectedSuccessful {
				t.Errorf("Expected %d successful requests, got %d", tc.expectedSuccessful, results.SuccessfulRequests)
			}
			if results.FailedRequests != tc.expectedFailed {
				t.Errorf("Expected %d failed requests, got %d", tc.expectedFailed, results.FailedRequests)
			}
			if results.StatusCodesCount[tc.statusCode] != tc.expectedStatusCount {
				t.Errorf("Expected %d requests with status %d, got %d", tc.expectedStatusCount, tc.statusCode, results.StatusCodesCount[tc.statusCode])
			}
			if tc.isError {
				if results.ErrorDetails[tc.errorMsg] != tc.expectedErrorCount {
					t.Errorf("Expected %d '%s' error, got %d", tc.expectedErrorCount, tc.errorMsg, results.ErrorDetails[tc.errorMsg])
				}
			}
			if results.AvgDuration != tc.expectedAvgDuration {
				t.Errorf("Expected average duration %v, got %v", tc.expectedAvgDuration, results.AvgDuration)
			}
		})
	}
}

func TestCollector_RecordMultipleRequests(t *testing.T) {
	collector := NewCollector()

	// Record multiple requests with different outcomes
	testCases := []struct {
		url        string
		method     string
		statusCode int
		duration   time.Duration
		isError    bool
		errorMsg   string
	}{
		{"https://api.example.com/users", "GET", 200, 50 * time.Millisecond, false, ""},
		{"https://api.example.com/posts", "POST", 201, 100 * time.Millisecond, false, ""},
		{"https://api.example.com/error", "GET", 500, 75 * time.Millisecond, false, ""},
		{"https://api.example.com/timeout", "GET", 0, 5 * time.Second, true, "timeout"},
		{"https://api.example.com/users", "GET", 200, 25 * time.Millisecond, false, ""},
	}

	for _, tc := range testCases {
		collector.RecordRequest(tc.url, tc.method, tc.statusCode, tc.duration, tc.isError, tc.errorMsg)
	}

	results := collector.GetResults()

	if results.TotalRequests != 5 {
		t.Errorf("Expected 5 total requests, got %d", results.TotalRequests)
	}

	if results.SuccessfulRequests != 3 {
		t.Errorf("Expected 3 successful requests, got %d", results.SuccessfulRequests)
	}

	if results.FailedRequests != 2 {
		t.Errorf("Expected 2 failed requests, got %d", results.FailedRequests)
	}

	// Check status code counts
	if results.StatusCodesCount[200] != 2 {
		t.Errorf("Expected 2 requests with status 200, got %d", results.StatusCodesCount[200])
	}

	if results.StatusCodesCount[201] != 1 {
		t.Errorf("Expected 1 request with status 201, got %d", results.StatusCodesCount[201])
	}

	if results.StatusCodesCount[500] != 1 {
		t.Errorf("Expected 1 request with status 500, got %d", results.StatusCodesCount[500])
	}

	if results.StatusCodesCount[0] != 1 {
		t.Errorf("Expected 1 request with status 0, got %d", results.StatusCodesCount[0])
	}

	// Check error details
	if results.ErrorDetails["timeout"] != 1 {
		t.Errorf("Expected 1 'timeout' error, got %d", results.ErrorDetails["timeout"])
	}
}

func TestCollector_DurationStatistics(t *testing.T) {
	collector := NewCollector()

	// Add requests with known durations
	durations := []time.Duration{
		25 * time.Millisecond,
		50 * time.Millisecond,
		75 * time.Millisecond,
		100 * time.Millisecond,
		200 * time.Millisecond,
	}

	for _, duration := range durations {
		collector.RecordRequest(
			"https://api.example.com/test",
			"GET",
			200,
			duration,
			false,
			"",
		)
	}

	results := collector.GetResults()

	// Check min duration (25ms)
	expectedMin := 25 * time.Millisecond
	if results.MinDuration != expectedMin {
		t.Errorf("MinDuration = %v, want %v", results.MinDuration, expectedMin)
	}

	// Check max duration (200ms)
	expectedMax := 200 * time.Millisecond
	if results.MaxDuration != expectedMax {
		t.Errorf("MaxDuration = %v, want %v", results.MaxDuration, expectedMax)
	}

	// Check average duration (25+50+75+100+200)/5 = 90ms
	expectedAvg := 90 * time.Millisecond
	if results.AvgDuration != expectedAvg {
		t.Errorf("AvgDuration = %v, want %v", results.AvgDuration, expectedAvg)
	}

	// Check total duration
	expectedTotal := 450 * time.Millisecond
	if results.TotalDuration != expectedTotal {
		t.Errorf("TotalDuration = %v, want %v", results.TotalDuration, expectedTotal)
	}
}

func TestCollector_AppendDetail(t *testing.T) {
	collector := NewCollector()

	detail := MetricDetail{
		URL:        "https://api.example.com/test",
		Method:     "POST",
		StatusCode: 201,
		Duration:   150 * time.Millisecond,
		IsError:    false,
		ErrorMsg:   "",
		Timestamp:  time.Now(),
	}

	collector.AppendDetail(detail)

	results := collector.GetResults()
	if results.TotalRequests != 1 {
		t.Errorf("Expected 1 total request, got %d", results.TotalRequests)
	}

	if results.StatusCodesCount[201] != 1 {
		t.Errorf("Expected 1 request with status 201, got %d", results.StatusCodesCount[201])
	}

	if results.AvgDuration != 150*time.Millisecond {
		t.Errorf("Expected average duration 150ms, got %v", results.AvgDuration)
	}
}

func TestCollector_ConcurrentAccess(t *testing.T) {
	collector := NewCollector()
	numGoroutines := 50
	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	// Test concurrent recording of requests
	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()
			collector.RecordRequest(
				"https://api.example.com/test",
				"GET",
				200,
				time.Duration(id)*time.Millisecond,
				false,
				"",
			)
		}(i)
	}

	wg.Wait()

	results := collector.GetResults()
	if results.TotalRequests != int64(numGoroutines) {
		t.Errorf("Expected %d requests after concurrent recording, got %d", numGoroutines, results.TotalRequests)
	}

	if results.SuccessfulRequests != int64(numGoroutines) {
		t.Errorf("Expected %d successful requests, got %d", numGoroutines, results.SuccessfulRequests)
	}

	if results.FailedRequests != 0 {
		t.Errorf("Expected 0 failed requests, got %d", results.FailedRequests)
	}
}

func TestCollector_GetResults_Race(t *testing.T) {
	collector := NewCollector()
	var wg sync.WaitGroup
	wg.Add(2)

	// Goroutine to continuously record requests
	go func() {
		defer wg.Done()
		for i := 0; i < 100; i++ {
			collector.RecordRequest("url", "GET", 200, 10*time.Millisecond, false, "")
			time.Sleep(1 * time.Millisecond)
		}
	}()

	// Goroutine to continuously get results
	go func() {
		defer wg.Done()
		for i := 0; i < 100; i++ {
			_ = collector.GetResults()
			time.Sleep(1 * time.Millisecond)
		}
	}()

	wg.Wait()
}

func TestCollector_RecordRequest_EdgeCases(t *testing.T) {
	collector := NewCollector()

	// Test case with status code 399 (should be successful)
	collector.RecordRequest("url", "GET", 399, 10*time.Millisecond, false, "")

	results := collector.GetResults()
	if results.SuccessfulRequests != 1 {
		t.Errorf("Expected 1 successful request for status 399, got %d", results.SuccessfulRequests)
	}
	if results.FailedRequests != 0 {
		t.Errorf("Expected 0 failed requests for status 399, got %d", results.FailedRequests)
	}
}

func TestCollector_EmptyResults(t *testing.T) {
	collector := NewCollector()

	results := collector.GetResults()

	if results.TotalRequests != 0 {
		t.Errorf("Expected 0 total requests, got %d", results.TotalRequests)
	}

	if results.SuccessfulRequests != 0 {
		t.Errorf("Expected 0 successful requests, got %d", results.SuccessfulRequests)
	}

	if results.FailedRequests != 0 {
		t.Errorf("Expected 0 failed requests, got %d", results.FailedRequests)
	}

	if results.TotalDuration != 0 {
		t.Errorf("Expected 0 total duration, got %v", results.TotalDuration)
	}

	if results.AvgDuration != 0 {
		t.Errorf("Expected 0 average duration, got %v", results.AvgDuration)
	}

	if results.MinDuration != 0 {
		t.Errorf("Expected 0 min duration, got %v", results.MinDuration)
	}

	if results.MaxDuration != 0 {
		t.Errorf("Expected 0 max duration, got %v", results.MaxDuration)
	}

	if len(results.StatusCodesCount) != 0 {
		t.Errorf("Expected empty status codes map, got %v", results.StatusCodesCount)
	}

	if len(results.ErrorDetails) != 0 {
		t.Errorf("Expected empty error details map, got %v", results.ErrorDetails)
	}
}

func TestMetricDetail_Fields(t *testing.T) {
	timestamp := time.Now()

	detail := MetricDetail{
		URL:        "https://api.example.com/endpoint",
		Method:     "POST",
		StatusCode: 201,
		Duration:   150 * time.Millisecond,
		IsError:    false,
		ErrorMsg:   "",
		Timestamp:  timestamp,
	}

	if detail.URL != "https://api.example.com/endpoint" {
		t.Errorf("URL = %v, want https://api.example.com/endpoint", detail.URL)
	}

	if detail.Method != "POST" {
		t.Errorf("Method = %v, want POST", detail.Method)
	}

	if detail.StatusCode != 201 {
		t.Errorf("StatusCode = %v, want 201", detail.StatusCode)
	}

	if detail.Duration != 150*time.Millisecond {
		t.Errorf("Duration = %v, want 150ms", detail.Duration)
	}

	if detail.IsError != false {
		t.Errorf("IsError = %v, want false", detail.IsError)
	}

	if detail.ErrorMsg != "" {
		t.Errorf("ErrorMsg = %v, want empty string", detail.ErrorMsg)
	}

	if !detail.Timestamp.Equal(timestamp) {
		t.Errorf("Timestamp = %v, want %v", detail.Timestamp, timestamp)
	}
}

func TestMetricDetail_WithError(t *testing.T) {
	timestamp := time.Now()

	detail := MetricDetail{
		URL:        "https://api.example.com/timeout",
		Method:     "GET",
		StatusCode: 0,
		Duration:   5 * time.Second,
		IsError:    true,
		ErrorMsg:   "connection timeout",
		Timestamp:  timestamp,
	}

	if !detail.IsError {
		t.Error("IsError should be true")
	}

	if detail.ErrorMsg != "connection timeout" {
		t.Errorf("ErrorMsg = %v, want 'connection timeout'", detail.ErrorMsg)
	}

	if detail.StatusCode != 0 {
		t.Errorf("StatusCode should be 0 for failed requests, got %v", detail.StatusCode)
	}
}

func TestAggregatedResults_Structure(t *testing.T) {
	var results AggregatedResults

	// Test that the struct can be instantiated
	results.TotalRequests = 100
	results.SuccessfulRequests = 95
	results.FailedRequests = 5
	results.TotalDuration = 10 * time.Second
	results.AvgDuration = 100 * time.Millisecond
	results.MinDuration = 10 * time.Millisecond
	results.MaxDuration = 500 * time.Millisecond
	results.StatusCodesCount = map[int]int64{200: 95, 500: 5}
	results.ErrorDetails = map[string]int{"timeout": 3, "connection refused": 2}

	if results.TotalRequests != 100 {
		t.Error("Failed to set TotalRequests")
	}

	if results.StatusCodesCount[200] != 95 {
		t.Error("Failed to set StatusCodesCount")
	}

	if results.ErrorDetails["timeout"] != 3 {
		t.Error("Failed to set ErrorDetails")
	}
}
