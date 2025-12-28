# Engine Component

The Engine processes the data gathered by the Collector.

## Responsibilities

- **Threshold Checking**: Comparing metrics against predefined limits (e.g., high CPU usage, low disk space).
- **Health Determination**: Assigning status levels (OK, WARN, CRIT) to various system components.

## Implementation

Located in `internal/engine/`.
- `checker.go`: Core logic for evaluating system health.
