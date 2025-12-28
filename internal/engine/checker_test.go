package engine

import (
	"syschecker/internal/collector"
	"testing"
)

func TestEvaluate(t *testing.T) {
	tests := []struct {
		name     string
		stats    *collector.RawStats
		expected map[string]string // Metric Name -> Expected Status
	}{
		{
			name: "All Healthy",
			stats: &collector.RawStats{
				CPUUsage:     10.0,
				RAMUsage:     20.0,
				DiskUsage:    30.0,
				TotalDisk_GB: 100,
				TotalInodes:  1000,
				InodeUsage:   10.0,
			},
			expected: map[string]string{
				"CPU Usage":   StatusHealthy,
				"RAM Usage":   StatusHealthy,
				"Disk Usage":  StatusHealthy,
				"Inode Usage": StatusHealthy,
			},
		},
		{
			name: "CPU Critical",
			stats: &collector.RawStats{
				CPUUsage:     95.0,
				RAMUsage:     20.0,
				DiskUsage:    30.0,
				TotalDisk_GB: 100,
			},
			expected: map[string]string{
				"CPU Usage": StatusCritical,
			},
		},
		{
			name: "RAM Warning",
			stats: &collector.RawStats{
				CPUUsage:     10.0,
				RAMUsage:     75.0,
				DiskUsage:    30.0,
				TotalDisk_GB: 100,
			},
			expected: map[string]string{
				"RAM Usage": StatusWarning,
			},
		},
		{
			name: "Disk Absolute Capacity Warning (<5GB free)",
			stats: &collector.RawStats{
				CPUUsage:     10.0,
				RAMUsage:     20.0,
				DiskUsage:    10.0, // 10% of 4GB is 0.4GB used, 3.6GB free
				TotalDisk_GB: 4,
			},
			expected: map[string]string{
				"Disk Usage": StatusWarning,
			},
		},
		{
			name: "Inode Critical",
			stats: &collector.RawStats{
				CPUUsage:    10.0,
				RAMUsage:    20.0,
				DiskUsage:   30.0,
				TotalInodes: 1000,
				InodeUsage:  95.0,
			},
			expected: map[string]string{
				"Inode Usage": StatusCritical,
			},
		},
		{
			name: "Skip Inodes when TotalInodes is 0",
			stats: &collector.RawStats{
				CPUUsage:    10.0,
				TotalInodes: 0,
			},
			expected: map[string]string{
				"CPU Usage": StatusHealthy,
			},
		},
		{
			name: "Partition Warning and Inode Healthy",
			stats: &collector.RawStats{
				CPUUsage:   10.0,
				RAMUsage:   10.0,
				DiskUsage:  10.0,
				Partitions: []collector.PartitionUsage{{Mountpoint: "/", UsedPercent: 85.0, TotalGB: 100, InodeUsage: 10.0, TotalInodes: 1000}},
			},
			expected: map[string]string{
				"Partition / Usage":  StatusWarning,
				"Partition / Inodes": StatusHealthy,
			},
		},
		{
			name: "Active TCP Critical",
			stats: &collector.RawStats{
				CPUUsage:  10.0,
				RAMUsage:  10.0,
				DiskUsage: 10.0,
				ActiveTCP: 600,
			},
			expected: map[string]string{
				"Active TCP": StatusCritical,
			},
		},
		{
			name: "NIC Errors Warning",
			stats: &collector.RawStats{
				NetInterfaces: []collector.NetInterfaceStats{{Name: "eth0", ErrIn: 1, ErrOut: 0, DropIn: 0, DropOut: 0}},
			},
			expected: map[string]string{
				"NIC eth0 Errors": StatusWarning,
				"NIC eth0 Drops":  StatusHealthy,
			},
		},
		{
			name: "Disk Health Failed",
			stats: &collector.RawStats{
				DiskHealth: []collector.DiskHealthInfo{{Device: "/dev/sda", Status: "failed"}},
			},
			expected: map[string]string{
				"Disk Health /dev/sda": StatusCritical,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results := Evaluate(tt.stats, DefaultConfig())

			// Check if we got the expected number of results for the "Skip Inodes" case
			if tt.name == "Skip Inodes when TotalInodes is 0" {
				for _, res := range results {
					if res.Name == "Inode Usage" {
						t.Errorf("%s: Inode Usage should have been skipped", tt.name)
					}
				}
			}

			// Verify statuses
			for _, res := range results {
				if want, ok := tt.expected[res.Name]; ok {
					if res.Status != want {
						t.Errorf("%s: for %s expected %s, got %s (Value: %.2f)", tt.name, res.Name, want, res.Status, res.Value)
					}
				}
			}
		})
	}
}
