package flagger

import (
	"fmt"

	"syschecker/internal/database/relational"
)

// FlaggerService implements relational.StatsFlagger
type FlaggerService struct {
	cfg Config
}

func NewFlaggerService(cfg Config) *FlaggerService {
	return &FlaggerService{cfg: cfg}
}

func (fs *FlaggerService) Flag(s *relational.RawStatsFixed, d *relational.DerivedRates) *relational.SnapshotFlags {
	f := &relational.SnapshotFlags{}
	var explanations []string

	// 1. CPU
	if s.CPUUsagePct > fs.cfg.CPU.Critical {
		f.FlagCPUOverloaded = true
		f.SeverityLevel = 3
		explanations = append(explanations, fmt.Sprintf("CPU critical: %.1f%%", s.CPUUsagePct))
	} else if s.CPUUsagePct > fs.cfg.CPU.Warning {
		f.SeverityLevel = max(f.SeverityLevel, 2)
		explanations = append(explanations, fmt.Sprintf("CPU warning: %.1f%%", s.CPUUsagePct))
	}

	// 2. RAM
	if s.RAMUsagePct > fs.cfg.RAM.Critical {
		f.FlagMemoryPressure = true
		f.SeverityLevel = 3
		explanations = append(explanations, fmt.Sprintf("RAM critical: %.1f%%", s.RAMUsagePct))
	} else if s.RAMUsagePct > fs.cfg.RAM.Warning {
		f.SeverityLevel = max(f.SeverityLevel, 2)
		explanations = append(explanations, fmt.Sprintf("RAM warning: %.1f%%", s.RAMUsagePct))
	}

	// 3. Disk
	if s.DiskUsagePct > fs.cfg.Disk.Critical {
		f.FlagDiskSpaceCritical = true
		f.SeverityLevel = 3
		explanations = append(explanations, fmt.Sprintf("Disk critical: %.1f%%", s.DiskUsagePct))
	} else if s.DiskUsagePct > fs.cfg.Disk.Warning {
		f.SeverityLevel = max(f.SeverityLevel, 2)
		explanations = append(explanations, fmt.Sprintf("Disk warning: %.1f%%", s.DiskUsagePct))
	}

	// 4. Inodes
	if s.InodeUsagePct > fs.cfg.Inode.Critical {
		f.FlagInodeExhaustion = true
		f.SeverityLevel = 3
		explanations = append(explanations, fmt.Sprintf("Inode critical: %.1f%%", s.InodeUsagePct))
	}

	// 5. Network Latency
	if s.NetLatencyMS > fs.cfg.Net.Critical {
		f.FlagNetworkLatencyDegraded = true
		f.SeverityLevel = max(f.SeverityLevel, 2)
		explanations = append(explanations, fmt.Sprintf("High latency: %.1fms", s.NetLatencyMS))
	}

	// 6. Derived Rates Checks (e.g. Disk IO Saturation)
	// Simple heuristic: if read/write bps is very high (arbitrary threshold for now, or from config)
	// For now, just checking if we have rates
	if d.DiskReadBps > 100*1024*1024 { // 100MB/s example
		f.FlagDiskIOSaturation = true
		explanations = append(explanations, "High Disk Read IO")
	}

	// 7. Docker
	if !s.DockerAvailable {
		f.FlagDockerUnavailable = true
		// Not necessarily critical unless expected
	}

	// Aggregate
	if len(explanations) > 0 {
		f.Explanation = explanations[0] // Just take the first one for primary explanation
		if len(explanations) > 1 {
			f.Explanation += fmt.Sprintf(" (+%d more)", len(explanations)-1)
		}
	}

	// Risk Score Calculation (Simple)
	f.RiskScore = f.SeverityLevel * 10
	if f.FlagHostOffline {
		f.RiskScore = 100
	}

	return f
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
