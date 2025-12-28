package views

import (
	"syschecker/ui/tui/state"
)

// ViewProps contains UI-specific properties provided by the Controller.
type ViewProps struct {
	Width, Height  int
	MouseX, MouseY int

	// Component States
	MenuCursor  int
	AnimCursor  float64
	SpinnerView string
	ChartView   string
	ScrollY     int
}

// View defines the contract for any renderable page in the TUI.
type View interface {
	Render(s state.AppState, props ViewProps) string
}
