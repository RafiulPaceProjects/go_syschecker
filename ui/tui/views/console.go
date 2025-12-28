package views

import (
	"fmt"
	"strings"
	"syschecker/internal/output"
	"syschecker/ui/tui/state"
	"syschecker/ui/tui/styles"

	"github.com/charmbracelet/lipgloss"
)

type ConsoleView struct{}

func (v ConsoleView) Render(s state.AppState, props ViewProps) string {
	header := MenuHeaderStyle.Width(props.Width).Render("Live Console View")

	dashboard := output.BuildDashboard(s.Results, s.Stats)
	
	var lines []string
	lines = append(lines, styles.TitleStyle.Foreground(lipgloss.Color("36")).Render("■ SYSCHECKER REPORT"))

	for _, sec := range dashboard.Sections {
		lines = append(lines, lipgloss.NewStyle().Foreground(lipgloss.Color("36")).Render("─ "+sec.Title))
		for _, it := range sec.Items {
			color := ColorForStatus(it.Status).GetForeground()
			
			label := it.Label
			if len(label) > 20 {
				label = label[:17] + "..."
			}

			valStr := ""
			if it.Unit != "" {
				valStr = fmt.Sprintf("%.1f%s", it.Value, it.Unit)
			} else if it.Value != 0 {
				valStr = fmt.Sprintf("%.1f", it.Value)
			} else if it.Note != "" {
				valStr = it.Note
				if len(valStr) > 25 {
					valStr = valStr[:22] + "..."
				}
			}

			statusMarker := ""
			statusStyle := lipgloss.NewStyle().Foreground(color)
			if it.Status != "" {
				switch it.Status {
				case "WARN":
					statusMarker = statusStyle.Render("!")
				case "CRIT":
					statusMarker = statusStyle.Render("X")
				case "OK":
					statusMarker = statusStyle.Render("✓")
				default:
					statusMarker = statusStyle.Render(it.Status[:1])
				}
			}

			dotsCount := 22 - len(label)
			if dotsCount < 0 {
				dotsCount = 0
			}
			dots := lipgloss.NewStyle().Foreground(lipgloss.Color("36")).Render(strings.Repeat("·", dotsCount))
			
			line := fmt.Sprintf("  %s%s %10s %s", label, dots, valStr, statusMarker)
			lines = append(lines, line)
		}
	}

	summary := lipgloss.NewStyle().Foreground(lipgloss.Color("36")).Render(fmt.Sprintf("─ Summary: RAM: %dGB | Disk: %dGB", dashboard.TotalRAMGB, dashboard.TotalDiskGB))
	lines = append(lines, summary)

	availableHeight := props.Height - lipgloss.Height(header) - 4
	if availableHeight < 1 {
		availableHeight = 1
	}

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
