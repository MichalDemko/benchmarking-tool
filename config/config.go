package config

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v2"
)

// ExecutionConfig defines how the benchmark should run
type ExecutionConfig struct {
	Mode              string `yaml:"mode"`              // "fixed" or "ramp"
	DurationSeconds   int    `yaml:"durationSeconds"`   // Total duration for the test
	RequestTimeoutMs  int    `yaml:"requestTimeoutMs"`  // Timeout for individual HTTP requests
	RequestsPerSecond int    `yaml:"requestsPerSecond"` // RPS for fixed mode
}

// ParameterGenerator defines how to generate parameter values
type ParameterGenerator struct {
	Type             string         `yaml:"type"`                       // generator discriminator
	Min              *int           `yaml:"min,omitempty"`              // For randomInt
	Max              *int           `yaml:"max,omitempty"`              // For randomInt
	Format           string         `yaml:"format,omitempty"`           // formattedInt / randomInt+format; timestamp format (unix|rfc3339|iso8601)
	Start            *int           `yaml:"start,omitempty"`            // sequence
	Increment        *int           `yaml:"increment,omitempty"`        // sequence (default 1)
	Precision        *int           `yaml:"precision,omitempty"`        // randomFloat decimal places
	Probability      *float64       `yaml:"probability,omitempty"`      // randomBool P(true)
	MinFloat         *float64       `yaml:"minFloat,omitempty"`         // randomFloat when YAML min is not an int
	MaxFloat         *float64       `yaml:"maxFloat,omitempty"`         // randomFloat
	Values           []any          `yaml:"values,omitempty"`           // For choice type
	Weights          []float64      `yaml:"weights,omitempty"`          // For weighted choice
	Value            any            `yaml:"value,omitempty"`            // For static type
	Length           *int           `yaml:"length,omitempty"`           // For randomString
	Charset          string         `yaml:"charset,omitempty"`          // For randomString
	Template         string         `yaml:"template,omitempty"`         // For template type
	Parameters       map[string]any `yaml:"parameters,omitempty"`       // For template type
	Params           map[string]any `yaml:"params,omitempty"`           // Alias for parameters (template)
	Properties       map[string]any `yaml:"properties,omitempty"`       // For object type
	Fields           map[string]any `yaml:"fields,omitempty"`           // Alias for properties (object)
	MinLength        *int           `yaml:"minLength,omitempty"`        // For array type
	MaxLength        *int           `yaml:"maxLength,omitempty"`        // For array type
	ElementGenerator any            `yaml:"elementGenerator,omitempty"` // For array type
}

// EndpointConfig defines a single API endpoint configuration
type EndpointConfig struct {
	Path            string            `yaml:"path"`
	Method          string            `yaml:"method"`
	Headers         map[string]string `yaml:"headers,omitempty"`
	PathParameters  map[string]any    `yaml:"pathParameters,omitempty"`
	QueryParameters map[string]any    `yaml:"queryParameters,omitempty"`
	BodyParameters  any               `yaml:"bodyParameters,omitempty"`
}

// EndpointSelectionConfig defines how endpoints are selected
type EndpointSelectionConfig struct {
	Strategy string             `yaml:"strategy"` // "weighted", "roundRobin", "random"
	Weights  map[string]float64 `yaml:"weights,omitempty"`
}

// Config holds the complete configuration
type Config struct {
	BaseUrls            []string                      `yaml:"baseUrls"`
	Execution           ExecutionConfig               `yaml:"execution"`
	ParameterGenerators map[string]ParameterGenerator `yaml:"parameterGenerators"`
	Endpoints           map[string]EndpointConfig     `yaml:"endpoints"`
	EndpointSelection   EndpointSelectionConfig       `yaml:"endpointSelection"`
	engine              *ParameterEngine              // Internal engine for parameter generation
}

// LoadConfig loads configuration from a YAML file
func LoadConfig(filePath string) (*Config, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("config file '%s' not found", filePath)
		}
		return nil, fmt.Errorf("failed to read config file '%s': %w", filePath, err)
	}

	var cfg Config
	err = yaml.Unmarshal(data, &cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal config data from '%s': %w", filePath, err)
	}

	// Set defaults
	if cfg.Execution.RequestTimeoutMs == 0 {
		cfg.Execution.RequestTimeoutMs = 5000
	}
	if cfg.Execution.Mode == "" {
		cfg.Execution.Mode = "fixed"
	}
	if cfg.Execution.DurationSeconds == 0 {
		cfg.Execution.DurationSeconds = 60
	}
	if cfg.Execution.RequestsPerSecond == 0 {
		cfg.Execution.RequestsPerSecond = 10
	}
	if cfg.EndpointSelection.Strategy == "" {
		cfg.EndpointSelection.Strategy = "roundRobin"
	}

	// Initialize parameter engine
	cfg.engine = NewParameterEngine()

	// Register named generators for simple references
	for name, genDef := range cfg.ParameterGenerators {
		gen, err := cfg.createGeneratorFromDef(genDef)
		if err != nil {
			return nil, fmt.Errorf("failed to create named generator '%s': %w", name, err)
		}
		cfg.engine.RegisterGenerator(name, gen)
	}

	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return &cfg, nil
}

// Validate checks configuration after defaults and generator registration.
func (c *Config) Validate() error {
	if len(c.BaseUrls) == 0 {
		return fmt.Errorf("baseUrls must not be empty")
	}
	if len(c.Endpoints) == 0 {
		return fmt.Errorf("endpoints must not be empty")
	}
	switch strings.ToLower(c.Execution.Mode) {
	case "fixed":
		if c.Execution.RequestsPerSecond <= 0 {
			return fmt.Errorf("requestsPerSecond must be positive for fixed mode")
		}
	case "ramp":
		return fmt.Errorf("execution.mode %q is not implemented yet", c.Execution.Mode)
	default:
		return fmt.Errorf("unknown execution.mode %q", c.Execution.Mode)
	}
	if c.Execution.DurationSeconds <= 0 {
		return fmt.Errorf("durationSeconds must be positive")
	}
	if c.Execution.RequestTimeoutMs <= 0 {
		return fmt.Errorf("requestTimeoutMs must be positive")
	}
	strat := strings.ToLower(c.EndpointSelection.Strategy)
	switch strat {
	case "weighted", "roundrobin", "random":
	default:
		return fmt.Errorf("unknown endpointSelection.strategy %q", c.EndpointSelection.Strategy)
	}
	if strat == "weighted" {
		for ep, w := range c.EndpointSelection.Weights {
			if _, ok := c.Endpoints[ep]; !ok {
				return fmt.Errorf("endpointSelection.weights: unknown endpoint %q", ep)
			}
			if w < 0 {
				return fmt.Errorf("endpointSelection.weights %q: weight must be non-negative", ep)
			}
		}
	}
	return nil
}

// GetParameterGenerator retrieves a parameter generator by name or creates one from inline definition
func (cfg *Config) GetParameterGenerator(nameOrDef any) (Generator, error) {
	return cfg.engine.createGeneratorWithConfig(nameOrDef, cfg)
}

// createGeneratorFromDef is a helper to create generators from ParameterGenerator structs
func (cfg *Config) createGeneratorFromDef(genDef ParameterGenerator) (Generator, error) {
	// Convert ParameterGenerator struct to a map for the engine
	defMap := make(map[string]any)
	defMap["type"] = genDef.Type

	if genDef.Min != nil {
		defMap["min"] = *genDef.Min
	}
	if genDef.Max != nil {
		defMap["max"] = *genDef.Max
	}
	if genDef.Format != "" {
		defMap["format"] = genDef.Format
	}
	if genDef.Start != nil {
		defMap["start"] = *genDef.Start
	}
	if genDef.Increment != nil {
		defMap["increment"] = *genDef.Increment
	}
	if genDef.Precision != nil {
		defMap["precision"] = *genDef.Precision
	}
	if genDef.Probability != nil {
		defMap["probability"] = *genDef.Probability
	}
	if genDef.MinFloat != nil {
		defMap["min"] = *genDef.MinFloat
	}
	if genDef.MaxFloat != nil {
		defMap["max"] = *genDef.MaxFloat
	}
	if genDef.Values != nil {
		defMap["values"] = genDef.Values
	}
	if genDef.Weights != nil {
		defMap["weights"] = genDef.Weights
	}
	if genDef.Value != nil {
		defMap["value"] = genDef.Value
	}
	if genDef.Length != nil {
		defMap["length"] = *genDef.Length
	}
	if genDef.Charset != "" {
		defMap["charset"] = genDef.Charset
	}
	if genDef.Template != "" {
		defMap["template"] = genDef.Template
	}
	if genDef.Parameters != nil {
		defMap["parameters"] = genDef.Parameters
	}
	if genDef.Params != nil {
		defMap["params"] = genDef.Params
	}
	if genDef.Properties != nil {
		defMap["properties"] = genDef.Properties
	}
	if genDef.Fields != nil {
		defMap["fields"] = genDef.Fields
	}
	if genDef.MinLength != nil {
		defMap["minLength"] = *genDef.MinLength
	}
	if genDef.MaxLength != nil {
		defMap["maxLength"] = *genDef.MaxLength
	}
	if genDef.ElementGenerator != nil {
		defMap["elementGenerator"] = genDef.ElementGenerator
	}

	return cfg.engine.createGeneratorWithConfig(defMap, cfg)
}
