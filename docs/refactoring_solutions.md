# Refactoring Solutions & Plan

## 1. Standardize Types (`ui/tui/views/types.go`)
- Define `NavigateMsg` here to be accessible by all views and the app.
- Clean up `ViewProps` to only include:
  - `Width`, `Height`
  - `MouseX`, `MouseY`
  - `SpinnerView` (if passed from global)
  - `ChartView` (if passed from global)

## 2. Fix Views
- **`ui/tui/views/menu.go`**: Remove `NavigateMsg` declaration. Ensure `Update` returns `NavigateMsg` wrapped in a command or directly (bubbletea commands return `Msg`).
- **`ui/tui/views/console.go`**: Fix `Render` to use `v.ScrollY`.
- **`ui/tui/views/renderers.go`**: Remove this file if it contains legacy render functions that are now methods on View structs. If it contains shared helpers, update them to use the new `ViewProps`.

## 3. Fix Main Application Logic (`ui/tui/app.go`)
- Clean up `MainModel` struct.
- Remove duplicated functions.
- Ensure `Update` handles `NavigateMsg` to switch pages.
- Ensure `Update` delegates messages to the `currentView`.
- Ensure `View` method calls `currentView.Render`.

## 4. Fix Tests (`ui/tui/menu_test.go`)
- Update test helpers to inspect the `MenuView` instance inside `MainModel.views`.
- Fix assertions to check `menuView.Cursor` instead of `model.menuCursor`.

## 5. Verify Entry Point (`main.go`)
- Once `app.go` is fixed, `tui.Start` should be available.

## Execution Order
1. `ui/tui/views/types.go`
2. `ui/tui/views/menu.go`
3. `ui/tui/views/console.go`
4. `ui/tui/views/renderers.go` (Cleanup)
5. `ui/tui/app.go`
6. `ui/tui/menu_test.go`
