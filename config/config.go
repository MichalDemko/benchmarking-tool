package config

import (
	"fmt"
	"os" // Changed from io/ioutil

	"gopkg.in/yaml.v2"
)

// Config holds the application configuration
type Config struct {
	Endpoints []EndpointConfig `yaml:"endpoints"`
	Mode      string           `yaml:"mode"` // e.g., "fixed", "ramp"
	// Common settings for all modes
	DurationSeconds    int `yaml:"durationSeconds"`    // Total duration for the test
	RequestTimeoutMs int `yaml:"requestTimeoutMs"` // Timeout for individual HTTP requests in milliseconds

	// Settings for "fixed" mode
	RequestsPerSecond int `yaml:"requestsPerSecond"`

	// Settings for "ramp" mode
	InitialRPS      int `yaml:"initialRps"`
	RampUpPeriodSec int `yaml:"rampUpPeriodSec"` // How often to increase RPS
	RPSIncrement    int `yaml:"rpsIncrement"`    // How much to increase RPS by
	MaxRPS          int `yaml:"maxRps"`          // Optional: stop ramp at this RPS
}

// EndpointConfig defines the configuration for a single API endpoint
type EndpointConfig struct {
	URL    string            `yaml:"url"`
	Method string            `yaml:"method"` // GET, POST, PUT, etc.
	Body   string            `yaml:"body"`   // Optional: request body for POST/PUT
	Headers map[string]string `yaml:"headers"` // Optional: request headers
}

// LoadConfig loads configuration from a file (e.g., YAML, JSON)
// For now, this will return a dummy config.
// TODO: Implement actual file loading and parsing.
func LoadConfig(filePath string) (*Config, error) {
	data, err := os.ReadFile(filePath) // Changed to os.ReadFile
	if err != nil {
		// Try to provide a more helpful error if the default config.yaml is not found
		if os.IsNotExist(err) && filePath == "config.yaml" {
			return nil, fmt.Errorf("config file 'config.yaml' not found in the current directory. Please create one or specify a path")
		}
		return nil, fmt.Errorf("failed to read config file '%s': %w", filePath, err)
	}

	var cfg Config
	err = yaml.Unmarshal(data, &cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal config data from '%s': %w", filePath, err)
	}

	// Set default timeout if not specified in config
	if cfg.RequestTimeoutMs == 0 {
		cfg.RequestTimeoutMs = 10000 // Default to 10 seconds
	}

	return &cfg, nil
}
