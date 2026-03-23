package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidate_WeightedUnknownEndpoint(t *testing.T) {
	c := &Config{
		BaseUrls: []string{"http://localhost"},
		Execution: ExecutionConfig{
			Mode:              "fixed",
			DurationSeconds:   10,
			RequestTimeoutMs:  1000,
			RequestsPerSecond: 1,
		},
		Endpoints: map[string]EndpointConfig{
			"a": {Path: "/", Method: "GET"},
		},
		EndpointSelection: EndpointSelectionConfig{
			Strategy: "weighted",
			Weights:  map[string]float64{"b": 1},
		},
	}
	if err := c.Validate(); err == nil || !strings.Contains(err.Error(), "unknown endpoint") {
		t.Fatalf("expected unknown endpoint error, got %v", err)
	}
}

func TestValidate_RampNotImplemented(t *testing.T) {
	c := minimalValidConfig()
	c.Execution.Mode = "ramp"
	if err := c.Validate(); err == nil || !strings.Contains(err.Error(), "not implemented") {
		t.Fatalf("expected ramp not implemented, got %v", err)
	}
}

func TestValidate_UnknownStrategy(t *testing.T) {
	c := minimalValidConfig()
	c.EndpointSelection.Strategy = "invalid"
	if err := c.Validate(); err == nil {
		t.Fatal("expected error for unknown strategy")
	}
}

func TestLoadConfig_TemplateWithStringKeyedParameters(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "cfg.yaml")
	yaml := `
baseUrls:
  - "http://127.0.0.1:9"
execution:
  mode: fixed
  durationSeconds: 1
  requestsPerSecond: 1
  requestTimeoutMs: 1000
parameterGenerators:
  tpl:
    type: "template"
    template: "pre{{x}}suf"
    parameters:
      x:
        type: "static"
        value: "mid"
endpoints:
  ep:
    path: "/"
    method: "GET"
endpointSelection:
  strategy: "roundRobin"
`
	if err := os.WriteFile(path, []byte(yaml), 0600); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	gen, err := cfg.createGeneratorFromDef(cfg.ParameterGenerators["tpl"])
	if err != nil {
		t.Fatal(err)
	}
	v, err := gen.Generate()
	if err != nil {
		t.Fatal(err)
	}
	if v != "premidsuf" {
		t.Fatalf("got %q", v)
	}
}

func minimalValidConfig() *Config {
	return &Config{
		BaseUrls: []string{"http://localhost"},
		Execution: ExecutionConfig{
			Mode:              "fixed",
			DurationSeconds:   10,
			RequestTimeoutMs:  1000,
			RequestsPerSecond: 1,
		},
		Endpoints: map[string]EndpointConfig{
			"a": {Path: "/", Method: "GET"},
		},
		EndpointSelection: EndpointSelectionConfig{Strategy: "roundRobin"},
	}
}
