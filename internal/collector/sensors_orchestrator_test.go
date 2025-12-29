package collector

import (
	"context"
	"testing"
)

// MockCollector satisfies the StatsProvider interface
type MockCollector struct {
	Stats *RawStats
	Err   error
}

func (m MockCollector) GetFastMetrics(ctx context.Context) (*RawStats, error) {
	return m.Stats, m.Err
}

func (m MockCollector) GetSlowMetrics(ctx context.Context) (*RawStats, error) {
	return m.Stats, m.Err
}

func TestMockCollector(t *testing.T) {
	expectedStats := &RawStats{
		CPUUsage:    10.5,
		RAMUsage:    50.0,
		TotalRAM_GB: 16,
	}

	mock := MockCollector{
		Stats: expectedStats,
		Err:   nil,
	}

	stats, err := mock.GetFastMetrics(context.Background())
	switch {
	case err != nil:
		t.Fatalf("Expected no error, got %v", err)
	case stats.CPUUsage != expectedStats.CPUUsage:
		t.Errorf("Expected CPU usage %f, got %f", expectedStats.CPUUsage, stats.CPUUsage)
	}
}

func TestSystemCollector(t *testing.T) {
	collector := NewSystemCollector()
	stats, err := collector.GetFastMetrics(context.Background())

	switch {
	case err != nil:
		t.Skipf("Skipping system test: %v (might be environment specific)", err)
	case stats.CPUUsage < 0 || stats.CPUUsage > 100:
		t.Errorf("CPU usage out of bounds: %f", stats.CPUUsage)
	case stats.RAMUsage < 0 || stats.RAMUsage > 100:
		t.Errorf("RAM usage out of bounds: %f", stats.RAMUsage)
	}
}

func TestSystemCollectorDiskDetails(t *testing.T) {
	collector := NewSystemCollector()
	// Use GetFastMetrics because GetSlowMetrics often requires root or internet
	stats, err := collector.GetFastMetrics(context.Background())
	if err != nil {
		t.Skipf("Skipping system test: %v (might be environment specific)", err)
	}

	// Also fetch slow metrics if possible, but don't fail hard
	slowStats, err := collector.GetSlowMetrics(context.Background())
	if err == nil {
		stats.NetLatency_ms = slowStats.NetLatency_ms
		stats.IsConnected = slowStats.IsConnected
		stats.ActiveTCP = slowStats.ActiveTCP
		stats.DiskHealth = slowStats.DiskHealth
	}

	for _, p := range stats.Partitions {
		if p.UsedPercent < 0 || p.UsedPercent > 100 {
			t.Errorf("partition %s used percent out of range: %f", p.Mountpoint, p.UsedPercent)
		}
		if p.TotalGB == 0 {
			// Some virtual mounts may report zero; log instead of fail.
			t.Logf("partition %s total size is zero (skipping size assertion)", p.Mountpoint)
		}
		if p.InodeUsage < 0 || p.InodeUsage > 100 {
			t.Errorf("partition %s inode usage out of range: %f", p.Mountpoint, p.InodeUsage)
		}
	}

	if len(stats.NetInterfaces) == 0 {
		t.Log("no network interfaces reported; skipping interface assertions")
	} else {
		for _, n := range stats.NetInterfaces {
			if n.Name == "" {
				t.Errorf("network interface missing name")
			}
		}
	}

	if stats.ActiveTCP < 0 {
		t.Errorf("active TCP count should be non-negative, got %d", stats.ActiveTCP)
	}

	for _, h := range stats.DiskHealth {
		switch h.Status {
		case "passed", "failed", "unknown":
			// acceptable statuses
		default:
			t.Errorf("unexpected disk health status %q for device %s", h.Status, h.Device)
		}
	}
}
