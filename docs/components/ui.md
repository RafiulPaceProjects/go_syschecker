# UI Component

The UI is built using the **Bubble Tea** (The Elm Architecture in Go) and **Lip Gloss** (styling) frameworks.

## Key Features

- **TUI Dashboard**: Real-time visualization of system metrics.
- **Menu System**: Interactive navigation between different views (CPU, Dashboard, Console).
- **Animations**: Fluid transitions using `harmonica` spring physics.

## Views

- `Dashboard`: Overview of all critical metrics.
- `CPU`: Detailed CPU usage and per-core statistics.
- `Console`: Log output and detailed system messages.
- `Menu`: Sidebar for navigation.

## Styles

The look and feel is defined in `ui/tui/styles/theme.go`, using a modern color palette and flexible layouts.
