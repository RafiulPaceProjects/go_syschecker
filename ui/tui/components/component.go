package components

import (
	tea "github.com/charmbracelet/bubbletea"
)

// Component is the interface that all UI components must implement.
// It is similar to tea.Model but tailored for widgets.
type Component interface {
	Init() tea.Cmd
	Update(msg tea.Msg) (tea.Model, tea.Cmd)
	View() string
}
