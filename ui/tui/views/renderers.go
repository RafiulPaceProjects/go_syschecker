package views

import (
	"syschecker/ui/tui/state"
)

func RenderMenu(width, height, cursor int, animCursor float64, mouseX, mouseY int) string {
	v := MenuView{}
	return v.Render(state.AppState{}, ViewProps{
		Width:      width,
		Height:     height,
		MenuCursor: cursor,
		AnimCursor: animCursor,
		MouseX:     mouseX,
		MouseY:     mouseY,
	})
}

func RenderDashboard(s state.AppState, spinnerView, chartView string) string {
	v := DashboardView{}
	return v.Render(s, ViewProps{
		SpinnerView: spinnerView,
		ChartView:   chartView,
	})
}

func RenderRawConsole(s state.AppState, width, height, scrollY int) string {
	v := ConsoleView{}
	return v.Render(s, ViewProps{
		Width:   width,
		Height:  height,
		ScrollY: scrollY,
	})
}

func RenderCPU(s state.AppState, chartView string, width, height int) string {
	v := CPUView{}
	return v.Render(s, ViewProps{
		Width:     width,
		Height:    height,
		ChartView: chartView,
	})
}
