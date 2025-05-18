package metrics

import (
	"sync"
	"time"
)

// MetricDetail holds detailed information for a single request
type MetricDetail struct {
	URL        string
	Method     string
	StatusCode int
	Duration   time.Duration
	IsError    bool
	ErrorMsg   string
	Timestamp  time.Time
}

// Collector stores benchmark metrics
type Collector struct {
	mutex         sync.Mutex
	allRequests   []MetricDetail // Stores all individual request details
}

// NewCollector creates a new metrics collector
func NewCollector() *Collector {
	return &Collector{
		allRequests: make([]MetricDetail, 0),
	}
}

// RecordRequest records the result of a single HTTP request
func (c *Collector) RecordRequest(url string, method string, statusCode int, duration time.Duration, isError bool, errorMsg string) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	c.allRequests = append(c.allRequests, MetricDetail{
		URL:        url,
		Method:     method,
		StatusCode: statusCode,
		Duration:   duration,
		IsError:    isError,
		ErrorMsg:   errorMsg,
		Timestamp:  time.Now(),
	})
}

// AppendDetail appends a MetricDetail to the collector (called from a single goroutine)
func (c *Collector) AppendDetail(detail MetricDetail) {
	c.allRequests = append(c.allRequests, detail)
}

// AggregatedResults provides a summary of all collected metrics.
// This is where you would calculate averages, percentiles, error rates, etc.
type AggregatedResults struct {
	TotalRequests      int64
	SuccessfulRequests int64
	FailedRequests     int64
	TotalDuration      time.Duration // Sum of all request durations
	AvgDuration        time.Duration
	MinDuration        time.Duration
	MaxDuration        time.Duration
	StatusCodesCount   map[int]int64 // Counts per status code
	ErrorDetails       map[string]int  // Count of specific error messages
	// TODO: Add latencies (p50, p90, p95, p99)
	// TODO: Add RPS achieved
}

// GetResults processes the collected metrics and returns an aggregated summary.
func (c *Collector) GetResults() AggregatedResults {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	if len(c.allRequests) == 0 {
		return AggregatedResults{ // Return empty/zeroed struct if no requests
			StatusCodesCount: make(map[int]int64),
			ErrorDetails:     make(map[string]int),
		}
	}

	var res AggregatedResults
	res.StatusCodesCount = make(map[int]int64)
	res.ErrorDetails = make(map[string]int)
	res.MinDuration = c.allRequests[0].Duration // Initialize with the first request

	for _, r := range c.allRequests {
		res.TotalRequests++
		res.TotalDuration += r.Duration

		if r.IsError || r.StatusCode >= 400 {
			res.FailedRequests++
			if r.ErrorMsg != "" {
				res.ErrorDetails[r.ErrorMsg]++
			}
		} else {
			res.SuccessfulRequests++
		}

		res.StatusCodesCount[r.StatusCode]++

		if r.Duration < res.MinDuration {
			res.MinDuration = r.Duration
		}
		if r.Duration > res.MaxDuration {
			res.MaxDuration = r.Duration
		}
	}

	if res.TotalRequests > 0 {
		res.AvgDuration = res.TotalDuration / time.Duration(res.TotalRequests)
	} else {
		res.MinDuration = 0 // Avoid returning the initial large value if no requests
	}

	return res
}
