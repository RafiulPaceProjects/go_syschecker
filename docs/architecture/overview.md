# Architecture Overview

`syschecker` is designed as a modular system monitoring tool. It follows a simple data flow:

1.  **Collector**: Interfaces with the host system (via `/proc`, `/sys`, `/dev`, and external tools like `smartctl`) to gather raw system metrics.
2.  **Engine**: Processes raw metrics to identify issues or health status.
3.  **UI**: Provides a real-time TUI dashboard for the user to visualize the system state.

## Deployment

The application is containerized using Docker, allowing it to run in an isolated environment while monitoring the host system through volume mounts and privileged access.
