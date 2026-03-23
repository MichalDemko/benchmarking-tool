package main

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestRun_MissingConfigFile(t *testing.T) {
	p := filepath.Join(t.TempDir(), "does-not-exist.yaml")
	err := run([]string{"benchmarking-tool", p})
	if err == nil {
		t.Fatal("expected error for missing config")
	}
	if !strings.Contains(err.Error(), "failed to load") {
		t.Fatalf("unexpected error: %v", err)
	}
}
