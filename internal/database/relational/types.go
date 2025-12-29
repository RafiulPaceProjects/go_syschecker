package relational

import "time"

type SnapshotKind string

const (
	KindFast   SnapshotKind = "fast"
	KindSlow   SnapshotKind = "slow"
	KindMerged SnapshotKind = "merged"
)

// RawStatsFixed is the “fixed” version of RawStats for storage/processing.
type RawStatsFixed struct {
	CollectedAt time.Time
	Kind        SnapshotKind

	// Host identity (stable)
	AgentID   string
	MachineID string
	BootID    string
	Hostname  string

	// CPU
	CPUUsagePct     float64
	CPUPerCorePct   []float64
	LoadAvg1        float64
	LoadAvg5        float64
	LoadAvg15       float64
	CPUModel        string
	CPUCoresLogical int

	// RAM bytes
	RAMUsagePct       float64
	RAMTotalBytes     uint64
	RAMAvailableBytes uint64
	RAMUsedBytes      uint64
	RAMFreeBytes      uint64
	RAMCachedBytes    uint64
	RAMBufferedBytes  uint64

	// Swap bytes
	SwapUsagePct   float64
	SwapTotalBytes uint64
	SwapUsedBytes  uint64

	// Disk root "/" bytes + inodes
	DiskUsagePct   float64
	DiskTotalBytes uint64
	InodeUsagePct  float64
	InodeTotal     uint64

	// Disk details
	Partitions []PartitionUsageFixed
	IOCounters []DiskIOCountersFixed
	DiskHealth []DiskHealthInfoFixed

	// Network
	NetLatencyMS  float64
	IsConnected   bool
	ActiveTCP     int
	NetInterfaces []NetInterfaceStatsFixed

	// Docker
	DockerAvailable  bool
	DockerContainers []DockerContainerInfoFixed

	// Host info
	OS            string
	Platform      string
	KernelVersion string
	UptimeSeconds uint64
	Procs         uint64

	// Physical
	Temperatures []TemperatureStatFixed

	// Processes (top N)
	TopProcesses []ProcessStatFixed
}

type DockerContainerInfoFixed struct {
	ID            string
	Name          string
	Image         string
	Status        string
	Running       bool
	CPUUsagePct   float64
	MemUsageBytes uint64
	MemLimitBytes uint64
	MemPercent    float64
}

type TemperatureStatFixed struct {
	SensorKey    string
	TemperatureC float64
}

type ProcessStatFixed struct {
	Rank   int
	PID    int32
	Name   string
	CPUPct float64
	MemPct float32
}

type NetInterfaceStatsFixed struct {
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

type PartitionUsageFixed struct {
	Mountpoint  string
	Device      string
	Fstype      string
	UsedPercent float64
	TotalBytes  uint64
	InodeUsage  float64
	TotalInodes uint64
}

type DiskIOCountersFixed struct {
	Device      string
	ReadBytes   uint64
	WriteBytes  uint64
	ReadCount   uint64
	WriteCount  uint64
	ReadTimeMS  uint64
	WriteTimeMS uint64
}

type DiskHealthInfoFixed struct {
	Device  string
	Status  string // passed|failed|unknown
	Message string
}

// DerivedRates contains rates computed from deltas.
type DerivedRates struct {
	DiskReadBps       float64
	DiskWriteBps      float64
	DiskReadIops      float64
	DiskWriteIops     float64
	DiskAvgReadLatMs  float64
	DiskAvgWriteLatMs float64
	NetTxBps          float64
	NetRxBps          float64
	NetErrPerS        float64
	NetDropPerS       float64
}

// SnapshotFlags contains analysis results.
type SnapshotFlags struct {
	FlagHostOffline             bool
	FlagCPUOverloaded           bool
	FlagMemoryPressure          bool
	FlagMemoryStarvation        bool
	FlagSwapThrashing           bool
	FlagDiskSpaceCritical       bool
	FlagInodeExhaustion         bool
	FlagDiskIOSaturation        bool
	FlagDiskHealthFailed        bool
	FlagNetworkLatencyDegraded  bool
	FlagNetworkPacketLoss       bool
	FlagNetworkInterfaceErrors  bool
	FlagDockerUnavailable       bool
	FlagContainerCPUHog         bool
	FlagContainerMemoryPressure bool
	FlagContainerOOMRisk        bool
	FlagRunawayProcessCPU       bool
	FlagRunawayProcessMemory    bool
	FlagThermalPressure         bool
	FlagSystemAtRisk            bool

	SeverityLevel int
	RiskScore     int
	Bitmask       int64

	PrimaryCause    string
	CauseEntityType string
	CauseEntityKey  string
	Explanation     string
}

// InsertResult contains IDs of inserted records.
type InsertResult struct {
	SnapshotID int64
	HostID     int64
}
