# Syschecker

**Syschecker** is a robust, terminal-based system monitoring tool written in Go. It provides real-time insights into your system's health, performance metrics, and hardware status through a beautiful and responsive Terminal User Interface (TUI).

![License](https://img.shields.io/badge/license-MIT-blue.svg)
![Go Version](https://img.shields.io/badge/go-1.25.5-cyan)
![Docker](https://img.shields.io/badge/docker-ready-blue)

## Features

- **Real-time Monitoring**: Live updates for CPU, Memory, Disk, and Network usage.
- **Hardware Health**: Integration with `smartctl` for disk health monitoring.
- **Interactive TUI**: Navigate effortlessly with a menu-driven interface built with `bubbletea`.
- **Dockerized**: specific configuration for safe, isolated deployment while monitoring the host.
- **Visuals**: Smooth animations and modern styling using `lipgloss`.

## Installation

### Using Docker (Recommended)

Syschecker is designed to run in a container while monitoring the host system.

1.  **Build**:
    ```bash
    docker-compose build
    ```

2.  **Run**:
    ```bash
    docker-compose up -d
    ```

3.  **Logs/Attach**:
    ```bash
    docker-compose logs -f
    ```

### Local Development

1.  **Prerequisites**: Go 1.25+, `smartmontools` (optional, for disk checks).
2.  **Clone**:
    ```bash
    git clone https://github.com/RafiulPaceProjects/go_syschecker.git
    cd syschecker
    ```
3.  **Run**:
    ```bash
    go run .
    ```

## Usage

Once running, use the keyboard to navigate:

- **Arrow Keys / H, J, K, L**: Navigate menus and lists.
- **Enter**: Select an option.
- **Q / Ctrl+C**: Quit the application.

## Documentation

Detailed documentation is available in the `docs/` directory:
- [Architecture Overview](docs/architecture/overview.md)
- [UI Component](docs/components/ui.md)
- [Collector Component](docs/components/collector.md)
- [Engine Component](docs/components/engine.md)