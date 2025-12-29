// Package relational provides a lightweight “ORM-ish” layer (models + repo methods)
// for storing your RawStats snapshots in DuckDB.
//
// Notes:
//   - DuckDB is columnar and loves wide fact tables + append-only inserts.
//   - This schema uses a practical normalization: keep the hot snapshot scalars in one table,
//     and put variable-length arrays in child tables keyed by snapshot_id.
//   - Dimension tables (interfaces/devices/mountpoints/sensors/containers/process names) reduce
//     repeated strings and speed up aggregations.
//
// Driver: github.com/marcboeker/go-duckdb
package relational

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math"
	"sync"
	"time"
)

// =============================================================================
// SCHEMA SQL
// =============================================================================

const SchemaSQL = `
CREATE TABLE IF NOT EXISTS hosts (
  host_id        BIGINT PRIMARY KEY,
  agent_id       VARCHAR NOT NULL UNIQUE,
  machine_id     VARCHAR,
  boot_id        VARCHAR,
  hostname       VARCHAR,
  created_at     TIMESTAMP NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS disk_devices (
  disk_device_id BIGINT PRIMARY KEY,
  host_id        BIGINT NOT NULL,
  device         VARCHAR NOT NULL,
  UNIQUE(host_id, device)
);

CREATE TABLE IF NOT EXISTS mountpoints (
  mountpoint_id  BIGINT PRIMARY KEY,
  host_id        BIGINT NOT NULL,
  mountpoint     VARCHAR NOT NULL,
  device         VARCHAR,
  fstype         VARCHAR,
  UNIQUE(host_id, mountpoint)
);

CREATE TABLE IF NOT EXISTS net_interfaces (
  net_interface_id BIGINT PRIMARY KEY,
  host_id          BIGINT NOT NULL,
  name             VARCHAR NOT NULL,
  UNIQUE(host_id, name)
);

CREATE TABLE IF NOT EXISTS temp_sensors (
  temp_sensor_id BIGINT PRIMARY KEY,
  host_id        BIGINT NOT NULL,
  sensor_key     VARCHAR NOT NULL,
  UNIQUE(host_id, sensor_key)
);

CREATE TABLE IF NOT EXISTS docker_containers (
  docker_container_key BIGINT PRIMARY KEY,
  host_id              BIGINT NOT NULL,
  container_id         VARCHAR NOT NULL,
  UNIQUE(host_id, container_id)
);

CREATE TABLE IF NOT EXISTS process_names (
  process_name_id BIGINT PRIMARY KEY,
  name            VARCHAR NOT NULL UNIQUE
);

CREATE TABLE IF NOT EXISTS snapshots (
  snapshot_id        BIGINT PRIMARY KEY,
  host_id            BIGINT NOT NULL,
  kind               VARCHAR NOT NULL,
  collected_at       TIMESTAMP NOT NULL,

  cpu_usage_pct      DOUBLE,
  load_avg_1         DOUBLE,
  load_avg_5         DOUBLE,
  load_avg_15        DOUBLE,
  cpu_model          VARCHAR,
  cpu_cores_logical  INTEGER,

  ram_usage_pct      DOUBLE,
  ram_total_bytes    BIGINT,
  ram_available_bytes BIGINT,
  ram_used_bytes     BIGINT,
  ram_free_bytes     BIGINT,
  ram_cached_bytes   BIGINT,
  ram_buffered_bytes BIGINT,

  swap_usage_pct     DOUBLE,
  swap_total_bytes   BIGINT,
  swap_used_bytes    BIGINT,

  disk_usage_pct     DOUBLE,
  disk_total_bytes   BIGINT,
  inode_usage_pct    DOUBLE,
  inode_total        BIGINT,

  net_latency_ms     DOUBLE,
  is_connected       BOOLEAN,
  active_tcp         INTEGER,

  docker_available   BOOLEAN,

  os                 VARCHAR,
  platform           VARCHAR,
  kernel_version     VARCHAR,
  uptime_seconds     BIGINT,
  procs              BIGINT,

  disk_read_bps      DOUBLE,
  disk_write_bps     DOUBLE,
  disk_read_iops     DOUBLE,
  disk_write_iops    DOUBLE,
  disk_avg_read_lat_ms DOUBLE,
  disk_avg_write_lat_ms DOUBLE,

  net_tx_bps         DOUBLE,
  net_rx_bps         DOUBLE,
  net_err_per_s      DOUBLE,
  net_drop_per_s     DOUBLE,

  severity_level     INTEGER,
  risk_score         INTEGER,
  flags_bitmask      BIGINT,

  primary_cause      VARCHAR,
  cause_entity_type  VARCHAR,
  cause_entity_key   VARCHAR,
  explanation        VARCHAR,

  flag_host_offline              BOOLEAN,
  flag_cpu_overloaded            BOOLEAN,
  flag_memory_pressure           BOOLEAN,
  flag_memory_starvation         BOOLEAN,
  flag_swap_thrashing            BOOLEAN,
  flag_disk_space_critical       BOOLEAN,
  flag_inode_exhaustion          BOOLEAN,
  flag_disk_io_saturation        BOOLEAN,
  flag_disk_health_failed        BOOLEAN,
  flag_network_latency_degraded  BOOLEAN,
  flag_network_packet_loss       BOOLEAN,
  flag_network_interface_errors  BOOLEAN,
  flag_docker_unavailable        BOOLEAN,
  flag_container_cpu_hog         BOOLEAN,
  flag_container_memory_pressure BOOLEAN,
  flag_container_oom_risk         BOOLEAN,
  flag_runaway_process_cpu       BOOLEAN,
  flag_runaway_process_memory    BOOLEAN,
  flag_thermal_pressure          BOOLEAN,
  flag_system_at_risk            BOOLEAN,

  created_at         TIMESTAMP NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS snapshot_cpu_cores (
  snapshot_id   BIGINT NOT NULL,
  core_index    INTEGER NOT NULL,
  usage_pct     DOUBLE NOT NULL,
  PRIMARY KEY(snapshot_id, core_index)
);

CREATE TABLE IF NOT EXISTS snapshot_partition_usage (
  snapshot_id      BIGINT NOT NULL,
  mountpoint_id    BIGINT NOT NULL,
  used_percent     DOUBLE,
  total_bytes      BIGINT,
  inode_usage_pct  DOUBLE,
  inode_total      BIGINT,
  PRIMARY KEY(snapshot_id, mountpoint_id)
);

CREATE TABLE IF NOT EXISTS snapshot_disk_io (
  snapshot_id     BIGINT NOT NULL,
  disk_device_id  BIGINT NOT NULL,
  read_bytes      BIGINT,
  write_bytes     BIGINT,
  read_count      BIGINT,
  write_count     BIGINT,
  read_time_ms    BIGINT,
  write_time_ms   BIGINT,
  PRIMARY KEY(snapshot_id, disk_device_id)
);

CREATE TABLE IF NOT EXISTS snapshot_disk_health (
  snapshot_id     BIGINT NOT NULL,
  disk_device_id  BIGINT NOT NULL,
  status          VARCHAR,
  message         VARCHAR,
  PRIMARY KEY(snapshot_id, disk_device_id)
);

CREATE TABLE IF NOT EXISTS snapshot_net_interface_stats (
  snapshot_id       BIGINT NOT NULL,
  net_interface_id  BIGINT NOT NULL,
  bytes_sent        BIGINT,
  bytes_recv        BIGINT,
  packets_sent      BIGINT,
  packets_recv      BIGINT,
  err_in            BIGINT,
  err_out           BIGINT,
  drop_in           BIGINT,
  drop_out          BIGINT,
  PRIMARY KEY(snapshot_id, net_interface_id)
);

CREATE TABLE IF NOT EXISTS snapshot_temperatures (
  snapshot_id    BIGINT NOT NULL,
  temp_sensor_id BIGINT NOT NULL,
  temperature_c  DOUBLE NOT NULL,
  PRIMARY KEY(snapshot_id, temp_sensor_id)
);

CREATE TABLE IF NOT EXISTS snapshot_docker_container_stats (
  snapshot_id           BIGINT NOT NULL,
  docker_container_key  BIGINT NOT NULL,
  name                  VARCHAR,
  image                 VARCHAR,
  status                VARCHAR,
  running               BOOLEAN,
  cpu_usage_pct         DOUBLE,
  mem_usage_bytes       BIGINT,
  mem_limit_bytes       BIGINT,
  mem_percent           DOUBLE,
  PRIMARY KEY(snapshot_id, docker_container_key)
);

CREATE TABLE IF NOT EXISTS snapshot_top_processes (
  snapshot_id       BIGINT NOT NULL,
  rank              INTEGER NOT NULL,
  pid               INTEGER NOT NULL,
  process_name_id   BIGINT NOT NULL,
  cpu_pct           DOUBLE,
  mem_pct           REAL,
  PRIMARY KEY(snapshot_id, rank)
);

CREATE TABLE IF NOT EXISTS current_state (
  host_id          BIGINT PRIMARY KEY,
  last_snapshot_id BIGINT,
  collected_at     TIMESTAMP,

  cpu_usage_pct    DOUBLE,
  load_avg_1       DOUBLE,
  ram_usage_pct    DOUBLE,
  ram_available_bytes BIGINT,
  swap_usage_pct   DOUBLE,
  disk_usage_pct   DOUBLE,
  inode_usage_pct  DOUBLE,
  net_latency_ms   DOUBLE,
  is_connected     BOOLEAN,
  docker_available BOOLEAN,

  disk_read_bps    DOUBLE,
  disk_write_bps   DOUBLE,
  net_tx_bps       DOUBLE,
  net_rx_bps       DOUBLE,

  severity_level   INTEGER,
  risk_score       INTEGER,
  flags_bitmask    BIGINT,
  explanation      VARCHAR,

  updated_at       TIMESTAMP NOT NULL DEFAULT now()
);
`

// =============================================================================
// REPO IMPLEMENTATION
// =============================================================================

type Repo struct {
	db *sql.DB
	mu sync.RWMutex
	// Simple in-memory cache for dimensions to reduce DB round-trips
	cache map[int64]*hostCache
}

type hostCache struct {
	diskDevice map[string]int64
	mountpoint map[string]int64
	netIf      map[string]int64
	tempSensor map[string]int64
	container  map[string]int64
	procName   map[string]int64
}

func NewRepo(db *sql.DB) *Repo {
	return &Repo{
		db:    db,
		cache: make(map[int64]*hostCache),
	}
}

func (r *Repo) Close() error {
	return r.db.Close()
}

func (r *Repo) Migrate(ctx context.Context) error {
	_, err := r.db.ExecContext(ctx, SchemaSQL)
	return err
}

// NewID generates a unique ID (time-based).
func NewID() int64 {
	return time.Now().UnixNano()
}

// UpsertHost ensures the host exists and returns its ID.
func (r *Repo) UpsertHost(ctx context.Context, agentID, machineID, bootID, hostname string) (int64, error) {
	if agentID == "" {
		return 0, errors.New("agentID required")
	}
	var hostID int64
	err := r.db.QueryRowContext(ctx, `SELECT host_id FROM hosts WHERE agent_id = ?`, agentID).Scan(&hostID)
	if err == nil {
		// Update mutable fields
		_, _ = r.db.ExecContext(ctx, `
			UPDATE hosts
			SET machine_id = COALESCE(NULLIF(?,''), machine_id),
			    boot_id    = COALESCE(NULLIF(?,''), boot_id),
			    hostname   = COALESCE(NULLIF(?,''), hostname)
			WHERE host_id = ?
		`, machineID, bootID, hostname, hostID)
		return hostID, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return 0, err
	}

	hostID = NewID()
	_, err = r.db.ExecContext(ctx, `INSERT INTO hosts(host_id, agent_id, machine_id, boot_id, hostname) VALUES(?,?,?,?,?)`,
		hostID, agentID, nullEmpty(machineID), nullEmpty(bootID), nullEmpty(hostname),
	)
	if err != nil {
		// Race condition fallback
		if e2 := r.db.QueryRowContext(ctx, `SELECT host_id FROM hosts WHERE agent_id = ?`, agentID).Scan(&hostID); e2 == nil {
			return hostID, nil
		}
		return 0, err
	}
	return hostID, nil
}

// GetDerivedRates computes rates based on the previous snapshot.
func (r *Repo) GetDerivedRates(ctx context.Context, current RawStatsFixed) (*DerivedRates, error) {
	// We need the hostID first
	hostID, err := r.UpsertHost(ctx, current.AgentID, current.MachineID, current.BootID, current.Hostname)
	if err != nil {
		return nil, err
	}

	// Fetch previous snapshot counters
	prev, err := r.getPrevCounters(ctx, hostID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return &DerivedRates{}, nil // First run
		}
		return nil, err
	}

	return ComputeDerivedRates(prev, current), nil
}

// InsertRawStats persists the snapshot.
func (r *Repo) InsertRawStats(ctx context.Context, s RawStatsFixed, d DerivedRates, f SnapshotFlags) (InsertResult, error) {
	hostID, err := r.UpsertHost(ctx, s.AgentID, s.MachineID, s.BootID, s.Hostname)
	if err != nil {
		return InsertResult{}, err
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return InsertResult{}, err
	}
	defer func() { _ = tx.Rollback() }()

	snapshotID := NewID()

	// Insert Snapshot
	_, err = tx.ExecContext(ctx, `
		INSERT INTO snapshots(
		  snapshot_id, host_id, kind, collected_at,
		  cpu_usage_pct, load_avg_1, load_avg_5, load_avg_15, cpu_model, cpu_cores_logical,
		  ram_usage_pct, ram_total_bytes, ram_available_bytes, ram_used_bytes, ram_free_bytes, ram_cached_bytes, ram_buffered_bytes,
		  swap_usage_pct, swap_total_bytes, swap_used_bytes,
		  disk_usage_pct, disk_total_bytes, inode_usage_pct, inode_total,
		  net_latency_ms, is_connected, active_tcp,
		  docker_available,
		  os, platform, kernel_version, uptime_seconds, procs,
		  disk_read_bps, disk_write_bps, disk_read_iops, disk_write_iops, disk_avg_read_lat_ms, disk_avg_write_lat_ms,
		  net_tx_bps, net_rx_bps, net_err_per_s, net_drop_per_s,
		  severity_level, risk_score, flags_bitmask,
		  primary_cause, cause_entity_type, cause_entity_key, explanation,
		  flag_host_offline, flag_cpu_overloaded, flag_memory_pressure, flag_memory_starvation, flag_swap_thrashing,
		  flag_disk_space_critical, flag_inode_exhaustion, flag_disk_io_saturation, flag_disk_health_failed,
		  flag_network_latency_degraded, flag_network_packet_loss, flag_network_interface_errors,
		  flag_docker_unavailable, flag_container_cpu_hog, flag_container_memory_pressure, flag_container_oom_risk,
		  flag_runaway_process_cpu, flag_runaway_process_memory, flag_thermal_pressure, flag_system_at_risk
		) VALUES (
		  ?,?,?,?,
		  ?,?,?,?,?, ?,
		  ?,?,?,?,?,?,?,
		  ?,?,?,
		  ?,?,?,?,
		  ?,?,?,
		  ?,
		  ?,?,?,?,?,
		  ?,?,?,?,?, ?,
		  ?,?,?,?,
		  ?,?,?,
		  ?,?,?,?,
		  ?,?,?,?,?, ?,?,?,?, ?,?,?, ?,?,?,?, ?,?,?,?
		)
	`,
		snapshotID, hostID, string(s.Kind), s.CollectedAt,
		nullFloat(s.CPUUsagePct), nullFloat(s.LoadAvg1), nullFloat(s.LoadAvg5), nullFloat(s.LoadAvg15), nullStr(s.CPUModel), nullInt(int64(s.CPUCoresLogical)),
		nullFloat(s.RAMUsagePct), nullUInt64(s.RAMTotalBytes), nullUInt64(s.RAMAvailableBytes), nullUInt64(s.RAMUsedBytes), nullUInt64(s.RAMFreeBytes), nullUInt64(s.RAMCachedBytes), nullUInt64(s.RAMBufferedBytes),
		nullFloat(s.SwapUsagePct), nullUInt64(s.SwapTotalBytes), nullUInt64(s.SwapUsedBytes),
		nullFloat(s.DiskUsagePct), nullUInt64(s.DiskTotalBytes), nullFloat(s.InodeUsagePct), nullUInt64(s.InodeTotal),
		nullFloat(s.NetLatencyMS), s.IsConnected, nullInt(int64(s.ActiveTCP)),
		s.DockerAvailable,
		nullStr(s.OS), nullStr(s.Platform), nullStr(s.KernelVersion), nullUInt64(s.UptimeSeconds), nullUInt64(s.Procs),
		nullFloat(d.DiskReadBps), nullFloat(d.DiskWriteBps), nullFloat(d.DiskReadIops), nullFloat(d.DiskWriteIops), nullFloat(d.DiskAvgReadLatMs), nullFloat(d.DiskAvgWriteLatMs),
		nullFloat(d.NetTxBps), nullFloat(d.NetRxBps), nullFloat(d.NetErrPerS), nullFloat(d.NetDropPerS),
		f.SeverityLevel, f.RiskScore, f.Bitmask,
		nullStr(f.PrimaryCause), nullStr(f.CauseEntityType), nullStr(f.CauseEntityKey), nullStr(f.Explanation),
		f.FlagHostOffline, f.FlagCPUOverloaded, f.FlagMemoryPressure, f.FlagMemoryStarvation, f.FlagSwapThrashing,
		f.FlagDiskSpaceCritical, f.FlagInodeExhaustion, f.FlagDiskIOSaturation, f.FlagDiskHealthFailed,
		f.FlagNetworkLatencyDegraded, f.FlagNetworkPacketLoss, f.FlagNetworkInterfaceErrors,
		f.FlagDockerUnavailable, f.FlagContainerCPUHog, f.FlagContainerMemoryPressure, f.FlagContainerOOMRisk,
		f.FlagRunawayProcessCPU, f.FlagRunawayProcessMemory, f.FlagThermalPressure, f.FlagSystemAtRisk,
	)
	if err != nil {
		return InsertResult{}, fmt.Errorf("insert snapshot: %w", err)
	}

	// Insert Children
	if err := r.insertChildrenTx(ctx, tx, hostID, snapshotID, s); err != nil {
		return InsertResult{}, err
	}

	// Update Current State
	_, err = tx.ExecContext(ctx, `
		INSERT INTO current_state(
		  host_id, last_snapshot_id, collected_at,
		  cpu_usage_pct, load_avg_1, ram_usage_pct, ram_available_bytes, swap_usage_pct,
		  disk_usage_pct, inode_usage_pct, net_latency_ms, is_connected, docker_available,
		  disk_read_bps, disk_write_bps, net_tx_bps, net_rx_bps,
		  severity_level, risk_score, flags_bitmask, explanation
		) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)
		ON CONFLICT(host_id) DO UPDATE SET
		  last_snapshot_id     = excluded.last_snapshot_id,
		  collected_at         = excluded.collected_at,
		  cpu_usage_pct        = excluded.cpu_usage_pct,
		  load_avg_1           = excluded.load_avg_1,
		  ram_usage_pct        = excluded.ram_usage_pct,
		  ram_available_bytes  = excluded.ram_available_bytes,
		  swap_usage_pct       = excluded.swap_usage_pct,
		  disk_usage_pct       = excluded.disk_usage_pct,
		  inode_usage_pct      = excluded.inode_usage_pct,
		  net_latency_ms       = excluded.net_latency_ms,
		  is_connected         = excluded.is_connected,
		  docker_available     = excluded.docker_available,
		  disk_read_bps        = excluded.disk_read_bps,
		  disk_write_bps       = excluded.disk_write_bps,
		  net_tx_bps           = excluded.net_tx_bps,
		  net_rx_bps           = excluded.net_rx_bps,
		  severity_level       = excluded.severity_level,
		  risk_score           = excluded.risk_score,
		  flags_bitmask        = excluded.flags_bitmask,
		  explanation          = excluded.explanation,
		  updated_at           = now()
	`,
		hostID, snapshotID, s.CollectedAt,
		nullFloat(s.CPUUsagePct), nullFloat(s.LoadAvg1), nullFloat(s.RAMUsagePct), nullUInt64(s.RAMAvailableBytes), nullFloat(s.SwapUsagePct),
		nullFloat(s.DiskUsagePct), nullFloat(s.InodeUsagePct), nullFloat(s.NetLatencyMS), s.IsConnected, s.DockerAvailable,
		nullFloat(d.DiskReadBps), nullFloat(d.DiskWriteBps), nullFloat(d.NetTxBps), nullFloat(d.NetRxBps),
		f.SeverityLevel, f.RiskScore, f.Bitmask, nullStr(f.Explanation),
	)
	if err != nil {
		return InsertResult{}, fmt.Errorf("update current_state: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return InsertResult{}, err
	}

	return InsertResult{SnapshotID: snapshotID, HostID: hostID}, nil
}

func (r *Repo) GetCurrentState(ctx context.Context, hostID int64) (map[string]any, error) {
	// Implementation omitted for brevity, similar to previous
	return nil, nil
}

// =============================================================================
// HELPERS
// =============================================================================

type PrevCounters struct {
	CollectedAt     time.Time
	DiskReadBytes   uint64
	DiskWriteBytes  uint64
	DiskReadCount   uint64
	DiskWriteCount  uint64
	DiskReadTimeMS  uint64
	DiskWriteTimeMS uint64
	NetBytesSent    uint64
	NetBytesRecv    uint64
	NetErrIn        uint64
	NetErrOut       uint64
	NetDropIn       uint64
	NetDropOut      uint64
}

func (r *Repo) getPrevCounters(ctx context.Context, hostID int64) (PrevCounters, error) {
	var sid int64
	var t time.Time
	err := r.db.QueryRowContext(ctx, `
		SELECT snapshot_id, collected_at
		FROM snapshots
		WHERE host_id = ?
		ORDER BY collected_at DESC
		LIMIT 1
	`, hostID).Scan(&sid, &t)
	if err != nil {
		return PrevCounters{}, err
	}

	var prev PrevCounters
	prev.CollectedAt = t

	// Sum disk counters
	_ = r.db.QueryRowContext(ctx, `
		SELECT
		  COALESCE(SUM(read_bytes),0), COALESCE(SUM(write_bytes),0),
		  COALESCE(SUM(read_count),0), COALESCE(SUM(write_count),0),
		  COALESCE(SUM(read_time_ms),0), COALESCE(SUM(write_time_ms),0)
		FROM snapshot_disk_io WHERE snapshot_id = ?
	`, sid).Scan(&prev.DiskReadBytes, &prev.DiskWriteBytes, &prev.DiskReadCount, &prev.DiskWriteCount, &prev.DiskReadTimeMS, &prev.DiskWriteTimeMS)

	// Sum net counters
	_ = r.db.QueryRowContext(ctx, `
		SELECT
		  COALESCE(SUM(bytes_sent),0), COALESCE(SUM(bytes_recv),0),
		  COALESCE(SUM(err_in),0), COALESCE(SUM(err_out),0),
		  COALESCE(SUM(drop_in),0), COALESCE(SUM(drop_out),0)
		FROM snapshot_net_interface_stats WHERE snapshot_id = ?
	`, sid).Scan(&prev.NetBytesSent, &prev.NetBytesRecv, &prev.NetErrIn, &prev.NetErrOut, &prev.NetDropIn, &prev.NetDropOut)

	return prev, nil
}

func ComputeDerivedRates(prev PrevCounters, now RawStatsFixed) *DerivedRates {
	if prev.CollectedAt.IsZero() {
		return &DerivedRates{}
	}
	dt := now.CollectedAt.Sub(prev.CollectedAt).Seconds()
	if dt <= 0.1 {
		return &DerivedRates{}
	}

	// Sum current
	var cur PrevCounters
	for _, io := range now.IOCounters {
		cur.DiskReadBytes += io.ReadBytes
		cur.DiskWriteBytes += io.WriteBytes
		cur.DiskReadCount += io.ReadCount
		cur.DiskWriteCount += io.WriteCount
		cur.DiskReadTimeMS += io.ReadTimeMS
		cur.DiskWriteTimeMS += io.WriteTimeMS
	}
	for _, ni := range now.NetInterfaces {
		cur.NetBytesSent += ni.BytesSent
		cur.NetBytesRecv += ni.BytesRecv
		cur.NetErrIn += ni.ErrIn
		cur.NetErrOut += ni.ErrOut
		cur.NetDropIn += ni.DropIn
		cur.NetDropOut += ni.DropOut
	}

	d := &DerivedRates{
		DiskReadBps:   rate(prev.DiskReadBytes, cur.DiskReadBytes, dt),
		DiskWriteBps:  rate(prev.DiskWriteBytes, cur.DiskWriteBytes, dt),
		DiskReadIops:  rate(prev.DiskReadCount, cur.DiskReadCount, dt),
		DiskWriteIops: rate(prev.DiskWriteCount, cur.DiskWriteCount, dt),
		NetTxBps:      rate(prev.NetBytesSent, cur.NetBytesSent, dt),
		NetRxBps:      rate(prev.NetBytesRecv, cur.NetBytesRecv, dt),
		NetErrPerS:    rate(prev.NetErrIn+prev.NetErrOut, cur.NetErrIn+cur.NetErrOut, dt),
		NetDropPerS:   rate(prev.NetDropIn+prev.NetDropOut, cur.NetDropIn+cur.NetDropOut, dt),
	}

	// Latency
	dReadC := delta(prev.DiskReadCount, cur.DiskReadCount)
	dWriteC := delta(prev.DiskWriteCount, cur.DiskWriteCount)
	if dReadC > 0 {
		d.DiskAvgReadLatMs = float64(delta(prev.DiskReadTimeMS, cur.DiskReadTimeMS)) / float64(dReadC)
	}
	if dWriteC > 0 {
		d.DiskAvgWriteLatMs = float64(delta(prev.DiskWriteTimeMS, cur.DiskWriteTimeMS)) / float64(dWriteC)
	}

	return d
}

func rate(prev, cur uint64, dt float64) float64 {
	return float64(delta(prev, cur)) / dt
}

func delta(prev, cur uint64) uint64 {
	if cur >= prev {
		return cur - prev
	}
	return 0 // reset
}

func (r *Repo) insertChildrenTx(ctx context.Context, tx *sql.Tx, hostID, snapshotID int64, s RawStatsFixed) error {
	// CPU Cores
	if len(s.CPUPerCorePct) > 0 {
		stmt, err := tx.PrepareContext(ctx, `INSERT INTO snapshot_cpu_cores(snapshot_id, core_index, usage_pct) VALUES(?,?,?)`)
		if err != nil {
			return err
		}
		defer stmt.Close()
		for i, u := range s.CPUPerCorePct {
			if _, err := stmt.ExecContext(ctx, snapshotID, i, u); err != nil {
				return err
			}
		}
	}
	// Partitions
	if len(s.Partitions) > 0 {
		stmt, err := tx.PrepareContext(ctx, `INSERT INTO snapshot_partition_usage(snapshot_id, mountpoint_id, used_percent, total_bytes, inode_usage_pct, inode_total) VALUES(?,?,?,?,?,?)`)
		if err != nil {
			return err
		}
		defer stmt.Close()
		for _, p := range s.Partitions {
			mpID, err := r.upsertMountpointTx(ctx, tx, hostID, p.Mountpoint, p.Device, p.Fstype)
			if err != nil {
				return err
			}
			if _, err := stmt.ExecContext(ctx, snapshotID, mpID, nullFloat(p.UsedPercent), nullUInt64(p.TotalBytes), nullFloat(p.InodeUsage), nullUInt64(p.TotalInodes)); err != nil {
				return err
			}
		}
	}
	// Disk IO
	if len(s.IOCounters) > 0 {
		stmt, err := tx.PrepareContext(ctx, `INSERT INTO snapshot_disk_io(snapshot_id, disk_device_id, read_bytes, write_bytes, read_count, write_count, read_time_ms, write_time_ms) VALUES(?,?,?,?,?,?,?,?)`)
		if err != nil {
			return err
		}
		defer stmt.Close()
		for _, io := range s.IOCounters {
			devID, err := r.upsertDiskDeviceTx(ctx, tx, hostID, io.Device)
			if err != nil {
				return err
			}
			if _, err := stmt.ExecContext(ctx, snapshotID, devID, io.ReadBytes, io.WriteBytes, io.ReadCount, io.WriteCount, io.ReadTimeMS, io.WriteTimeMS); err != nil {
				return err
			}
		}
	}
	// Disk Health
	if len(s.DiskHealth) > 0 {
		stmt, err := tx.PrepareContext(ctx, `INSERT INTO snapshot_disk_health(snapshot_id, disk_device_id, status, message) VALUES(?,?,?,?)`)
		if err != nil {
			return err
		}
		defer stmt.Close()
		for _, h := range s.DiskHealth {
			devID, err := r.upsertDiskDeviceTx(ctx, tx, hostID, h.Device)
			if err != nil {
				return err
			}
			if _, err := stmt.ExecContext(ctx, snapshotID, devID, nullStr(h.Status), nullStr(h.Message)); err != nil {
				return err
			}
		}
	}
	// Net Interfaces
	if len(s.NetInterfaces) > 0 {
		stmt, err := tx.PrepareContext(ctx, `INSERT INTO snapshot_net_interface_stats(snapshot_id, net_interface_id, bytes_sent, bytes_recv, packets_sent, packets_recv, err_in, err_out, drop_in, drop_out) VALUES(?,?,?,?,?,?,?,?,?,?)`)
		if err != nil {
			return err
		}
		defer stmt.Close()
		for _, ni := range s.NetInterfaces {
			ifID, err := r.upsertNetInterfaceTx(ctx, tx, hostID, ni.Name)
			if err != nil {
				return err
			}
			if _, err := stmt.ExecContext(ctx, snapshotID, ifID, ni.BytesSent, ni.BytesRecv, ni.PacketsSent, ni.PacketsRecv, ni.ErrIn, ni.ErrOut, ni.DropIn, ni.DropOut); err != nil {
				return err
			}
		}
	}
	// Temperatures
	if len(s.Temperatures) > 0 {
		stmt, err := tx.PrepareContext(ctx, `INSERT INTO snapshot_temperatures(snapshot_id, temp_sensor_id, temperature_c) VALUES(?,?,?)`)
		if err != nil {
			return err
		}
		defer stmt.Close()
		for _, t := range s.Temperatures {
			sID, err := r.upsertTempSensorTx(ctx, tx, hostID, t.SensorKey)
			if err != nil {
				return err
			}
			if _, err := stmt.ExecContext(ctx, snapshotID, sID, t.TemperatureC); err != nil {
				return err
			}
		}
	}
	// Docker
	if len(s.DockerContainers) > 0 {
		stmt, err := tx.PrepareContext(ctx, `INSERT INTO snapshot_docker_container_stats(snapshot_id, docker_container_key, name, image, status, running, cpu_usage_pct, mem_usage_bytes, mem_limit_bytes, mem_percent) VALUES(?,?,?,?,?,?,?,?,?,?)`)
		if err != nil {
			return err
		}
		defer stmt.Close()
		for _, c := range s.DockerContainers {
			key, err := r.upsertDockerContainerTx(ctx, tx, hostID, c.ID)
			if err != nil {
				return err
			}
			if _, err := stmt.ExecContext(ctx, snapshotID, key, nullStr(c.Name), nullStr(c.Image), nullStr(c.Status), c.Running, nullFloat(c.CPUUsagePct), nullUInt64(c.MemUsageBytes), nullUInt64(c.MemLimitBytes), nullFloat(c.MemPercent)); err != nil {
				return err
			}
		}
	}
	// Processes
	if len(s.TopProcesses) > 0 {
		stmt, err := tx.PrepareContext(ctx, `INSERT INTO snapshot_top_processes(snapshot_id, rank, pid, process_name_id, cpu_pct, mem_pct) VALUES(?,?,?,?,?,?)`)
		if err != nil {
			return err
		}
		defer stmt.Close()
		for _, p := range s.TopProcesses {
			pnID, err := r.upsertProcessNameTx(ctx, tx, p.Name)
			if err != nil {
				return err
			}
			if _, err := stmt.ExecContext(ctx, snapshotID, p.Rank, p.PID, pnID, nullFloat(p.CPUPct), nullFloat(float64(p.MemPct))); err != nil {
				return err
			}
		}
	}
	return nil
}

// Dimension Upserts (simplified)
func (r *Repo) ensureCache(hostID int64) *hostCache {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.cache[hostID]; !ok {
		r.cache[hostID] = &hostCache{
			diskDevice: make(map[string]int64),
			mountpoint: make(map[string]int64),
			netIf:      make(map[string]int64),
			tempSensor: make(map[string]int64),
			container:  make(map[string]int64),
			procName:   make(map[string]int64),
		}
	}
	return r.cache[hostID]
}

func (r *Repo) upsertDim(ctx context.Context, tx *sql.Tx, hostID int64, cache map[string]int64, key string, querySel, queryIns string, argsIns ...any) (int64, error) {
	r.mu.RLock()
	if id, ok := cache[key]; ok {
		r.mu.RUnlock()
		return id, nil
	}
	r.mu.RUnlock()

	var id int64
	err := tx.QueryRowContext(ctx, querySel, hostID, key).Scan(&id)
	if err == nil {
		r.mu.Lock()
		cache[key] = id
		r.mu.Unlock()
		return id, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return 0, err
	}

	id = NewID()
	args := append([]any{id}, argsIns...)
	_, err = tx.ExecContext(ctx, queryIns, args...)
	if err != nil {
		if e2 := tx.QueryRowContext(ctx, querySel, hostID, key).Scan(&id); e2 == nil {
			r.mu.Lock()
			cache[key] = id
			r.mu.Unlock()
			return id, nil
		}
		return 0, err
	}
	r.mu.Lock()
	cache[key] = id
	r.mu.Unlock()
	return id, nil
}

func (r *Repo) upsertDiskDeviceTx(ctx context.Context, tx *sql.Tx, hostID int64, device string) (int64, error) {
	return r.upsertDim(ctx, tx, hostID, r.ensureCache(hostID).diskDevice, device,
		`SELECT disk_device_id FROM disk_devices WHERE host_id=? AND device=?`,
		`INSERT INTO disk_devices(disk_device_id, host_id, device) VALUES(?,?,?)`,
		hostID, device)
}

func (r *Repo) upsertMountpointTx(ctx context.Context, tx *sql.Tx, hostID int64, mp, dev, fs string) (int64, error) {
	return r.upsertDim(ctx, tx, hostID, r.ensureCache(hostID).mountpoint, mp,
		`SELECT mountpoint_id FROM mountpoints WHERE host_id=? AND mountpoint=?`,
		`INSERT INTO mountpoints(mountpoint_id, host_id, mountpoint, device, fstype) VALUES(?,?,?,?,?)`,
		hostID, mp, nullEmpty(dev), nullEmpty(fs))
}

func (r *Repo) upsertNetInterfaceTx(ctx context.Context, tx *sql.Tx, hostID int64, name string) (int64, error) {
	return r.upsertDim(ctx, tx, hostID, r.ensureCache(hostID).netIf, name,
		`SELECT net_interface_id FROM net_interfaces WHERE host_id=? AND name=?`,
		`INSERT INTO net_interfaces(net_interface_id, host_id, name) VALUES(?,?,?)`,
		hostID, name)
}

func (r *Repo) upsertTempSensorTx(ctx context.Context, tx *sql.Tx, hostID int64, key string) (int64, error) {
	return r.upsertDim(ctx, tx, hostID, r.ensureCache(hostID).tempSensor, key,
		`SELECT temp_sensor_id FROM temp_sensors WHERE host_id=? AND sensor_key=?`,
		`INSERT INTO temp_sensors(temp_sensor_id, host_id, sensor_key) VALUES(?,?,?)`,
		hostID, key)
}

func (r *Repo) upsertDockerContainerTx(ctx context.Context, tx *sql.Tx, hostID int64, cid string) (int64, error) {
	return r.upsertDim(ctx, tx, hostID, r.ensureCache(hostID).container, cid,
		`SELECT docker_container_key FROM docker_containers WHERE host_id=? AND container_id=?`,
		`INSERT INTO docker_containers(docker_container_key, host_id, container_id) VALUES(?,?,?)`,
		hostID, cid)
}

func (r *Repo) upsertProcessNameTx(ctx context.Context, tx *sql.Tx, name string) (int64, error) {
	// Process names are global, but we use host cache for simplicity or need global cache
	// For now, just use a separate query without host_id in WHERE
	// But upsertDim expects hostID.
	// Let's just implement it manually.
	hc := r.ensureCache(0) // Use 0 for global
	r.mu.RLock()
	if id, ok := hc.procName[name]; ok {
		r.mu.RUnlock()
		return id, nil
	}
	r.mu.RUnlock()

	var id int64
	err := tx.QueryRowContext(ctx, `SELECT process_name_id FROM process_names WHERE name=?`, name).Scan(&id)
	if err == nil {
		r.mu.Lock()
		hc.procName[name] = id
		r.mu.Unlock()
		return id, nil
	}

	id = NewID()
	_, err = tx.ExecContext(ctx, `INSERT INTO process_names(process_name_id, name) VALUES(?,?)`, id, name)
	if err != nil {
		if e2 := tx.QueryRowContext(ctx, `SELECT process_name_id FROM process_names WHERE name=?`, name).Scan(&id); e2 == nil {
			r.mu.Lock()
			hc.procName[name] = id
			r.mu.Unlock()
			return id, nil
		}
		return 0, err
	}
	r.mu.Lock()
	hc.procName[name] = id
	r.mu.Unlock()
	return id, nil
}

// Null helpers
func nullStr(s string) sql.NullString {
	if s == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: s, Valid: true}
}

func nullEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func nullFloat(v float64) sql.NullFloat64 {
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return sql.NullFloat64{}
	}
	return sql.NullFloat64{Float64: v, Valid: true}
}

func nullInt(v int64) sql.NullInt64 {
	return sql.NullInt64{Int64: v, Valid: true}
}

func nullUInt64(v uint64) sql.NullInt64 {
	return sql.NullInt64{Int64: int64(v), Valid: true}
}
