package collector

import "time"

// CollectorConfig contains configurable parameters for the system collector.
// Use DefaultCollectorConfig() to get sensible defaults, then override as needed.
type CollectorConfig struct {
	// Timeout settings
	FastMetricsTimeout time.Duration // Timeout for fast metrics collection (default: 2s)
	SlowMetricsTimeout time.Duration // Timeout for slow metrics collection (default: 25s)

	// Network check settings
	NetworkCheckEndpoint string        // Endpoint for network latency check (default: "8.8.8.8:53")
	NetworkCheckTimeout  time.Duration // Timeout for network check (default: 3s)

	// Polling intervals (for workers/TUI)
	FastPollInterval time.Duration // How often to poll fast metrics (default: 1s)
	SlowPollInterval time.Duration // How often to poll slow metrics (default: 30s)

	// Collection limits
	TopProcessCount    int // Number of top processes to collect (default: 10)
	MaxConsoleLogs     int // Maximum console log entries to retain (default: 100)
	CPUHistoryCapacity int // Capacity for CPU history buffer (default: 31)

	// Feature flags
	EnableDockerMetrics  bool // Whether to collect Docker metrics (default: true)
	EnableDiskHealth     bool // Whether to collect disk health via smartctl (default: true)
	EnableTemperatures   bool // Whether to collect temperature sensors (default: true)
	EnableProcessMetrics bool // Whether to collect process metrics (default: true)
}

// DefaultCollectorConfig returns a CollectorConfig with sensible defaults.
func DefaultCollectorConfig() CollectorConfig {
	return CollectorConfig{
		// Timeouts
		FastMetricsTimeout: 2 * time.Second,
		SlowMetricsTimeout: 25 * time.Second,

		// Network
		NetworkCheckEndpoint: "8.8.8.8:53",
		NetworkCheckTimeout:  3 * time.Second,

		// Polling
		FastPollInterval: 1 * time.Second,
		SlowPollInterval: 30 * time.Second,

		// Limits
		TopProcessCount:    10,
		MaxConsoleLogs:     100,
		CPUHistoryCapacity: 31,

		// Features (all enabled by default)
		EnableDockerMetrics:  true,
		EnableDiskHealth:     true,
		EnableTemperatures:   true,
		EnableProcessMetrics: true,
	}
}

// WithFastTimeout returns a copy of the config with modified fast timeout.
func (c CollectorConfig) WithFastTimeout(d time.Duration) CollectorConfig {
	c.FastMetricsTimeout = d
	return c
}

// WithSlowTimeout returns a copy of the config with modified slow timeout.
func (c CollectorConfig) WithSlowTimeout(d time.Duration) CollectorConfig {
	c.SlowMetricsTimeout = d
	return c
}

// WithNetworkEndpoint returns a copy of the config with modified network check endpoint.
func (c CollectorConfig) WithNetworkEndpoint(endpoint string) CollectorConfig {
	c.NetworkCheckEndpoint = endpoint
	return c
}

// WithFastPollInterval returns a copy of the config with modified fast poll interval.
func (c CollectorConfig) WithFastPollInterval(d time.Duration) CollectorConfig {
	c.FastPollInterval = d
	return c
}

// WithSlowPollInterval returns a copy of the config with modified slow poll interval.
func (c CollectorConfig) WithSlowPollInterval(d time.Duration) CollectorConfig {
	c.SlowPollInterval = d
	return c
}

// WithDockerMetrics returns a copy of the config with Docker metrics enabled/disabled.
func (c CollectorConfig) WithDockerMetrics(enabled bool) CollectorConfig {
	c.EnableDockerMetrics = enabled
	return c
}

// WithDiskHealth returns a copy of the config with disk health collection enabled/disabled.
func (c CollectorConfig) WithDiskHealth(enabled bool) CollectorConfig {
	c.EnableDiskHealth = enabled
	return c
}

// Validate checks if the configuration is valid and returns an error if not.
func (c CollectorConfig) Validate() error {
	if c.FastMetricsTimeout <= 0 {
		return &ConfigError{Field: "FastMetricsTimeout", Message: "must be positive"}
	}
	if c.SlowMetricsTimeout <= 0 {
		return &ConfigError{Field: "SlowMetricsTimeout", Message: "must be positive"}
	}
	if c.NetworkCheckEndpoint == "" {
		return &ConfigError{Field: "NetworkCheckEndpoint", Message: "must not be empty"}
	}
	if c.TopProcessCount <= 0 {
		return &ConfigError{Field: "TopProcessCount", Message: "must be positive"}
	}
	return nil
}

// ConfigError represents a configuration validation error.
type ConfigError struct {
	Field   string
	Message string
}

func (e *ConfigError) Error() string {
	return "config error: " + e.Field + " " + e.Message
}
