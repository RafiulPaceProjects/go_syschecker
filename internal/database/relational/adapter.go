package relational

import (
	"time"

	"syschecker/internal/collector"
)

// =============================================================================
// ADAPTER FUNCTIONS
// =============================================================================

// ToRawStatsFixed converts collector.RawStats to ORM-compatible RawStatsFixed.
// The agentID, machineID, bootID should be provided by the caller.
func ToRawStatsFixed(cs *collector.RawStats, kind SnapshotKind, agentID, machineID, bootID string) RawStatsFixed {
	now := time.Now()

	// Convert partitions
	partitions := make([]PartitionUsageFixed, 0, len(cs.Partitions))
	for _, p := range cs.Partitions {
		partitions = append(partitions, PartitionUsageFixed{
			Mountpoint:  p.Mountpoint,
			Device:      p.Device,
			Fstype:      p.Fstype,
			UsedPercent: p.UsedPercent,
			TotalBytes:  p.TotalGB * 1024 * 1024 * 1024, // GB to Bytes
			InodeUsage:  p.InodeUsage,
			TotalInodes: p.TotalInodes,
		})
	}

	// Convert IO counters
	ioCounters := make([]DiskIOCountersFixed, 0, len(cs.IOCounters))
	for _, io := range cs.IOCounters {
		ioCounters = append(ioCounters, DiskIOCountersFixed{
			Device:      io.Device,
			ReadBytes:   io.ReadBytes,
			WriteBytes:  io.WriteBytes,
			ReadCount:   io.ReadCount,
			WriteCount:  io.WriteCount,
			ReadTimeMS:  io.ReadTimeMS,
			WriteTimeMS: io.WriteTimeMS,
		})
	}

	// Convert disk health
	diskHealth := make([]DiskHealthInfoFixed, 0, len(cs.DiskHealth))
	for _, h := range cs.DiskHealth {
		diskHealth = append(diskHealth, DiskHealthInfoFixed{
			Device:  h.Device,
			Status:  h.Status,
			Message: h.Message,
		})
	}

	// Convert net interfaces
	netInterfaces := make([]NetInterfaceStatsFixed, 0, len(cs.NetInterfaces))
	for _, ni := range cs.NetInterfaces {
		netInterfaces = append(netInterfaces, NetInterfaceStatsFixed{
			Name:        ni.Name,
			BytesSent:   ni.BytesSent,
			BytesRecv:   ni.BytesRecv,
			PacketsSent: ni.PacketsSent,
			PacketsRecv: ni.PacketsRecv,
			ErrIn:       ni.ErrIn,
			ErrOut:      ni.ErrOut,
			DropIn:      ni.DropIn,
			DropOut:     ni.DropOut,
		})
	}

	// Convert docker containers
	dockerContainers := make([]DockerContainerInfoFixed, 0, len(cs.DockerContainers))
	for _, c := range cs.DockerContainers {
		dockerContainers = append(dockerContainers, DockerContainerInfoFixed{
			ID:            c.ID,
			Name:          c.Name,
			Image:         c.Image,
			Status:        c.Status,
			Running:       c.Running,
			CPUUsagePct:   c.CPUUsage,
			MemUsageBytes: c.MemUsage,
			MemLimitBytes: c.MemLimit,
			MemPercent:    c.MemPercent,
		})
	}

	// Convert temperatures
	temps := make([]TemperatureStatFixed, 0, len(cs.Temperatures))
	for _, t := range cs.Temperatures {
		temps = append(temps, TemperatureStatFixed{
			SensorKey:    t.SensorKey,
			TemperatureC: t.Temperature,
		})
	}

	// Convert top processes
	procs := make([]ProcessStatFixed, 0, len(cs.TopProcesses))
	for i, p := range cs.TopProcesses {
		procs = append(procs, ProcessStatFixed{
			Rank:   i + 1,
			PID:    p.PID,
			Name:   p.Name,
			CPUPct: p.CPU,
			MemPct: p.Memory,
		})
	}

	return RawStatsFixed{
		CollectedAt: now,
		Kind:        kind,
		AgentID:     agentID,
		MachineID:   machineID,
		BootID:      bootID,
		Hostname:    cs.Hostname,

		CPUUsagePct:     cs.CPUUsage,
		CPUPerCorePct:   cs.CPUPerCore,
		LoadAvg1:        cs.LoadAvg1,
		LoadAvg5:        cs.LoadAvg5,
		LoadAvg15:       cs.LoadAvg15,
		CPUModel:        cs.CPUModel,
		CPUCoresLogical: cs.CPUCores,

		RAMUsagePct:       cs.RAMUsage,
		RAMTotalBytes:     cs.TotalRAM_GB * 1024 * 1024 * 1024,
		RAMAvailableBytes: cs.RAMAvailable * 1024 * 1024 * 1024,
		RAMUsedBytes:      cs.RAMUsed * 1024 * 1024 * 1024,
		RAMFreeBytes:      cs.RAMFree * 1024 * 1024 * 1024,
		RAMCachedBytes:    cs.RAMCached * 1024 * 1024 * 1024,
		RAMBufferedBytes:  cs.RAMBuffered * 1024 * 1024 * 1024,

		SwapUsagePct:   cs.SwapUsage,
		SwapTotalBytes: cs.SwapTotal * 1024 * 1024 * 1024,
		SwapUsedBytes:  cs.SwapUsed * 1024 * 1024 * 1024,

		DiskUsagePct:   cs.DiskUsage,
		DiskTotalBytes: cs.TotalDisk_GB * 1024 * 1024 * 1024,
		InodeUsagePct:  cs.InodeUsage,
		InodeTotal:     cs.TotalInodes,

		Partitions: partitions,
		IOCounters: ioCounters,
		DiskHealth: diskHealth,

		NetLatencyMS:  cs.NetLatency_ms,
		IsConnected:   cs.IsConnected,
		ActiveTCP:     cs.ActiveTCP,
		NetInterfaces: netInterfaces,

		DockerAvailable:  cs.DockerAvailable,
		DockerContainers: dockerContainers,

		OS:            cs.OS,
		Platform:      cs.Platform,
		KernelVersion: cs.KernelVersion,
		UptimeSeconds: cs.Uptime,
		Procs:         cs.Procs,

		Temperatures: temps,
		TopProcesses: procs,
	}
}

// MergeStats combines fast and slow metrics into a single RawStatsFixed.
func MergeStats(fast, slow *collector.RawStats, agentID, machineID, bootID string) RawStatsFixed {
	merged := ToRawStatsFixed(fast, KindMerged, agentID, machineID, bootID)

	if slow != nil {
		// Overlay slow metrics
		merged.NetLatencyMS = slow.NetLatency_ms
		merged.IsConnected = slow.IsConnected
		merged.ActiveTCP = slow.ActiveTCP

		merged.DiskHealth = make([]DiskHealthInfoFixed, 0, len(slow.DiskHealth))
		for _, h := range slow.DiskHealth {
			merged.DiskHealth = append(merged.DiskHealth, DiskHealthInfoFixed{
				Device:  h.Device,
				Status:  h.Status,
				Message: h.Message,
			})
		}

		merged.Hostname = slow.Hostname
		merged.OS = slow.OS
		merged.Platform = slow.Platform
		merged.KernelVersion = slow.KernelVersion
		merged.UptimeSeconds = slow.Uptime
		merged.Procs = slow.Procs

		merged.Temperatures = make([]TemperatureStatFixed, 0, len(slow.Temperatures))
		for _, t := range slow.Temperatures {
			merged.Temperatures = append(merged.Temperatures, TemperatureStatFixed{
				SensorKey:    t.SensorKey,
				TemperatureC: t.Temperature,
			})
		}
	}

	return merged
}
