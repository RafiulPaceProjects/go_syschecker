package views

import (
	"fmt"
	"strings"
	"syschecker/ui/tui/state"

	"github.com/charmbracelet/lipgloss"
)

type ConsoleView struct {
	Content string
}

func (v ConsoleView) Render(s state.AppState, props ViewProps) string {
	// Note: s (state) isn't used here because the content is pre-generated and passed in the struct or props.
	// But to match interface we accept it.
	// Actually, the interface says `Render(s state.AppState, props ViewProps)`.
	// We can put Content in Props or State.
	// In the App, we generated content into a buffer.
	// Let's assume the content is passed via the struct construction or we add "ConsoleContent" to ViewProps.
	// Adding "ConsoleContent" to ViewProps seems cleaner to keep views stateless.

	header := MenuHeaderStyle.Width(props.Width).Render("Live Console View")

	availableHeight := props.Height - lipgloss.Height(header) - 4
	if availableHeight < 1 {
		availableHeight = 1
	}

	lines := strings.Split(v.Content, "\n")
	totalLines := len(lines)

	scrollY := props.ScrollY
	if scrollY < 0 {
		scrollY = 0
	}
	if scrollY > totalLines-availableHeight {
		scrollY = totalLines - availableHeight
	}
	if scrollY < 0 {
		scrollY = 0
	}

	end := scrollY + availableHeight
	if end > totalLines {
		end = totalLines
	}

	visibleLines := lines[scrollY:end]
	viewContent := strings.Join(visibleLines, "\n")

	box := lipgloss.NewStyle().
		Width(props.Width-4).
		Height(availableHeight).
		Padding(0, 1).
		Render(viewContent)

	footerText := fmt.Sprintf("Scroll: %d/%d • Press 'b' to go back", scrollY, totalLines)
	if totalLines > availableHeight {
		footerText += " • Use ↑/↓ to scroll"
	}

	return lipgloss.JoinVertical(lipgloss.Left,
		header,
		lipgloss.NewStyle().Padding(1, 2).Render(box),
		lipgloss.NewStyle().PaddingLeft(2).Foreground(lipgloss.Color("#555")).Render(footerText),
	)
}
