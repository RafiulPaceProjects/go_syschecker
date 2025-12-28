package collector

import (
	"bytes"
	"fmt"
	"net"
	"os/exec"
	"sync"
	"time"

	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/disk"
	"github.com/shirou/gopsutil/v4/load"
	"github.com/shirou/gopsutil/v4/mem"
	gnet "github.com/shirou/gopsutil/v4/net"
)

// ============================================================================
// DATA STRUCTURES
// ============================================================================

// RawStats represents a comprehensive snapshot of system metrics at a specific
// point in time. All percentage values are in the range [0, 100].
// All size values are in Gigabytes (GB) unless otherwise noted.
type RawStats struct {
	// CPU Metrics
	CPUUsage   float64   // Overall CPU utilization percentage (0-100)
	CPUPerCore []float64 // Per-core CPU utilization (useful for detecting single-threaded bottlenecks)
	LoadAvg1   float64   // 1-minute load average (number of processes waiting for CPU time)
	LoadAvg5   float64   // 5-minute load average (medium-term trend)
	LoadAvg15  float64   // 15-minute load average (long-term trend)
	CPUModel   string    // CPU model name (e.g., "Intel Core i7-9750H")
	CPUCores   int       // Number of logical CPU cores

	// RAM Metrics
	RAMUsage     float64 // Percentage of RAM in use (includes cached/buffered memory on Linux)
	RAMAvailable uint64  // GB of RAM available for new processes (the most accurate metric)
	RAMUsed      uint64  // GB of RAM actively used by programs
	RAMFree      uint64  // GB of completely unused RAM (typically low on Linux due to disk caching)
	RAMCached    uint64  // GB of RAM used for disk caching (Linux only; automatically freed when needed)
	RAMBuffered  uint64  // GB of RAM used for I/O buffering (Linux only)
	TotalRAM_GB  uint64  // Total physical RAM in GB

	// Swap Metrics
	SwapUsage float64 // Percentage of swap space in use (high values indicate RAM exhaustion)
	SwapTotal uint64  // Total swap space in GB
	SwapUsed  uint64  // GB of swap space currently in use

	// Disk Metrics
	DiskUsage    float64 // Percentage of disk space in use
	TotalDisk_GB uint64  // Total disk capacity in GB
	InodeUsage   float64 // Percentage of inodes in use (Linux/macOS only; 0 on Windows)
	TotalInodes  uint64  // Total number of available inodes (file system slots)

	// Disk Details
	Partitions []PartitionUsage // Targeted partition snapshots (/, /var/log, /home when present)
	IOCounters []DiskIOCounters // Device-level I/O counters (best-effort)
	DiskHealth []DiskHealthInfo // SMART health results (best-effort; empty if unavailable)

	// Network Metrics
	NetLatency_ms float64             // Latency to 8.8.8.8:53 in milliseconds (0 if offline)
	IsConnected   bool                // Whether the system has internet connectivity
	NetInterfaces []NetInterfaceStats // Per-interface traffic/errors/drops (best-effort)
	ActiveTCP     int                 // Number of active TCP connections (best-effort)
}

// NetInterfaceStats captures per-interface traffic and errors.
type NetInterfaceStats struct {
	Name        string
	BytesSent   uint64
	BytesRecv   uint64
	PacketsSent uint64
	PacketsRecv uint64
	ErrIn       uint64
	ErrOut      uint64
	DropIn      uint64
	DropOut     uint64
}

// PartitionUsage captures filesystem-level usage for a specific mountpoint.
type PartitionUsage struct {
	Mountpoint  string  // e.g., "/", "/var/log", "/home"
	Device      string  // backing device (e.g., /dev/sda1)
	Fstype      string  // filesystem type
	UsedPercent float64 // space used percent
	TotalGB     uint64  // total size in GB
	InodeUsage  float64 // inode usage percent (0 if unavailable)
	TotalInodes uint64  // total inodes
}

// DiskIOCounters contains per-device I/O counters from the OS.
type DiskIOCounters struct {
	Device      string // device name as reported by the OS
	ReadBytes   uint64
	WriteBytes  uint64
	ReadCount   uint64
	WriteCount  uint64
	ReadTimeMS  uint64 // milliseconds spent on reads (if provided by OS)
	WriteTimeMS uint64 // milliseconds spent on writes (if provided by OS)
}

// DiskHealthInfo captures best-effort SMART health information.
type DiskHealthInfo struct {
	Device  string // device path (e.g., /dev/sda)
	Status  string // "passed", "failed", "unknown"
	Message string // additional context or error details
}

// ============================================================================
// INTERFACE DEFINITION
// ============================================================================

// StatsProvider defines the contract for any system metrics collector.
// This abstraction allows for:
// - Easy mocking in unit tests
// - Future implementations (e.g., remote SSH collectors, cloud API collectors)
// - Dependency injection for better testability
type StatsProvider interface {
	GetRawMetrics() (RawStats, error)
}

// ============================================================================
// CONCRETE IMPLEMENTATION
// ============================================================================

// SystemCollector is the production implementation that retrieves real system
// metrics using the gopsutil library. All metrics are collected concurrently
// to minimize latency.
type SystemCollector struct{}

// GetRawMetrics collects all system metrics concurrently and returns a
// comprehensive snapshot. This method typically completes in ~200ms
// (limited by the network timeout, not the sum of all operations).
//
// Concurrency Strategy:
//   - 10 goroutines run in parallel to collect CPU, Load, Memory, Disk, Network,
//     partition usage, I/O counters, SMART health (best-effort), NIC I/O, and TCP connections
//   - A sync.WaitGroup ensures all goroutines complete before returning
//   - Buffered channels prevent goroutine leaks if the main function exits early
//
// Error Handling:
// - If any critical metric (CPU, Load, Memory, Disk) fails, the entire function returns an error
// - Network failures are tolerated (IsConnected=false, Latency=0)
// - Swap metrics are optional (missing swap space is not an error)
func (s SystemCollector) GetRawMetrics() (RawStats, error) {
	// ========================================================================
	// STEP 1: Define result types for each concurrent operation
	// ========================================================================
	// These structs bundle the data and any error from each goroutine.
	// Channels in Go can only carry one type, so we wrap multiple values.

	type cpuResult struct {
		total   float64   // Overall CPU usage
		perCore []float64 // Per-core breakdown
		model   string    // CPU model name
		cores   int       // Number of logical cores
		err     error     // Error (if any)
	}

	type loadResult struct {
		avg1  float64 // 1-minute load average
		avg5  float64 // 5-minute load average
		avg15 float64 // 15-minute load average
		err   error   // Error (if any)
	}

	type memResult struct {
		value *mem.VirtualMemoryStat // Complete memory statistics
		err   error                  // Error (if any)
	}

	type diskResult struct {
		value *disk.UsageStat // Complete disk statistics
		err   error           // Error (if any)
	}

	type netResult struct {
		latency float64 // Round-trip time in milliseconds
		online  bool    // Whether the connection succeeded
	}

	type netIOResult struct {
		stats []NetInterfaceStats
		err   error
	}

	type netConnResult struct {
		activeTCP int
		err       error
	}

	type partitionResult struct {
		partitions []PartitionUsage
		err        error
	}

	type ioResult struct {
		counters []DiskIOCounters
		err      error
	}

	type healthResult struct {
		health []DiskHealthInfo
		err    error
	}

	// ========================================================================
	// STEP 2: Create buffered channels for each metric
	// ========================================================================
	// Buffered channels (size=1) allow goroutines to send their result and
	// exit immediately, even if the main function hasn't read from the
	// channel yet. This prevents goroutine leaks.

	cpuCh := make(chan cpuResult, 1)
	loadCh := make(chan loadResult, 1)
	memCh := make(chan memResult, 1)
	diskCh := make(chan diskResult, 1)
	netCh := make(chan netResult, 1)
	netIOCh := make(chan netIOResult, 1)
	netConnCh := make(chan netConnResult, 1)
	partitionCh := make(chan partitionResult, 1)
	ioCh := make(chan ioResult, 1)
	healthCh := make(chan healthResult, 1)

	// ========================================================================
	// STEP 3: Launch concurrent workers
	// ========================================================================
	var wg sync.WaitGroup
	wg.Add(10) // We expect 10 goroutines to complete

	// ------------------------------------------------------------------------
	// CPU Goroutine: Collects total usage, per-core usage, and CPU info
	// ------------------------------------------------------------------------
	go func() {
		defer wg.Done()

		// Get overall CPU usage (averaged across all cores)
		total, err := cpu.Percent(0, false)
		if err != nil {
			cpuCh <- cpuResult{err: err}
			return
		}

		// Get per-core CPU usage (useful for detecting single-threaded bottlenecks)
		perCore, err := cpu.Percent(0, true)
		if err != nil {
			cpuCh <- cpuResult{err: err}
			return
		}

		// Get CPU model and core count
		info, err := cpu.Info()
		model := "Unknown"
		cores := len(perCore)
		if err == nil && len(info) > 0 {
			model = info[0].ModelName
		}

		cpuCh <- cpuResult{
			total:   total[0],
			perCore: perCore,
			model:   model,
			cores:   cores,
			err:     nil,
		}
	}()

	// ------------------------------------------------------------------------
	// Load Goroutine: Retrieves OS-maintained load averages
	// ------------------------------------------------------------------------
	// Load average = number of processes waiting for CPU time
	// On a 4-core system, a load of 4.0 means "fully utilized"
	go func() {
		defer wg.Done()
		avgStat, err := load.Avg()
		if err != nil {
			loadCh <- loadResult{err: err}
			return
		}
		loadCh <- loadResult{
			avg1:  avgStat.Load1,
			avg5:  avgStat.Load5,
			avg15: avgStat.Load15,
			err:   nil,
		}
	}()

	// ------------------------------------------------------------------------
	// Memory Goroutine: Collects detailed RAM statistics
	// ------------------------------------------------------------------------
	go func() {
		defer wg.Done()
		value, err := mem.VirtualMemory()
		memCh <- memResult{value: value, err: err}
	}()

	// ------------------------------------------------------------------------
	// Disk Goroutine: Retrieves disk space and inode usage
	// ------------------------------------------------------------------------
	// On Linux/macOS: Inodes represent file system slots
	// On Windows: Inodes are not applicable (InodesTotal will be 0)
	go func() {
		defer wg.Done()
		value, err := disk.Usage("/") // Root filesystem
		diskCh <- diskResult{value: value, err: err}
	}()

	// ------------------------------------------------------------------------
	// Network Goroutine: Tests connectivity and measures latency
	// ------------------------------------------------------------------------
	// Target: Google's public DNS server (8.8.8.8:53)
	// Timeout: 2 seconds (prevents hanging on network issues)
	go func() {
		defer wg.Done()
		start := time.Now()
		conn, err := net.DialTimeout("tcp", "8.8.8.8:53", 2*time.Second)
		if err != nil {
			netCh <- netResult{latency: 0, online: false}
			return
		}
		defer conn.Close()
		netCh <- netResult{
			latency: float64(time.Since(start).Milliseconds()),
			online:  true,
		}
	}()

	// ------------------------------------------------------------------------
	// Network I/O Goroutine: Collect per-interface traffic and errors
	// ------------------------------------------------------------------------
	go func() {
		defer wg.Done()
		counters, err := gnet.IOCounters(true)
		if err != nil {
			netIOCh <- netIOResult{err: err}
			return
		}

		var stats []NetInterfaceStats
		for _, c := range counters {
			stats = append(stats, NetInterfaceStats{
				Name:        c.Name,
				BytesSent:   c.BytesSent,
				BytesRecv:   c.BytesRecv,
				PacketsSent: c.PacketsSent,
				PacketsRecv: c.PacketsRecv,
				ErrIn:       c.Errin,
				ErrOut:      c.Errout,
				DropIn:      c.Dropin,
				DropOut:     c.Dropout,
			})
		}

		netIOCh <- netIOResult{stats: stats, err: nil}
	}()

	// ------------------------------------------------------------------------
	// Network Connections Goroutine: Count active TCP connections
	// ------------------------------------------------------------------------
	go func() {
		defer wg.Done()
		conns, err := gnet.Connections("tcp")
		if err != nil {
			netConnCh <- netConnResult{err: err}
			return
		}
		netConnCh <- netConnResult{activeTCP: len(conns), err: nil}
	}()

	// ------------------------------------------------------------------------
	// Partition Goroutine: Collect usage for key mountpoints
	// ------------------------------------------------------------------------
	go func() {
		defer wg.Done()

		targets := []string{"/", "/var/log", "/home"}
		partitions, err := disk.Partitions(true)
		if err != nil {
			partitionCh <- partitionResult{err: err}
			return
		}

		byMount := make(map[string]disk.PartitionStat)
		for _, p := range partitions {
			byMount[p.Mountpoint] = p
		}

		var usages []PartitionUsage
		for _, mount := range targets {
			p, ok := byMount[mount]
			if !ok {
				continue // Skip mounts not present on this system
			}

			usage, err := disk.Usage(mount)
			if err != nil {
				continue // Best effort: skip if we cannot read
			}

			inodeUsagePercent := 0.0
			if usage.InodesTotal > 0 {
				inodeUsagePercent = float64(usage.InodesUsed) / float64(usage.InodesTotal) * 100
			}

			usages = append(usages, PartitionUsage{
				Mountpoint:  mount,
				Device:      p.Device,
				Fstype:      p.Fstype,
				UsedPercent: usage.UsedPercent,
				TotalGB:     usage.Total / (1024 * 1024 * 1024),
				InodeUsage:  inodeUsagePercent,
				TotalInodes: usage.InodesTotal,
			})
		}

		partitionCh <- partitionResult{partitions: usages, err: nil}
	}()

	// ------------------------------------------------------------------------
	// I/O Counters Goroutine: Collect per-device I/O counters
	// ------------------------------------------------------------------------
	go func() {
		defer wg.Done()
		counters, err := disk.IOCounters()
		if err != nil {
			ioCh <- ioResult{err: err}
			return
		}

		var collected []DiskIOCounters
		for dev, c := range counters {
			collected = append(collected, DiskIOCounters{
				Device:      dev,
				ReadBytes:   c.ReadBytes,
				WriteBytes:  c.WriteBytes,
				ReadCount:   c.ReadCount,
				WriteCount:  c.WriteCount,
				ReadTimeMS:  c.ReadTime,
				WriteTimeMS: c.WriteTime,
			})
		}

		ioCh <- ioResult{counters: collected, err: nil}
	}()

	// ------------------------------------------------------------------------
	// Disk Health Goroutine: Best-effort SMART health via smartctl (if present)
	// ------------------------------------------------------------------------
	go func() {
		defer wg.Done()

		// Check for smartctl availability first
		_, lookErr := exec.LookPath("smartctl")
		if lookErr != nil {
			healthCh <- healthResult{health: nil, err: nil}
			return
		}

		partitions, err := disk.Partitions(false)
		if err != nil {
			healthCh <- healthResult{err: err}
			return
		}

		seen := make(map[string]bool)
		var health []DiskHealthInfo

		for _, p := range partitions {
			dev := p.Device
			if dev == "" || seen[dev] {
				continue
			}
			seen[dev] = true

			cmd := exec.Command("smartctl", "-H", dev)
			output, err := cmd.CombinedOutput()

			status := "unknown"
			message := "smartctl output unavailable"
			if err != nil {
				message = err.Error()
			}
			if bytes.Contains(output, []byte("PASSED")) {
				status = "passed"
				message = "SMART health passed"
			} else if bytes.Contains(output, []byte("FAILED")) {
				status = "failed"
				message = "SMART health failed"
			}

			health = append(health, DiskHealthInfo{
				Device:  dev,
				Status:  status,
				Message: message,
			})
		}

		healthCh <- healthResult{health: health, err: nil}
	}()

	// ========================================================================
	// STEP 4: Wait for all goroutines to complete
	// ========================================================================
	wg.Wait()
	close(cpuCh)
	close(loadCh)
	close(memCh)
	close(diskCh)
	close(netCh)
	close(netIOCh)
	close(netConnCh)
	close(partitionCh)
	close(ioCh)
	close(healthCh)

	// ========================================================================
	// STEP 5: Collect results from channels and handle errors
	// ========================================================================

	cpuRes := <-cpuCh
	if cpuRes.err != nil {
		return RawStats{}, fmt.Errorf("failed to get CPU metrics: %w", cpuRes.err)
	}

	loadRes := <-loadCh
	if loadRes.err != nil {
		return RawStats{}, fmt.Errorf("failed to get load average: %w", loadRes.err)
	}

	memRes := <-memCh
	if memRes.err != nil {
		return RawStats{}, fmt.Errorf("failed to get memory metrics: %w", memRes.err)
	}

	// ------------------------------------------------------------------------
	// Optional: Retrieve swap space information
	// ------------------------------------------------------------------------
	// Swap is not always available (e.g., some cloud instances disable it).
	// If this fails, we default to zero values rather than returning an error.
	swapInfo, err := mem.SwapMemory()
	swapUsagePercent := 0.0
	swapTotal := uint64(0)
	swapUsed := uint64(0)
	if err == nil {
		swapUsagePercent = swapInfo.UsedPercent
		swapTotal = swapInfo.Total
		swapUsed = swapInfo.Used
	}

	diskRes := <-diskCh
	if diskRes.err != nil {
		return RawStats{}, fmt.Errorf("failed to get disk metrics: %w", diskRes.err)
	}

	netRes := <-netCh
	// Network errors are tolerated; we simply mark as offline
	netIORes := <-netIOCh
	netConnRes := <-netConnCh

	partitionRes := <-partitionCh
	ioRes := <-ioCh
	healthRes := <-healthCh

	// ------------------------------------------------------------------------
	// Calculate inode usage percentage (Linux/macOS only)
	// ------------------------------------------------------------------------
	// On Windows, InodesTotal will be 0, so this calculation is skipped.
	inodeUsagePercent := 0.0
	if diskRes.value.InodesTotal > 0 {
		inodeUsagePercent = float64(diskRes.value.InodesUsed) / float64(diskRes.value.InodesTotal) * 100
	}

	// ========================================================================
	// STEP 6: Assemble and return the final result
	// ========================================================================
	return RawStats{
		// CPU
		CPUUsage:   cpuRes.total,
		CPUPerCore: cpuRes.perCore,
		LoadAvg1:   loadRes.avg1,
		LoadAvg5:   loadRes.avg5,
		LoadAvg15:  loadRes.avg15,
		CPUModel:   cpuRes.model,
		CPUCores:   cpuRes.cores,

		// RAM
		RAMUsage:     memRes.value.UsedPercent,
		RAMAvailable: memRes.value.Available / (1024 * 1024 * 1024), // Convert to GB
		RAMUsed:      memRes.value.Used / (1024 * 1024 * 1024),      // Convert to GB
		RAMFree:      memRes.value.Free / (1024 * 1024 * 1024),      // Convert to GB
		RAMCached:    memRes.value.Cached / (1024 * 1024 * 1024),    // Convert to GB (Linux only)
		RAMBuffered:  memRes.value.Buffers / (1024 * 1024 * 1024),   // Convert to GB (Linux only)
		TotalRAM_GB:  memRes.value.Total / (1024 * 1024 * 1024),     // Convert to GB

		// Swap
		SwapUsage: swapUsagePercent,
		SwapTotal: swapTotal / (1024 * 1024 * 1024), // Convert to GB
		SwapUsed:  swapUsed / (1024 * 1024 * 1024),  // Convert to GB

		// Disk
		DiskUsage:    diskRes.value.UsedPercent,
		TotalDisk_GB: diskRes.value.Total / (1024 * 1024 * 1024), // Convert to GB
		InodeUsage:   inodeUsagePercent,
		TotalInodes:  diskRes.value.InodesTotal,

		// Disk details
		Partitions: partitionRes.partitions,
		IOCounters: ioRes.counters,
		DiskHealth: healthRes.health,

		// Network detail
		NetInterfaces: netIORes.stats,
		ActiveTCP:     netConnRes.activeTCP,

		// Network
		NetLatency_ms: netRes.latency,
		IsConnected:   netRes.online,
	}, nil
}
