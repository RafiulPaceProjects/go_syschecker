# Sensor Report – 2025-12-28

## Sensor Test Run
- Command: `go test ./internal/collector/services -run TestSensorsSuite -v`
- Status: PASS for all required sensors; `Physical` and `Docker` remain optional and only log a warning when their environments do not expose temperatures or Docker daemon.
- Logs are emitted per sensor via the test helper so CI can capture the JSON snapshot for later inspection.

## Sensor Entries (log style)
- `[2025-12-28T00:00:00Z] CPU` – gathers total and per-core utilization via `cpu.PercentWithContext`, records the reported CPU model name and logical core count from `cpu.InfoWithContext` and `cpu.CountsWithContext`.
- `[2025-12-28T00:00:01Z] Memory` – snapshots the full `mem.VirtualMemoryWithContext` structure (available, used, cached, swap, huge pages, slab, etc.) plus the swap stats from `mem.SwapMemoryWithContext`.
- `[2025-12-28T00:00:02Z] Disk` – enumerates partitions (`disk.PartitionsWithContext`), collects usage metrics per mountpoint (`disk.UsageWithContext`), and captures per-device I/O counters (`disk.IOCountersWithContext`).
- `[2025-12-28T00:00:03Z] Network` – pulls per-interface byte/packet counters with `net.IOCountersWithContext(..., true)`.
- `[2025-12-28T00:00:04Z] Host` – surfaces host metadata including OS, platform, kernel version, virtualization info, boot time, uptime, and process count from `host.InfoWithContext`.
- `[2025-12-28T00:00:05Z] Process` – iterates over every process from `process.ProcessesWithContext` and records the expanded `ProcessInfo` payload (PID, parent, cmdline, binary path, working dir, terminal, status, CPU/memory usage, affinity, times, I/O counters, threads, open files, connections, environment, rlimits, memory maps, page faults, etc.).
- `[2025-12-28T00:00:06Z] Physical` – optionally samples temperature sensors via `sensors.TemperaturesWithContext`; missing sensors are logged but do not fail the suite.
- `[2025-12-28T00:00:07Z] Docker` – when the daemon responds (`docker.GetDockerStatWithContext`), captures container stats plus optional cgroup CPU/memory values; it reports `Available=false` otherwise.

Each entry mirrors the JSON logged by the test helper so the file can be used as an audit trail for what data each sensor is expected to produce.
