# Refactoring Issues & Analysis

## Overview
The codebase is currently in the middle of a refactoring process to modularize the TUI (Text User Interface). The goal was to move state and logic from a monolithic `MainModel` into individual `View` components. However, this has resulted in several compilation errors and structural inconsistencies.

## Identified Issues

### 1. Duplicate Declarations
- **File:** `ui/tui/views/menu.go`
- **Issue:** `NavigateMsg` is declared in `menu.go`, but it should likely be in `types.go` to be shared across views and the main app. If it's also in `types.go`, it causes a collision.

### 2. Missing Fields in `ViewProps`
- **File:** `ui/tui/views/renderers.go`
- **Issue:** The `ViewProps` struct in `types.go` was modified to remove fields like `MenuCursor`, `AnimCursor`, `ScrollY`, etc., as these were moved to specific views. However, `renderers.go` (and possibly other files) still tries to initialize `ViewProps` with these fields.

### 3. Undeclared Variables in Views
- **File:** `ui/tui/views/console.go`
- **Issue:** The code references `scrollY` (likely a previous local variable or prop), but it is undefined. The `ConsoleView` struct now has a `ScrollY` field, so the code should reference `v.ScrollY`.

### 4. Broken Tests
- **File:** `ui/tui/menu_test.go`
- **Issue:** The tests rely on fields that no longer exist on `MainModel` (e.g., `menuCursor`, `animCursor`).
- **Root Cause:** State has been moved to `MenuView`, so tests need to access `model.views[state.PageMenu]` to verify state changes.

### 5. MainModel Structure & Logic
- **File:** `ui/tui/app.go`
- **Issue:** The `MainModel` struct definition and `Update` method have been partially updated but contain duplicated code blocks (as seen in previous file reads) and need to correctly delegate to the `views` map.
- **Issue:** `slowTickCmd` and `animateCmd` appear to be duplicated in the file.

### 6. Entry Point
- **File:** `main.go`
- **Issue:** `tui.Start` is reported as undeclared or having an issue, likely due to the broken `app.go` file preventing the package from compiling correctly.

## Architecture & Data Flow Problems

- **State Fragmentation:** While moving state to views is good, the `MainModel` still holds `AppState` (global data like stats), while Views hold UI state (cursors, scroll position). We need to ensure `Render` methods receive the global `AppState` correctly.
- **Message Passing:** The `NavigateMsg` needs to be bubbled up from Views to the `MainModel` so it can switch the `CurrentPage`.
- **Props vs. State:** `ViewProps` should only contain ephemeral data passed from parent to child (like window dimensions), not persistent state.
