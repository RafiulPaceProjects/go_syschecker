package relational

import "time"

// ==========================
// 1) CORE / DIM TABLES
// ==========================

type Host struct {
	HostID    int64
	AgentID   string // REQUIRED stable ID
	MachineID string // optional
	BootID    string // optional
	Hostname  string // optional
	CreatedAt time.Time
}

type DiskDevice struct {
	DiskDeviceID int64
	HostID       int64
	Device       string // /dev/sda1 etc
	// UNIQUE(host_id, device)
}

type Mountpoint struct {
	MountpointID int64
	HostID       int64
	Mountpoint   string // /mnt/data
	Device       string // /dev/sda1
	Fstype       string // ext4
	// UNIQUE(host_id, mountpoint)
}

type NetInterface struct {
	NetInterfaceID int64
	HostID         int64
	Name           string // eth0
	// UNIQUE(host_id, name)
}

type TempSensor struct {
	TempSensorID int64
	HostID       int64
	SensorKey    string // coretemp_package_id_0 etc
	// UNIQUE(host_id, sensor_key)
}

type DockerContainer struct {
	DockerContainerKey int64
	HostID             int64
	ContainerID        string // stable container id
	// UNIQUE(host_id, container_id)
}

type ProcessName struct {
	ProcessNameID int64
	Name          string // normalized dictionary of process names
	// UNIQUE(name)
}

// ==========================
// 2) SNAPSHOT FACT TABLE
// ==========================

type Snapshot struct {
	SnapshotID  int64
	HostID      int64
	Kind        string // "fast" | "slow" | "merged"
	CollectedAt time.Time

	// ---- Raw CPU ----
	CPUUsagePct     float64
	LoadAvg1        float64
	LoadAvg5        float64
	LoadAvg15       float64
	CPUModel        string
	CPUCoresLogical int32

	// ---- Raw RAM (bytes) ----
	RAMUsagePct       float64
	RAMTotalBytes     int64
	RAMAvailableBytes int64
	RAMUsedBytes      int64
	RAMFreeBytes      int64
	RAMCachedBytes    int64
	RAMBufferedBytes  int64

	// ---- Raw Swap (bytes) ----
	SwapUsagePct   float64
	SwapTotalBytes int64
	SwapUsedBytes  int64

	// ---- Raw Disk root "/" (bytes + inodes) ----
	DiskUsagePct   float64
	DiskTotalBytes int64
	InodeUsagePct  float64
	InodeTotal     int64

	// ---- Network probe ----
	NetLatencyMS float64
	IsConnected  bool
	ActiveTCP    int32

	// ---- Docker availability ----
	DockerAvailable bool

	// ---- Host info ----
	OS            string
	Platform      string
	KernelVersion string
	UptimeSeconds int64
	Procs         int64

	// ---- Derived rates (from deltas of counters) ----
	DiskReadBps       float64
	DiskWriteBps      float64
	DiskReadIops      float64
	DiskWriteIops     float64
	DiskAvgReadLatMS  float64
	DiskAvgWriteLatMS float64

	NetTxBps    float64
	NetRxBps    float64
	NetErrPerS  float64
	NetDropPerS float64

	// ---- Scoring / explanation ----
	SeverityLevel int32 // 0..4
	RiskScore     int32 // 0..100
	FlagsBitmask  int64

	PrimaryCause    string // cpu|memory|disk|network|docker|thermal|unknown
	CauseEntityType string // container|process|disk|netif|mount|sensor|none
	CauseEntityKey  string // container_id, process name, device, interface name...
	Explanation     string // short human explanation

	// ---- Boolean flags (fast WHERE filtering) ----
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

	CreatedAt time.Time
}

// ==========================
// 3) CHILD FACT TABLES
// ==========================

type SnapshotCPUCore struct {
	SnapshotID int64
	CoreIndex  int32
	UsagePct   float64
	// Per-core usage
}

type SnapshotPartitionUsage struct {
	SnapshotID    int64
	MountpointID  int64
	UsedPercent   float64
	TotalBytes    int64
	InodeUsagePct float64
	InodeTotal    int64
	// Partition usage mapped to mountpoint dimension
}

type SnapshotDiskIO struct {
	SnapshotID   int64
	DiskDeviceID int64
	ReadBytes    int64
	WriteBytes   int64
	ReadCount    int64
	WriteCount   int64
	ReadTimeMS   int64
	WriteTimeMS  int64
	// Disk IO mapped to disk device dimension (cumulative counters since boot)
}

type SnapshotDiskHealth struct {
	SnapshotID   int64
	DiskDeviceID int64
	Status       string // passed|failed|unknown
	Message      string // parsed summary
	// Disk health mapped to disk device dimension
}

type SnapshotNetInterfaceStats struct {
	SnapshotID     int64
	NetInterfaceID int64
	BytesSent      int64
	BytesRecv      int64
	PacketsSent    int64
	PacketsRecv    int64
	ErrIn          int64
	ErrOut         int64
	DropIn         int64
	DropOut        int64
	// Net interface stats (cumulative counters since boot)
}

type SnapshotTemperature struct {
	SnapshotID   int64
	TempSensorID int64
	TemperatureC float64
	// Temperature readings mapped to temp sensor dimension
}

type SnapshotDockerContainerStats struct {
	SnapshotID         int64
	DockerContainerKey int64
	Name               string
	Image              string
	Status             string
	Running            bool
	CPUUsagePct        float64
	MemUsageBytes      int64
	MemLimitBytes      int64
	MemPercent         float64
	// Container stats mapped to docker container dimension
}

type SnapshotTopProcess struct {
	SnapshotID    int64
	Rank          int32 // 1..N
	PID           int32
	ProcessNameID int64
	CPUPct        float64
	MemPct        float32
	// Top processes mapped to process name dictionary
}

// ==========================
// 4) CURRENT STATE (optional)
// ==========================

type CurrentState struct {
	HostID         int64
	LastSnapshotID int64
	CollectedAt    time.Time

	CPUUsagePct       float64
	LoadAvg1          float64
	RAMUsagePct       float64
	RAMAvailableBytes int64
	SwapUsagePct      float64
	DiskUsagePct      float64
	InodeUsagePct     float64
	NetLatencyMS      float64
	IsConnected       bool
	DockerAvailable   bool

	DiskReadBps  float64
	DiskWriteBps float64
	NetTxBps     float64
	NetRxBps     float64

	SeverityLevel int32
	RiskScore     int32
	FlagsBitmask  int64
	Explanation   string
	UpdatedAt     time.Time
}
