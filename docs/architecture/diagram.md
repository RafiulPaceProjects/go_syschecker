# Architecture & Data Flow Diagram

**Version Status**: `v1.0.0` (Current Architecture)
**Last Updated**: 2025-12-28

The following Mermaid chart illustrates the high-level architecture and the reactive data flow cycle within the `syschecker` application.

```mermaid
graph TD
    subgraph "Initialization"
        A[User] -->|Executes| B(main.go)
        B -->|Loads| C{Config}
        B -->|Inits| D[Collector]
        B -->|Starts| E[TUI Application]
    end

    subgraph "TUI Loop (Bubble Tea)"
        E -->|Tick 1s| F(Update Loop)
        F -->|Request Metrics| D
    end

    subgraph "Data Collection Layer"
        D -->|gopsutil/smartctl| G[(System / OS)]
        G -->|Raw Metrics| D
        D -->|RawStats| F
    end

    subgraph "Logic Layer"
        F -->|RawStats + Config| H[Engine]
        H -->|Evaluate()| I[CheckResults]
        I -->|Update State| F
    end

    subgraph "Presentation Layer"
        F -->|AppState| J[View Renderer]
        J -->|Render| K[Terminal Output]
        K -->|Visual Feedback| A
    end

    %% Data Flow Styling
    linkStyle 5,6,7 stroke:#f66,stroke-width:2px;
    linkStyle 8,9,10 stroke:#6f6,stroke-width:2px;
```

## Description

1.  **Initialization**: The application starts, loads configuration, initializes the `Collector`, and launches the `Bubble Tea` TUI runtime.
2.  **Event Loop**: A `TickMsg` triggers every second.
3.  **Collection**: The `Update` loop requests metrics from the `Collector`, which queries the underlying **System/OS** APIs.
4.  **Processing**: Received `RawStats` are passed to the `Engine`. The `Engine` evaluates them against the `Config` thresholds to generate `CheckResults` (OK/WARN/CRIT).
5.  **Rendering**: The application state (`AppState`) is updated with new stats and results. The `View` function then renders the appropriate screen (Dashboard, Console, etc.) to the **Terminal**.
