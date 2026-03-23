package runner

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"benchmarking-tool/config"
	"benchmarking-tool/metrics"
)

func TestRunFixedRPS_Integration(t *testing.T) {
	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}))
	defer srv.Close()

	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "bench.yaml")
	yaml := `baseUrls:
  - "` + srv.URL + `"
execution:
  mode: fixed
  durationSeconds: 1
  requestsPerSecond: 8
  requestTimeoutMs: 2000
endpoints:
  root:
    path: "/"
    method: GET
endpointSelection:
  strategy: roundRobin
`
	if err := os.WriteFile(cfgPath, []byte(yaml), 0600); err != nil {
		t.Fatal(err)
	}

	cfg, err := config.LoadConfig(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	col := metrics.NewCollector()
	r := NewRunner(cfg, col)
	res, err := r.Run()
	if err != nil {
		t.Fatal(err)
	}
	if hits < 1 {
		t.Fatalf("server got no requests, hits=%d", hits)
	}
	agg := col.GetResults()
	if agg.TotalRequests < 1 {
		t.Fatalf("expected requests in collector, got %d", agg.TotalRequests)
	}
	if res.TotalRequestsMade != agg.TotalRequests {
		t.Fatalf("BenchmarkResult total %d vs collector %d", res.TotalRequestsMade, agg.TotalRequests)
	}
}
