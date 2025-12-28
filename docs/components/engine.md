# Engine Component

The Engine processes the data gathered by the Collector.

## Responsibilities

- **Threshold Checking**: Comparing metrics against predefined limits (e.g., high CPU usage, low disk space).
- **Health Determination**: Assigning status levels (OK, WARN, CRIT) to various system components.

## Implementation

Located in `internal/engine/`.
- `checker.go`: Core logic for evaluating system health.
- `config.go`: Defines the `Config` and `Thresholds` structures for customizable health checks.

## Configuration

The engine uses a `Config` struct to determine health status. You can use `engine.DefaultConfig()` for standard thresholds or customize them as needed.
