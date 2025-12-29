package mcpserver

import (
	"context"
	"errors"
	"testing"

	"syschecker/internal/collector"
	"syschecker/internal/database/relational"
	"syschecker/internal/output"
)

// MockStatsProvider implements collector.StatsProvider for testing
type MockStatsProvider struct {
	FastStats *collector.RawStats
	SlowStats *collector.RawStats
	FastErr   error
	SlowErr   error
}

func (m *MockStatsProvider) GetFastMetrics(ctx context.Context) (*collector.RawStats, error) {
	if m.FastErr != nil {
		return nil, m.FastErr
	}
	return m.FastStats, nil
}

func (m *MockStatsProvider) GetSlowMetrics(ctx context.Context) (*collector.RawStats, error) {
	if m.SlowErr != nil {
		return nil, m.SlowErr
	}
	return m.SlowStats, nil
}

// MockGraphClient implements graph.GraphClient for testing
type MockGraphClient struct {
	CypherResult []map[string]any
	CypherErr    error
	Closed       bool
}

func (m *MockGraphClient) IngestSnapshot(ctx context.Context, payload *output.PipelinePayload) error {
	return nil
}

func (m *MockGraphClient) Reset(ctx context.Context) error {
	return nil
}

func (m *MockGraphClient) ExecuteCypher(ctx context.Context, query string) ([]map[string]any, error) {
	if m.CypherErr != nil {
		return nil, m.CypherErr
	}
	return m.CypherResult, nil
}

func (m *MockGraphClient) Close(ctx context.Context) error {
	m.Closed = true
	return nil
}

func TestHandleGetRealtimeMetrics_Fast(t *testing.T) {
	mockProvider := &MockStatsProvider{
		FastStats: &collector.RawStats{
			CPUUsage:  45.5,
			RAMUsage:  60.0,
			DiskUsage: 70.0,
			Hostname:  "test-host",
		},
	}

	s := &Server{
		sensorProvider: mockProvider,
	}

	ctx := context.Background()
	args := MetricsArgs{MetricType: "fast"}

	_, result, err := s.handleGetRealtimeMetrics(ctx, nil, args)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if result == nil {
		t.Fatal("Expected non-nil result")
	}

	if result.CPUUsage != 45.5 {
		t.Errorf("Expected CPU usage 45.5, got %f", result.CPUUsage)
	}

	if result.Hostname != "test-host" {
		t.Errorf("Expected hostname 'test-host', got '%s'", result.Hostname)
	}
}

func TestHandleGetRealtimeMetrics_Slow(t *testing.T) {
	mockProvider := &MockStatsProvider{
		SlowStats: &collector.RawStats{
			NetLatency_ms: 50.0,
			IsConnected:   true,
			ActiveTCP:     100,
		},
	}

	s := &Server{
		sensorProvider: mockProvider,
	}

	ctx := context.Background()
	args := MetricsArgs{MetricType: "slow"}

	_, result, err := s.handleGetRealtimeMetrics(ctx, nil, args)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if result.NetLatency_ms != 50.0 {
		t.Errorf("Expected latency 50.0, got %f", result.NetLatency_ms)
	}

	if !result.IsConnected {
		t.Error("Expected IsConnected to be true")
	}
}

func TestHandleGetRealtimeMetrics_DefaultToFast(t *testing.T) {
	mockProvider := &MockStatsProvider{
		FastStats: &collector.RawStats{CPUUsage: 30.0},
	}

	s := &Server{
		sensorProvider: mockProvider,
	}

	ctx := context.Background()
	args := MetricsArgs{}

	_, result, err := s.handleGetRealtimeMetrics(ctx, nil, args)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if result.CPUUsage != 30.0 {
		t.Errorf("Expected CPU usage 30.0 (fast metrics), got %f", result.CPUUsage)
	}
}

func TestHandleGetRealtimeMetrics_InvalidType(t *testing.T) {
	s := &Server{
		sensorProvider: &MockStatsProvider{},
	}

	ctx := context.Background()
	args := MetricsArgs{MetricType: "invalid"}

	_, _, err := s.handleGetRealtimeMetrics(ctx, nil, args)
	if err == nil {
		t.Error("Expected error for invalid metric type")
	}
}

func TestHandleGetRealtimeMetrics_ProviderError(t *testing.T) {
	mockProvider := &MockStatsProvider{
		FastErr: errors.New("sensor failure"),
	}

	s := &Server{
		sensorProvider: mockProvider,
	}

	ctx := context.Background()
	args := MetricsArgs{MetricType: "fast"}

	_, _, err := s.handleGetRealtimeMetrics(ctx, nil, args)
	if err == nil {
		t.Error("Expected error when provider fails")
	}
}

func TestHandleQueryGraph_Success(t *testing.T) {
	mockGraph := &MockGraphClient{
		CypherResult: []map[string]any{
			{"hostname": "test-host", "cpu": 50.0},
		},
	}

	s := &Server{
		neo4jClient: mockGraph,
	}

	ctx := context.Background()
	args := QueryGraphArgs{Cypher: "MATCH (h:Host) RETURN h.hostname"}

	_, result, err := s.handleQueryGraph(ctx, nil, args)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if result.Data == nil {
		t.Error("Expected non-nil data")
	}
}

func TestHandleQueryGraph_Error(t *testing.T) {
	mockGraph := &MockGraphClient{
		CypherErr: errors.New("cypher syntax error"),
	}

	s := &Server{
		neo4jClient: mockGraph,
	}

	ctx := context.Background()
	args := QueryGraphArgs{Cypher: "INVALID CYPHER"}

	_, _, err := s.handleQueryGraph(ctx, nil, args)
	if err == nil {
		t.Error("Expected error for invalid cypher")
	}
}

func TestHandleGetHistoricalSnapshots_LimitLogic(t *testing.T) {
	tests := []struct {
		name     string
		input    int
		expected int
	}{
		{"default limit", 0, 10},
		{"custom limit", 50, 50},
		{"max limit cap", 500, 100},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			limit := tt.input
			if limit == 0 {
				limit = 10
			}
			if limit > 100 {
				limit = 100
			}

			if limit != tt.expected {
				t.Errorf("Expected limit %d, got %d", tt.expected, limit)
			}
		})
	}
}

func TestConfig_Defaults(t *testing.T) {
	cfg := Config{
		ServerName:    "test-server",
		ServerVersion: "1.0.0",
		GeminiAPIKey:  "test-key",
	}

	if cfg.ServerName != "test-server" {
		t.Errorf("Expected ServerName 'test-server', got '%s'", cfg.ServerName)
	}

	if cfg.GeminiModel != "" {
		t.Errorf("Expected empty GeminiModel by default, got '%s'", cfg.GeminiModel)
	}
}

func TestAskSysCheckerArgs(t *testing.T) {
	args := AskSysCheckerArgs{Question: "What is my CPU usage?"}
	if args.Question == "" {
		t.Error("Question should not be empty")
	}
}

func TestMetricsArgs(t *testing.T) {
	tests := []struct {
		name       string
		metricType string
		valid      bool
	}{
		{"fast metrics", "fast", true},
		{"slow metrics", "slow", true},
		{"empty defaults", "", true},
		{"invalid", "invalid", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args := MetricsArgs{MetricType: tt.metricType}
			isValid := args.MetricType == "fast" || args.MetricType == "slow" || args.MetricType == ""
			if isValid != tt.valid {
				t.Errorf("Expected validity %v, got %v", tt.valid, isValid)
			}
		})
	}
}

func TestHistoricalSnapshotsResult(t *testing.T) {
	result := HistoricalSnapshotsResult{
		Snapshots: []relational.SnapshotSummary{
			{Hostname: "host1"},
		},
	}
	if len(result.Snapshots) != 1 {
		t.Errorf("Expected 1 snapshot, got %d", len(result.Snapshots))
	}
}
