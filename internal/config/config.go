package config

import (
	"encoding/json"
	"os"
)

// RetryConfig describes retry behavior for a source.
type RetryConfig struct {
	MaxAttempts          int   `json:"max_attempts"`
	InitialDelayMs       int   `json:"initial_delay_ms"`
	MaxDelayMs           int   `json:"max_delay_ms"`
	RetryableStatusCodes []int `json:"retryable_status_codes"`
}

// RateLimitConfig describes rate limiting for a source.
type RateLimitConfig struct {
	RequestsPerSecond int `json:"requests_per_second"`
}

// CircuitBreakerConfig describes circuit breaker behavior.
type CircuitBreakerConfig struct {
	FailureThreshold       int `json:"failure_threshold"`
	SuccessThreshold       int `json:"success_threshold"`
	RecoveryTimeoutSeconds int `json:"recovery_timeout_seconds"`
}

// SourceConfig describes one external data source.
type SourceConfig struct {
	Name            string               `json:"name"`
	URL             string               `json:"url"`
	Method          string               `json:"method"`
	Headers         map[string]string    `json:"headers"`
	DefaultParams   map[string]string    `json:"default_params"`
	TimeoutSeconds  int                  `json:"timeout_seconds"`
	CacheTTLSeconds int                  `json:"cache_ttl_seconds"`
	Retry           RetryConfig          `json:"retry"`
	RateLimit       RateLimitConfig      `json:"rate_limit"`
	CircuitBreaker  CircuitBreakerConfig `json:"circuit_breaker"`
	ResponseMapping map[string]string    `json:"response_mapping"`
}

// Config is the root configuration file structure for sources.
type Config struct {
	Sources []SourceConfig `json:"sources"`
}

// TargetConfig describes one publication target: which source to poll,
// which publisher to use, how often and how to render the payload.
type TargetConfig struct {
	Name            string `json:"name"`
	Source          string `json:"source"`
	Publisher       string `json:"publisher"`
	IntervalSeconds int    `json:"interval_seconds"`
	Template        string `json:"template"`
}

// TargetsConfig is the root configuration file structure for targets.
type TargetsConfig struct {
	Targets []TargetConfig `json:"targets"`
}

// Load reads configuration from a JSON file.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cfg Config
	err = json.Unmarshal(data, &cfg)
	if err != nil {
		return nil, err
	}
	return &cfg, nil
}

// LoadTargets reads publication targets from a JSON file.
func LoadTargets(path string) (*TargetsConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cfg TargetsConfig
	err = json.Unmarshal(data, &cfg)
	if err != nil {
		return nil, err
	}
	return &cfg, nil
}
