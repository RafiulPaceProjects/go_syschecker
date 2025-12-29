package collector

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"os/exec"
	"sync"
	"time"

	"syschecker/internal/collector/services"

	"github.com/shirou/gopsutil/v4/disk"
	"github.com/shirou/gopsutil/v4/load"
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

	// Docker Metrics
	DockerAvailable  bool
	DockerContainers []DockerContainerInfo

	// Host Metrics
	Hostname      string
	OS            string
	Platform      string
	KernelVersion string
	Uptime        uint64
	Procs         uint64

	// Physical Metrics
	Temperatures []TemperatureStat

	// Process Metrics
	TopProcesses []ProcessStat
}

type DockerContainerInfo struct {
	ID         string
	Name       string
	Image      string
	Status     string
	Running    bool
	CPUUsage   float64
	MemUsage   uint64
	MemLimit   uint64
	MemPercent float64
}

type TemperatureStat struct {
	SensorKey   string
	Temperature float64
}

type ProcessStat struct {
	PID    int32
	Name   string
	CPU    float64
	Memory float32
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
	GetFastMetrics(ctx context.Context) (*RawStats, error)
	GetSlowMetrics(ctx context.Context) (*RawStats, error)
}

// ============================================================================
// CONCRETE IMPLEMENTATION
// ============================================================================

type SystemCollector struct {
	cpuSensor      services.Sensor
	memSensor      services.Sensor
	diskSensor     services.Sensor
	netSensor      services.Sensor
	dockerSensor   services.Sensor
	hostSensor     services.Sensor
	physicalSensor services.Sensor
	processSensor  services.Sensor
}

func NewSystemCollector() *SystemCollector {
	return &SystemCollector{
		cpuSensor:      services.NewCPUSensor(),
		memSensor:      services.NewMemSensor(),
		diskSensor:     services.NewDiskSensor(),
		netSensor:      services.NewNetSensor(),
		dockerSensor:   services.NewDockerSensor(),
		hostSensor:     services.NewHostSensor(),
		physicalSensor: services.NewPhysicalSensor(),
		processSensor:  services.NewProcessSensor(),
	}
}

// Internal result types for concurrency
type cpuResult struct {
	stats services.CPUResult
	err   error
}

type loadResult struct {
	avg1  float64
	avg5  float64
	avg15 float64
	err   error
}

type memResult struct {
	stats services.MemResult
	err   error
}

type diskResult struct {
	stats services.DiskResult
	err   error
}

type dockerMetricsResult struct {
	stats services.DockerResult
	err   error
}

type hostResult struct {
	stats services.HostResult
	err   error
}

type physicalResult struct {
	stats services.PhysicalResult
	err   error
}

type processResult struct {
	stats services.ProcessResult
	err   error
}

type netResult struct {
	latency float64
	online  bool
}

type netIOResult struct {
	stats services.NetResult
	err   error
}

type netConnResult struct {
	activeTCP int
	err       error
}

type healthResult struct {
	health []DiskHealthInfo
	err    error
}

// GetFastMetrics collects high-frequency metrics (CPU, RAM, Disk Usage/IO, Net IO, Docker, Processes).
func (s *SystemCollector) GetFastMetrics(ctx context.Context) (*RawStats, error) {
	cpuCh := make(chan cpuResult, 1)
	loadCh := make(chan loadResult, 1)
	memCh := make(chan memResult, 1)
	diskCh := make(chan diskResult, 1)
	netIOCh := make(chan netIOResult, 1)
	dockerCh := make(chan dockerMetricsResult, 1)
	processCh := make(chan processResult, 1)

	var wg sync.WaitGroup
	wg.Add(7)

	go func() {
		defer wg.Done()
		res, err := s.cpuSensor.Collect(ctx)
		if err != nil {
			cpuCh <- cpuResult{err: err}
			return
		}
		cpuCh <- cpuResult{stats: res.(services.CPUResult), err: nil}
	}()

	go s.fetchLoad(&wg, loadCh)

	go func() {
		defer wg.Done()
		res, err := s.memSensor.Collect(ctx)
		if err != nil {
			memCh <- memResult{err: err}
			return
		}
		memCh <- memResult{stats: res.(services.MemResult), err: nil}
	}()
	go func() {
		defer wg.Done()
		res, err := s.diskSensor.Collect(ctx)
		if err != nil {
			diskCh <- diskResult{err: err}
			return
		}
		diskCh <- diskResult{stats: res.(services.DiskResult), err: nil}
	}()

	go func() {
		defer wg.Done()
		res, err := s.netSensor.Collect(ctx)
		if err != nil {
			netIOCh <- netIOResult{err: err}
			return
		}
		netIOCh <- netIOResult{stats: res.(services.NetResult), err: nil}
	}()

	go func() {
		defer wg.Done()
		res, err := s.dockerSensor.Collect(ctx)
		if err != nil {
			dockerCh <- dockerMetricsResult{err: err}
			return
		}
		dockerCh <- dockerMetricsResult{stats: res.(services.DockerResult), err: nil}
	}()

	go func() {
		defer wg.Done()
		res, err := s.processSensor.Collect(ctx)
		if err != nil {
			processCh <- processResult{err: err}
			return
		}
		processCh <- processResult{stats: res.(services.ProcessResult), err: nil}
	}()

	wg.Wait()

	// Gather results
	cpuRes := <-cpuCh
	loadRes := <-loadCh
	memRes := <-memCh
	diskRes := <-diskCh
	netIORes := <-netIOCh
	dockerRes := <-dockerCh
	processRes := <-processCh

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

	// Process Disk Results
	var rootUsage services.UsageStat
	usageMap := make(map[string]services.UsageStat)
	for _, u := range diskRes.stats.Usage {
		usageMap[u.Path] = u
		if u.Path == "/" {
			rootUsage = u
		}
	}

	partitions := []PartitionUsage{} // Initialize as empty slice
	for _, p := range diskRes.stats.Partitions {
		if u, ok := usageMap[p.Mountpoint]; ok {
			partitions = append(partitions, PartitionUsage{
				Mountpoint:  p.Mountpoint,
				Device:      p.Device,
				Fstype:      p.Fstype,
				UsedPercent: u.UsedPercent,
				TotalGB:     u.Total / (1024 * 1024 * 1024),
				InodeUsage:  u.InodesUsedPercent,
				TotalInodes: u.InodesTotal,
			})
		}
	}

	ioCounters := []DiskIOCounters{} // Initialize as empty slice
	for _, c := range diskRes.stats.IOCounters {
		ioCounters = append(ioCounters, DiskIOCounters{
			Device:      c.Name,
			ReadBytes:   c.ReadBytes,
			WriteBytes:  c.WriteBytes,
			ReadCount:   c.ReadCount,
			WriteCount:  c.WriteCount,
			ReadTimeMS:  c.ReadTime,
			WriteTimeMS: c.WriteTime,
		})
	}

	netStats := []NetInterfaceStats{} // Initialize as empty slice
	for _, ns := range netIORes.stats.Interfaces {
		netStats = append(netStats, NetInterfaceStats{
			Name:        ns.Name,
			BytesSent:   ns.BytesSent,
			BytesRecv:   ns.BytesRecv,
			PacketsSent: ns.PacketsSent,
			PacketsRecv: ns.PacketsRecv,
			ErrIn:       ns.ErrIn,
			ErrOut:      ns.ErrOut,
			DropIn:      ns.DropIn,
			DropOut:     ns.DropOut,
		})
	}

	dockerContainers := []DockerContainerInfo{} // Initialize as empty slice
	if dockerRes.err == nil && dockerRes.stats.Available {
		for _, c := range dockerRes.stats.Containers {
			dockerContainers = append(dockerContainers, DockerContainerInfo{
				ID:         c.ID,
				Name:       c.Name,
				Image:      c.Image,
				Status:     c.Status,
				Running:    c.Running,
				CPUUsage:   c.CPUUsage,
				MemUsage:   c.MemUsage,
				MemLimit:   c.MemLimit,
				MemPercent: c.MemPercent,
			})
		}
	}

	topProcesses := []ProcessStat{} // Initialize as empty slice
	if processRes.err == nil {
		for _, p := range processRes.stats.Processes {
			topProcesses = append(topProcesses, ProcessStat{
				PID:    p.PID,
				Name:   p.Name,
				CPU:    p.CPU,
				Memory: p.Memory,
			})
		}
	}

	return &RawStats{
		CPUUsage:         cpuRes.stats.TotalUsage,
		CPUPerCore:       cpuRes.stats.PerCore,
		LoadAvg1:         loadRes.avg1,
		LoadAvg5:         loadRes.avg5,
		LoadAvg15:        loadRes.avg15,
		CPUModel:         cpuRes.stats.Model,
		CPUCores:         cpuRes.stats.Cores,
		RAMUsage:         memRes.stats.UsedPercent,
		RAMAvailable:     memRes.stats.Available / (1024 * 1024 * 1024),
		RAMUsed:          memRes.stats.Used / (1024 * 1024 * 1024),
		RAMFree:          memRes.stats.Free / (1024 * 1024 * 1024),
		RAMCached:        memRes.stats.Cached / (1024 * 1024 * 1024),
		RAMBuffered:      memRes.stats.Buffers / (1024 * 1024 * 1024),
		TotalRAM_GB:      memRes.stats.Total / (1024 * 1024 * 1024),
		SwapUsage:        memRes.stats.SwapUsage,
		SwapTotal:        memRes.stats.SwapTotal / (1024 * 1024 * 1024),
		SwapUsed:         memRes.stats.SwapUsed / (1024 * 1024 * 1024),
		DiskUsage:        rootUsage.UsedPercent,
		TotalDisk_GB:     rootUsage.Total / (1024 * 1024 * 1024),
		InodeUsage:       rootUsage.InodesUsedPercent,
		TotalInodes:      rootUsage.InodesTotal,
		Partitions:       partitions,
		IOCounters:       ioCounters,
		NetInterfaces:    netStats,
		DockerAvailable:  dockerRes.stats.Available,
		DockerContainers: dockerContainers,
		TopProcesses:     topProcesses,
		DiskHealth:       []DiskHealthInfo{},  // Not collected in fast metrics
		Temperatures:     []TemperatureStat{}, // Not collected in fast metrics
		NetLatency_ms:    0,                   // Not collected in fast metrics
		IsConnected:      true,                // Assume connected in fast metrics
		ActiveTCP:        0,                   // Not collected in fast metrics
		Hostname:         "",                  // Not collected in fast metrics
		OS:               "",                  // Not collected in fast metrics
		Platform:         "",                  // Not collected in fast metrics
		KernelVersion:    "",                  // Not collected in fast metrics
		Uptime:           0,                   // Not collected in fast metrics
		Procs:            0,                   // Not collected in fast metrics
	}, nil
}

// GetSlowMetrics collects low-frequency metrics (Disk Health, Network Latency, Net Connections, Host, Physical).
func (s *SystemCollector) GetSlowMetrics(ctx context.Context) (*RawStats, error) {
	netCh := make(chan netResult, 1)
	netConnCh := make(chan netConnResult, 1)
	healthCh := make(chan healthResult, 1)
	hostCh := make(chan hostResult, 1)
	physCh := make(chan physicalResult, 1)

	var wg sync.WaitGroup
	wg.Add(5)

	go s.fetchNetwork(ctx, &wg, netCh)
	go s.fetchNetConns(&wg, netConnCh)
	go s.fetchHealth(&wg, healthCh)

	go func() {
		defer wg.Done()
		res, err := s.hostSensor.Collect(ctx)
		if err != nil {
			hostCh <- hostResult{err: err}
			return
		}
		hostCh <- hostResult{stats: res.(services.HostResult), err: nil}
	}()

	go func() {
		defer wg.Done()
		res, err := s.physicalSensor.Collect(ctx)
		if err != nil {
			physCh <- physicalResult{err: err}
			return
		}
		physCh <- physicalResult{stats: res.(services.PhysicalResult), err: nil}
	}()

	wg.Wait()

	netRes := <-netCh
	netConnRes := <-netConnCh
	healthRes := <-healthCh
	hostRes := <-hostCh
	physRes := <-physCh

	temps := []TemperatureStat{} // Initialize as empty slice
	if physRes.err == nil {
		for _, t := range physRes.stats.Temperatures {
			temps = append(temps, TemperatureStat{
				SensorKey:   t.SensorKey,
				Temperature: t.Temperature,
			})
		}
	}

	return &RawStats{
		NetLatency_ms: netRes.latency,
		IsConnected:   netRes.online,
		ActiveTCP:     netConnRes.activeTCP,
		DiskHealth:    healthRes.health,
		Hostname:      hostRes.stats.Hostname,
		OS:            hostRes.stats.OS,
		Platform:      hostRes.stats.Platform,
		KernelVersion: hostRes.stats.KernelVersion,
		Uptime:        hostRes.stats.Uptime,
		Procs:         hostRes.stats.Procs,
		Temperatures:  temps,
	}, nil
}

// Helper methods for concurrent fetching

func (s *SystemCollector) fetchLoad(wg *sync.WaitGroup, ch chan loadResult) {
	defer wg.Done()
	defer close(ch)
	avgStat, err := load.Avg()
	ch <- loadResult{avg1: avgStat.Load1, avg5: avgStat.Load5, avg15: avgStat.Load15, err: err}
}

func (s *SystemCollector) fetchNetwork(ctx context.Context, wg *sync.WaitGroup, ch chan netResult) {
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

func (s *SystemCollector) fetchNetConns(wg *sync.WaitGroup, ch chan netConnResult) {
	defer wg.Done()
	defer close(ch)
	conns, err := gnet.Connections("tcp")
	active := 0
	if err == nil {
		active = len(conns)
	}
	ch <- netConnResult{activeTCP: active, err: err}
}

func (s *SystemCollector) fetchHealth(wg *sync.WaitGroup, ch chan healthResult) {
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
	health := []DiskHealthInfo{} // Initialize as empty slice, not nil

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
