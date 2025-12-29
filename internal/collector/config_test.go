package collector

import (
	"testing"
	"time"
)

func TestDefaultCollectorConfig(t *testing.T) {
	cfg := DefaultCollectorConfig()

	// Check default timeouts
	if cfg.FastMetricsTimeout != 2*time.Second {
		t.Errorf("Expected FastMetricsTimeout 2s, got %v", cfg.FastMetricsTimeout)
	}
	if cfg.SlowMetricsTimeout != 25*time.Second {
		t.Errorf("Expected SlowMetricsTimeout 25s, got %v", cfg.SlowMetricsTimeout)
	}

	// Check network defaults
	if cfg.NetworkCheckEndpoint != "8.8.8.8:53" {
		t.Errorf("Expected NetworkCheckEndpoint '8.8.8.8:53', got '%s'", cfg.NetworkCheckEndpoint)
	}

	// Check polling defaults
	if cfg.FastPollInterval != 1*time.Second {
		t.Errorf("Expected FastPollInterval 1s, got %v", cfg.FastPollInterval)
	}
	if cfg.SlowPollInterval != 30*time.Second {
		t.Errorf("Expected SlowPollInterval 30s, got %v", cfg.SlowPollInterval)
	}

	// Check limits
	if cfg.TopProcessCount != 10 {
		t.Errorf("Expected TopProcessCount 10, got %d", cfg.TopProcessCount)
	}
	if cfg.MaxConsoleLogs != 100 {
		t.Errorf("Expected MaxConsoleLogs 100, got %d", cfg.MaxConsoleLogs)
	}
	if cfg.CPUHistoryCapacity != 31 {
		t.Errorf("Expected CPUHistoryCapacity 31, got %d", cfg.CPUHistoryCapacity)
	}

	// Check feature flags
	if !cfg.EnableDockerMetrics {
		t.Error("Expected EnableDockerMetrics to be true by default")
	}
	if !cfg.EnableDiskHealth {
		t.Error("Expected EnableDiskHealth to be true by default")
	}
}

func TestCollectorConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		cfg     CollectorConfig
		wantErr bool
	}{
		{
			name:    "valid default config",
			cfg:     DefaultCollectorConfig(),
			wantErr: false,
		},
		{
			name: "invalid fast timeout",
			cfg: CollectorConfig{
				FastMetricsTimeout:   0,
				SlowMetricsTimeout:   25 * time.Second,
				NetworkCheckEndpoint: "8.8.8.8:53",
				TopProcessCount:      10,
			},
			wantErr: true,
		},
		{
			name: "invalid slow timeout",
			cfg: CollectorConfig{
				FastMetricsTimeout:   2 * time.Second,
				SlowMetricsTimeout:   0,
				NetworkCheckEndpoint: "8.8.8.8:53",
				TopProcessCount:      10,
			},
			wantErr: true,
		},
		{
			name: "empty network endpoint",
			cfg: CollectorConfig{
				FastMetricsTimeout:   2 * time.Second,
				SlowMetricsTimeout:   25 * time.Second,
				NetworkCheckEndpoint: "",
				TopProcessCount:      10,
			},
			wantErr: true,
		},
		{
			name: "invalid process count",
			cfg: CollectorConfig{
				FastMetricsTimeout:   2 * time.Second,
				SlowMetricsTimeout:   25 * time.Second,
				NetworkCheckEndpoint: "8.8.8.8:53",
				TopProcessCount:      0,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestCollectorConfig_WithMethods(t *testing.T) {
	cfg := DefaultCollectorConfig()

	// Test WithFastTimeout
	newCfg := cfg.WithFastTimeout(5 * time.Second)
	if newCfg.FastMetricsTimeout != 5*time.Second {
		t.Errorf("WithFastTimeout failed, got %v", newCfg.FastMetricsTimeout)
	}
	// Original should be unchanged
	if cfg.FastMetricsTimeout != 2*time.Second {
		t.Error("WithFastTimeout mutated original config")
	}

	// Test WithSlowTimeout
	newCfg = cfg.WithSlowTimeout(60 * time.Second)
	if newCfg.SlowMetricsTimeout != 60*time.Second {
		t.Errorf("WithSlowTimeout failed, got %v", newCfg.SlowMetricsTimeout)
	}

	// Test WithNetworkEndpoint
	newCfg = cfg.WithNetworkEndpoint("1.1.1.1:53")
	if newCfg.NetworkCheckEndpoint != "1.1.1.1:53" {
		t.Errorf("WithNetworkEndpoint failed, got %s", newCfg.NetworkCheckEndpoint)
	}

	// Test WithFastPollInterval
	newCfg = cfg.WithFastPollInterval(500 * time.Millisecond)
	if newCfg.FastPollInterval != 500*time.Millisecond {
		t.Errorf("WithFastPollInterval failed, got %v", newCfg.FastPollInterval)
	}

	// Test WithDockerMetrics
	newCfg = cfg.WithDockerMetrics(false)
	if newCfg.EnableDockerMetrics {
		t.Error("WithDockerMetrics(false) failed")
	}

	// Test WithDiskHealth
	newCfg = cfg.WithDiskHealth(false)
	if newCfg.EnableDiskHealth {
		t.Error("WithDiskHealth(false) failed")
	}
}

func TestConfigError(t *testing.T) {
	err := &ConfigError{
		Field:   "TestField",
		Message: "test message",
	}

	expected := "config error: TestField test message"
	if err.Error() != expected {
		t.Errorf("Expected error '%s', got '%s'", expected, err.Error())
	}
}

func TestCollectorConfig_Chaining(t *testing.T) {
	// Test method chaining
	cfg := DefaultCollectorConfig().
		WithFastTimeout(3 * time.Second).
		WithSlowTimeout(30 * time.Second).
		WithNetworkEndpoint("1.1.1.1:53").
		WithDockerMetrics(false)

	if cfg.FastMetricsTimeout != 3*time.Second {
		t.Errorf("Chained FastMetricsTimeout failed")
	}
	if cfg.SlowMetricsTimeout != 30*time.Second {
		t.Errorf("Chained SlowMetricsTimeout failed")
	}
	if cfg.NetworkCheckEndpoint != "1.1.1.1:53" {
		t.Errorf("Chained NetworkCheckEndpoint failed")
	}
	if cfg.EnableDockerMetrics {
		t.Errorf("Chained EnableDockerMetrics failed")
	}

	// Validate should pass
	if err := cfg.Validate(); err != nil {
		t.Errorf("Chained config should be valid, got error: %v", err)
	}
}
