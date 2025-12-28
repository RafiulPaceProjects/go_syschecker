package engine

import (
	"fmt"
	"syschecker/internal/collector"
)

type Category string

const (
	StatusHealthy  = "OK"
	StatusWarning  = "WARN"
	StatusCritical = "CRIT"

	CategoryCPU     Category = "cpu"
	CategoryRAM     Category = "ram"
	CategoryDisk    Category = "disk"
	CategoryNetwork Category = "network"
)

type CheckResult struct {
	Name     string
	Value    float64
	Status   string
	Category Category
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

func Evaluate(stats *collector.RawStats, cfg Config) []CheckResult {
	// Pre-allocate to reduce allocations during append (estimated count ~15-20)
	results := make([]CheckResult, 0, 24)

	results = append(results, checkCPU(stats, cfg)...)
	results = append(results, checkRAM(stats, cfg)...)
	results = append(results, checkDisk(stats, cfg)...)
	results = append(results, checkNetwork(stats, cfg)...)

	return results
}

func checkCPU(stats *collector.RawStats, cfg Config) []CheckResult {
	return []CheckResult{
		{
			Name:     "CPU Usage",
			Value:    stats.CPUUsage,
			Status:   getStatus(stats.CPUUsage, cfg.CPU.Warning, cfg.CPU.Critical),
			Category: CategoryCPU,
		},
	}
}

func checkRAM(stats *collector.RawStats, cfg Config) []CheckResult {
	return []CheckResult{
		{
			Name:     "RAM Usage",
			Value:    stats.RAMUsage,
			Status:   getStatus(stats.RAMUsage, cfg.RAM.Warning, cfg.RAM.Critical),
			Category: CategoryRAM,
		},
	}
}

func checkDisk(stats *collector.RawStats, cfg Config) []CheckResult {
	var results []CheckResult

	// Overall Disk
	diskStatus := getStatus(stats.DiskUsage, cfg.Disk.Warning, cfg.Disk.Critical)
	// Absolute capacity check: Warn if less than 5GB remains, even if % is okay
	freeGB := stats.TotalDisk_GB - uint64(float64(stats.TotalDisk_GB)*(stats.DiskUsage/100))
	if freeGB < 5 && diskStatus == StatusHealthy {
		diskStatus = StatusWarning
	}

	results = append(results, CheckResult{
		Name:     "Disk Usage",
		Value:    stats.DiskUsage,
		Status:   diskStatus,
		Category: CategoryDisk,
	})

	// Partitions
	for _, p := range stats.Partitions {
		pStatus := getStatus(p.UsedPercent, cfg.Disk.Warning, cfg.Disk.Critical)
		pFreeGB := p.TotalGB - uint64(float64(p.TotalGB)*(p.UsedPercent/100))
		if pFreeGB < 5 && pStatus == StatusHealthy {
			pStatus = StatusWarning
		}
		results = append(results, CheckResult{
			Name:     fmt.Sprintf("Partition %s Usage", p.Mountpoint),
			Value:    p.UsedPercent,
			Status:   pStatus,
			Category: CategoryDisk,
		})

		if p.TotalInodes > 0 {
			inodeStatus := getStatus(p.InodeUsage, cfg.Inode.Warning, cfg.Inode.Critical)
			results = append(results, CheckResult{
				Name:     fmt.Sprintf("Partition %s Inodes", p.Mountpoint),
				Value:    p.InodeUsage,
				Status:   inodeStatus,
				Category: CategoryDisk,
			})
		}
	}

	// Overall Inodes
	if stats.TotalInodes > 0 {
		results = append(results, CheckResult{
			Name:     "Inode Usage",
			Value:    stats.InodeUsage,
			Status:   getStatus(stats.InodeUsage, cfg.Inode.Warning, cfg.Inode.Critical),
			Category: CategoryDisk,
		})
	}

	// Disk Health
	for _, h := range stats.DiskHealth {
		healthStatus := StatusHealthy
		switch h.Status {
		case "failed":
			healthStatus = StatusCritical
		case "unknown":
			healthStatus = StatusWarning
		}
		val := 0.0
		if h.Status == "failed" {
			val = 1.0
		}
		results = append(results, CheckResult{
			Name:     fmt.Sprintf("Disk Health %s", h.Device),
			Value:    val,
			Status:   healthStatus,
			Category: CategoryDisk,
		})
	}

	return results
}

func checkNetwork(stats *collector.RawStats, cfg Config) []CheckResult {
	var results []CheckResult

	// Connectivity & Latency
	netStatus := StatusHealthy
	if !stats.IsConnected {
		netStatus = StatusCritical
	} else {
		netStatus = getStatus(stats.NetLatency_ms, cfg.Net.Warning, cfg.Net.Critical)
	}

	results = append(results, CheckResult{
		Name:     "Net Latency",
		Value:    stats.NetLatency_ms,
		Status:   netStatus,
		Category: CategoryNetwork,
	})

	// Interface Errors/Drops
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

		results = append(results, CheckResult{
			Name:     fmt.Sprintf("NIC %s Errors", nic.Name),
			Value:    errTotal,
			Status:   errStatus,
			Category: CategoryNetwork,
		})
		results = append(results, CheckResult{
			Name:     fmt.Sprintf("NIC %s Drops", nic.Name),
			Value:    dropTotal,
			Status:   dropStatus,
			Category: CategoryNetwork,
		})
	}

	// Active TCP
	results = append(results, CheckResult{
		Name:     "Active TCP",
		Value:    float64(stats.ActiveTCP),
		Status:   getStatus(float64(stats.ActiveTCP), cfg.ActiveTCP.Warning, cfg.ActiveTCP.Critical),
		Category: CategoryNetwork,
	})

	return results
}