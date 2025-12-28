# Collector Component

The Collector is responsible for fetching raw data from the system.

## Sources

- **OS/Exec**: Used to call system utilities like `smartctl` for disk health.
- **gopsutil**: Used for standard system metrics like CPU, RAM, and Disk usage.
- **File System**: Direct reading from `/proc` and `/sys` for Linux-specific details.

## Implementation

Located in `internal/collector/`.
- `sensors.go`: Handles hardware sensor data and disk health.
