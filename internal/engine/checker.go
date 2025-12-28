package engine

import (
	"fmt"
	"syschecker/internal/collector"
)

const (
	StatusHealthy  = "OK"
	StatusWarning  = "WARN"
	StatusCritical = "CRIT"

	CPUWarningThreshold    = 70.0
	CPUCriticalThreshold   = 90.0
	RAMWarningThreshold    = 70.0
	RAMCriticalThreshold   = 90.0
	DiskWarningThreshold   = 80.0
	DiskCriticalThreshold  = 90.0
	InodeWarningThreshold  = 80.0
	InodeCriticalThreshold = 90.0
	NetWarningThreshold    = 150.0 // ms
	NetCriticalThreshold   = 500.0 // ms
	ActiveTCPWarning       = 200.0
	ActiveTCPCritical      = 500.0
)

type CheckResult struct {
	Name   string
	Value  float64
	Status string
}

func getStatus(value, warning, critical float64) string {
	if value > critical {
		return StatusCritical
	}
	if value > warning {
		return StatusWarning
	}
	return StatusHealthy
}

func Evaluate(stats collector.RawStats) []CheckResult {
	var result []CheckResult

	// CPU
	result = append(result, CheckResult{
		Name:   "CPU Usage",
		Value:  stats.CPUUsage,
		Status: getStatus(stats.CPUUsage, CPUWarningThreshold, CPUCriticalThreshold),
	})

	// RAM
	result = append(result, CheckResult{
		Name:   "RAM Usage",
		Value:  stats.RAMUsage,
		Status: getStatus(stats.RAMUsage, RAMWarningThreshold, RAMCriticalThreshold),
	})

	// Disk
	diskStatus := getStatus(stats.DiskUsage, DiskWarningThreshold, DiskCriticalThreshold)

	// Absolute capacity check: Warn if less than 5GB remains, even if % is okay
	freeGB := stats.TotalDisk_GB - uint64(float64(stats.TotalDisk_GB)*(stats.DiskUsage/100))
	if freeGB < 5 && diskStatus == StatusHealthy {
		diskStatus = StatusWarning
	}

	result = append(result, CheckResult{
		Name:   "Disk Usage",
		Value:  stats.DiskUsage,
		Status: diskStatus,
	})

	// Per-partition disk usage (best-effort)
	for _, p := range stats.Partitions {
		pStatus := getStatus(p.UsedPercent, DiskWarningThreshold, DiskCriticalThreshold)
		freeGB := p.TotalGB - uint64(float64(p.TotalGB)*(p.UsedPercent/100))
		if freeGB < 5 && pStatus == StatusHealthy {
			pStatus = StatusWarning
		}
		result = append(result, CheckResult{
			Name:   fmt.Sprintf("Partition %s Usage", p.Mountpoint),
			Value:  p.UsedPercent,
			Status: pStatus,
		})
		// Inode pressure per-partition if available
		if p.TotalInodes > 0 {
			inodeStatus := getStatus(p.InodeUsage, InodeWarningThreshold, InodeCriticalThreshold)
			result = append(result, CheckResult{
				Name:   fmt.Sprintf("Partition %s Inodes", p.Mountpoint),
				Value:  p.InodeUsage,
				Status: inodeStatus,
			})
		}
	}

	// Inodes (only if available)
	if stats.TotalInodes > 0 {
		result = append(result, CheckResult{
			Name:   "Inode Usage",
			Value:  stats.InodeUsage,
			Status: getStatus(stats.InodeUsage, InodeWarningThreshold, InodeCriticalThreshold),
		})
	}

	// Network Check
	netStatus := StatusHealthy
	if !stats.IsConnected {
		netStatus = StatusCritical
	} else {
		netStatus = getStatus(stats.NetLatency_ms, NetWarningThreshold, NetCriticalThreshold)
	}

	result = append(result, CheckResult{
		Name:   "Net Latency",
		Value:  stats.NetLatency_ms,
		Status: netStatus,
	})

	// Per-interface packet errors/drops (best-effort)
	for _, nic := range stats.NetInterfaces {
		errTotal := float64(nic.ErrIn + nic.ErrOut)
		dropTotal := float64(nic.DropIn + nic.DropOut)

		errStatus := StatusHealthy
		if errTotal > 0 {
			errStatus = StatusWarning
		}

		dropStatus := StatusHealthy
		if dropTotal > 0 {
			dropStatus = StatusWarning
		}

		result = append(result, CheckResult{
			Name:   fmt.Sprintf("NIC %s Errors", nic.Name),
			Value:  errTotal,
			Status: errStatus,
		})
		result = append(result, CheckResult{
			Name:   fmt.Sprintf("NIC %s Drops", nic.Name),
			Value:  dropTotal,
			Status: dropStatus,
		})
	}

	// Active TCP connections (best-effort)
	result = append(result, CheckResult{
		Name:   "Active TCP",
		Value:  float64(stats.ActiveTCP),
		Status: getStatus(float64(stats.ActiveTCP), ActiveTCPWarning, ActiveTCPCritical),
	})

	// Disk health (SMART) best-effort
	for _, h := range stats.DiskHealth {
		healthStatus := StatusHealthy
		switch h.Status {
		case "failed":
			healthStatus = StatusCritical
		case "unknown":
			healthStatus = StatusWarning
		default:
			healthStatus = StatusHealthy
		}
		value := 0.0
		if h.Status == "failed" {
			value = 1.0
		}
		result = append(result, CheckResult{
			Name:   fmt.Sprintf("Disk Health %s", h.Device),
			Value:  value,
			Status: healthStatus,
		})
	}

	return result
}
