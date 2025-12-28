package collector

import (
	"bytes"
	"context"
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

// RawStats represents a comprehensive snapshot of system metrics.
type RawStats struct {
	// CPU Metrics
	CPUUsage   float64   // Overall CPU utilization percentage (0-100)
	CPUPerCore []float64 // Per-core CPU utilization
	LoadAvg1   float64   // 1-minute load average
	LoadAvg5   float64   // 5-minute load average
	LoadAvg15  float64   // 15-minute load average
	CPUModel   string    // CPU model name
	CPUCores   int       // Number of logical CPU cores

	// RAM Metrics
	RAMUsage     float64
	RAMAvailable uint64
	RAMUsed      uint64
	RAMFree      uint64
	RAMCached    uint64
	RAMBuffered  uint64
	TotalRAM_GB  uint64

	// Swap Metrics
	SwapUsage float64
	SwapTotal uint64
	SwapUsed  uint64

	// Disk Metrics
	DiskUsage    float64
	TotalDisk_GB uint64
	InodeUsage   float64
	TotalInodes  uint64

	// Disk Details
	Partitions []PartitionUsage
	IOCounters []DiskIOCounters
	DiskHealth []DiskHealthInfo

	// Network Metrics
	NetLatency_ms float64
	IsConnected   bool
	NetInterfaces []NetInterfaceStats
	ActiveTCP     int
}

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

type PartitionUsage struct {
	Mountpoint  string
	Device      string
	Fstype      string
	UsedPercent float64
	TotalGB     uint64
	InodeUsage  float64
	TotalInodes uint64
}

type DiskIOCounters struct {
	Device      string
	ReadBytes   uint64
	WriteBytes  uint64
	ReadCount   uint64
	WriteCount  uint64
	ReadTimeMS  uint64
	WriteTimeMS uint64
}

type DiskHealthInfo struct {
	Device  string
	Status  string
	Message string
}

// ============================================================================
// INTERFACE DEFINITION
// ============================================================================

// StatsProvider defines the contract for any system metrics collector.
type StatsProvider interface {
	GetRawMetrics() (*RawStats, error)
	GetFastMetrics(ctx context.Context) (*RawStats, error)
	GetSlowMetrics(ctx context.Context) (*RawStats, error)
}

// ============================================================================
// CONCRETE IMPLEMENTATION
// ============================================================================

type SystemCollector struct{}

// Internal result types for concurrency
type cpuResult struct {
	total   float64
	perCore []float64
	model   string
	cores   int
	err     error
}

type loadResult struct {
	avg1  float64
	avg5  float64
	avg15 float64
	err   error
}

type memResult struct {
	value *mem.VirtualMemoryStat
	err   error
}

type diskResult struct {
	value *disk.UsageStat
	err   error
}

type netResult struct {
	latency float64
	online  bool
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

// GetRawMetrics collects all system metrics concurrently.
// It is kept for backward compatibility and delegates to the new split methods.
func (s SystemCollector) GetRawMetrics() (*RawStats, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var wg sync.WaitGroup
	wg.Add(2)

	var fastStats, slowStats *RawStats
	var fastErr, slowErr error

	go func() {
		defer wg.Done()
		fastStats, fastErr = s.GetFastMetrics(ctx)
	}()

	go func() {
		defer wg.Done()
		slowStats, slowErr = s.GetSlowMetrics(ctx)
	}()

	wg.Wait()

	if fastErr != nil {
		return nil, fastErr
	}

	// Merging:
	stats := fastStats
	// We check slowErr only if we care, but for now we just skip merging if it failed or is nil
	if slowErr == nil && slowStats != nil {
		stats.NetLatency_ms = slowStats.NetLatency_ms
		stats.IsConnected = slowStats.IsConnected
		stats.ActiveTCP = slowStats.ActiveTCP
		stats.DiskHealth = slowStats.DiskHealth
	}
	return stats, nil
}

// GetFastMetrics collects high-frequency metrics (CPU, RAM, Disk Usage/IO, Net IO).
func (s SystemCollector) GetFastMetrics(ctx context.Context) (*RawStats, error) {
	cpuCh := make(chan cpuResult, 1)
	loadCh := make(chan loadResult, 1)
	memCh := make(chan memResult, 1)
	diskCh := make(chan diskResult, 1)
	netIOCh := make(chan netIOResult, 1)
	partitionCh := make(chan partitionResult, 1)
	ioCh := make(chan ioResult, 1)

	var wg sync.WaitGroup
	wg.Add(7)

	go s.fetchCPU(&wg, cpuCh)
	go s.fetchLoad(&wg, loadCh)
	go s.fetchMemory(&wg, memCh)
	go s.fetchDisk(&wg, diskCh)
	go s.fetchNetIO(&wg, netIOCh)
	go s.fetchPartitions(&wg, partitionCh)
	go s.fetchIO(&wg, ioCh)

	// Wait with context awareness?
	// For simplicity, we just wait, but individual fetchers could take context.
	// Currently fetchers are synchronous Gopsutil calls which are generally fast (except maybe net/disk io on bad hardware).
	wg.Wait()

	// Gather results
	cpuRes := <-cpuCh
	loadRes := <-loadCh
	memRes := <-memCh
	diskRes := <-diskCh
	netIORes := <-netIOCh
	partitionRes := <-partitionCh
	ioRes := <-ioCh

	if cpuRes.err != nil {
		return nil, fmt.Errorf("failed to get CPU metrics: %w", cpuRes.err)
	}
	if loadRes.err != nil {
		return nil, fmt.Errorf("failed to get load average: %w", loadRes.err)
	}
	if memRes.err != nil {
		return nil, fmt.Errorf("failed to get memory metrics: %w", memRes.err)
	}
	if diskRes.err != nil {
		return nil, fmt.Errorf("failed to get disk metrics: %w", diskRes.err)
	}

	swapInfo, err := mem.SwapMemory()
	swapUsagePercent := 0.0
	swapTotal := uint64(0)
	swapUsed := uint64(0)
	if err == nil {
		swapUsagePercent = swapInfo.UsedPercent
		swapTotal = swapInfo.Total
		swapUsed = swapInfo.Used
	}

	inodeUsagePercent := 0.0
	if diskRes.value.InodesTotal > 0 {
		inodeUsagePercent = float64(diskRes.value.InodesUsed) / float64(diskRes.value.InodesTotal) * 100
	}

	return &RawStats{
		CPUUsage:      cpuRes.total,
		CPUPerCore:    cpuRes.perCore,
		LoadAvg1:      loadRes.avg1,
		LoadAvg5:      loadRes.avg5,
		LoadAvg15:     loadRes.avg15,
		CPUModel:      cpuRes.model,
		CPUCores:      cpuRes.cores,
		RAMUsage:      memRes.value.UsedPercent,
		RAMAvailable:  memRes.value.Available / (1024 * 1024 * 1024),
		RAMUsed:       memRes.value.Used / (1024 * 1024 * 1024),
		RAMFree:       memRes.value.Free / (1024 * 1024 * 1024),
		RAMCached:     memRes.value.Cached / (1024 * 1024 * 1024),
		RAMBuffered:   memRes.value.Buffers / (1024 * 1024 * 1024),
		TotalRAM_GB:   memRes.value.Total / (1024 * 1024 * 1024),
		SwapUsage:     swapUsagePercent,
		SwapTotal:     swapTotal / (1024 * 1024 * 1024),
		SwapUsed:      swapUsed / (1024 * 1024 * 1024),
		DiskUsage:     diskRes.value.UsedPercent,
		TotalDisk_GB:  diskRes.value.Total / (1024 * 1024 * 1024),
		InodeUsage:    inodeUsagePercent,
		TotalInodes:   diskRes.value.InodesTotal,
		Partitions:    partitionRes.partitions,
		IOCounters:    ioRes.counters,
		NetInterfaces: netIORes.stats,
	}, nil
}

// GetSlowMetrics collects low-frequency metrics (Disk Health, Network Latency, Net Connections).
func (s SystemCollector) GetSlowMetrics(ctx context.Context) (*RawStats, error) {
	netCh := make(chan netResult, 1)
	netConnCh := make(chan netConnResult, 1)
	healthCh := make(chan healthResult, 1)

	var wg sync.WaitGroup
	wg.Add(3)

	go s.fetchNetwork(ctx, &wg, netCh)
	go s.fetchNetConns(&wg, netConnCh)
	go s.fetchHealth(&wg, healthCh)

	wg.Wait()

	netRes := <-netCh
	netConnRes := <-netConnCh
	healthRes := <-healthCh

	return &RawStats{
		NetLatency_ms: netRes.latency,
		IsConnected:   netRes.online,
		ActiveTCP:     netConnRes.activeTCP,
		DiskHealth:    healthRes.health,
	}, nil
}

// Helper methods for concurrent fetching

func (s SystemCollector) fetchCPU(wg *sync.WaitGroup, ch chan cpuResult) {
	defer wg.Done()
	defer close(ch)

	total, err := cpu.Percent(0, false)
	if err != nil {
		ch <- cpuResult{err: err}
		return
	}
	perCore, err := cpu.Percent(0, true)
	if err != nil {
		ch <- cpuResult{err: err}
		return
	}
	info, err := cpu.Info()
	model := "Unknown"
	if err == nil && len(info) > 0 {
		model = info[0].ModelName
	}
	ch <- cpuResult{total: total[0], perCore: perCore, model: model, cores: len(perCore)}
}

func (s SystemCollector) fetchLoad(wg *sync.WaitGroup, ch chan loadResult) {
	defer wg.Done()
	defer close(ch)
	avgStat, err := load.Avg()
	ch <- loadResult{avg1: avgStat.Load1, avg5: avgStat.Load5, avg15: avgStat.Load15, err: err}
}

func (s SystemCollector) fetchMemory(wg *sync.WaitGroup, ch chan memResult) {
	defer wg.Done()
	defer close(ch)
	value, err := mem.VirtualMemory()
	ch <- memResult{value: value, err: err}
}

func (s SystemCollector) fetchDisk(wg *sync.WaitGroup, ch chan diskResult) {
	defer wg.Done()
	defer close(ch)
	value, err := disk.Usage("/")
	ch <- diskResult{value: value, err: err}
}

func (s SystemCollector) fetchNetwork(ctx context.Context, wg *sync.WaitGroup, ch chan netResult) {
	defer wg.Done()
	defer close(ch)

	start := time.Now()
	var d net.Dialer
	conn, err := d.DialContext(ctx, "tcp", "8.8.8.8:53")
	if err != nil {
		ch <- netResult{latency: 0, online: false}
		return
	}
	conn.Close()
	ch <- netResult{latency: float64(time.Since(start).Milliseconds()), online: true}
}

func (s SystemCollector) fetchNetIO(wg *sync.WaitGroup, ch chan netIOResult) {
	defer wg.Done()
	defer close(ch)
	counters, err := gnet.IOCounters(true)
	if err != nil {
		ch <- netIOResult{err: err}
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
	ch <- netIOResult{stats: stats}
}

func (s SystemCollector) fetchNetConns(wg *sync.WaitGroup, ch chan netConnResult) {
	defer wg.Done()
	defer close(ch)
	conns, err := gnet.Connections("tcp")
	active := 0
	if err == nil {
		active = len(conns)
	}
	ch <- netConnResult{activeTCP: active, err: err}
}

func (s SystemCollector) fetchPartitions(wg *sync.WaitGroup, ch chan partitionResult) {
	defer wg.Done()
	defer close(ch)

	targets := []string{"/", "/var/log", "/home"}
	partitions, err := disk.Partitions(true)
	if err != nil {
		ch <- partitionResult{err: err}
		return
	}
	byMount := make(map[string]disk.PartitionStat)
	for _, p := range partitions {
		byMount[p.Mountpoint] = p
	}

	var usages []PartitionUsage
	for _, mount := range targets {
		if p, ok := byMount[mount]; ok {
			if usage, err := disk.Usage(mount); err == nil {
				inodeUsage := 0.0
				if usage.InodesTotal > 0 {
					inodeUsage = float64(usage.InodesUsed) / float64(usage.InodesTotal) * 100
				}
				usages = append(usages, PartitionUsage{
					Mountpoint:  mount,
					Device:      p.Device,
					Fstype:      p.Fstype,
					UsedPercent: usage.UsedPercent,
					TotalGB:     usage.Total / (1024 * 1024 * 1024),
					InodeUsage:  inodeUsage,
					TotalInodes: usage.InodesTotal,
				})
			}
		}
	}
	ch <- partitionResult{partitions: usages}
}

func (s SystemCollector) fetchIO(wg *sync.WaitGroup, ch chan ioResult) {
	defer wg.Done()
	defer close(ch)
	counters, err := disk.IOCounters()
	if err != nil {
		ch <- ioResult{err: err}
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
	ch <- ioResult{counters: collected}
}

func (s SystemCollector) fetchHealth(wg *sync.WaitGroup, ch chan healthResult) {
	defer wg.Done()
	defer close(ch)

	if _, err := exec.LookPath("smartctl"); err != nil {
		ch <- healthResult{}
		return
	}

	partitions, err := disk.Partitions(false)
	if err != nil {
		ch <- healthResult{err: err}
		return
	}

	seen := make(map[string]bool)
	var health []DiskHealthInfo

	for _, p := range partitions {
		if p.Device == "" || seen[p.Device] {
			continue
		}
		seen[p.Device] = true

		cmd := exec.Command("smartctl", "-H", p.Device)
		output, err := cmd.CombinedOutput()

		status := "unknown"
		msg := "smartctl output unavailable"
		if err != nil {
			msg = err.Error()
		}
		if bytes.Contains(output, []byte("PASSED")) {
			status = "passed"
			msg = "SMART health passed"
		} else if bytes.Contains(output, []byte("FAILED")) {
			status = "failed"
			msg = "SMART health failed"
		}
		health = append(health, DiskHealthInfo{Device: p.Device, Status: status, Message: msg})
	}
	ch <- healthResult{health: health}
}
