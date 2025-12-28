package views

import (
	"syschecker/ui/tui/styles"

	"github.com/charmbracelet/lipgloss"
)

func RenderSection(title string, items map[string]string) string {
	// Simple generic renderer if needed, but for now we'll stick to the specific logic in Dashboard
	// This is a placeholder for future reusable components (like a List or Table component)
	return ""
}

func ColorForStatus(status string) lipgloss.Style {
	sStyle := styles.StatusStyle
	if status == "WARN" {
		return sStyle.Foreground(lipgloss.Color("220")) // Gold
	} else if status == "CRIT" {
		return sStyle.Foreground(lipgloss.Color("196")) // Red
	}
	return sStyle.Foreground(lipgloss.Color("46")) // Green
}
