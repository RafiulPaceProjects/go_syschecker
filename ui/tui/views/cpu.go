package views

import (
	"fmt"
	"strings"
	"syschecker/ui/tui/state"
	"syschecker/ui/tui/styles"

	"github.com/charmbracelet/lipgloss"
)

type CPUView struct{}

func (v CPUView) Render(s state.AppState, props ViewProps) string {
	header := MenuHeaderStyle.Width(props.Width).Render("CPU Telemetry & Analysis")

	// Info section
	info := lipgloss.NewStyle().
		Padding(1, 2).
		Render(fmt.Sprintf("Model: %s\nCores: %d\nLoad: %.2f, %.2f, %.2f",
			s.Stats.CPUModel, s.Stats.CPUCores,
			s.Stats.LoadAvg1, s.Stats.LoadAvg5, s.Stats.LoadAvg15))

	// Chart section
	chart := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(styles.Highlight).
		Padding(1, 2).
		Render(lipgloss.JoinVertical(lipgloss.Left,
			lipgloss.NewStyle().Bold(true).Render("Usage History"),
			props.ChartView,
		))

	// Per-core section
	var cores []string
	for i, usage := range s.Stats.CPUPerCore {
		barWidth := 20
		filled := int(float64(barWidth) * usage / 100)
		if filled > barWidth {
			filled = barWidth
		}
		bar := strings.Repeat("█", filled) + strings.Repeat("░", barWidth-filled)

		color := lipgloss.Color("46") // Green
		if usage > 90 {
			color = lipgloss.Color("196") // Red
		} else if usage > 70 {
			color = lipgloss.Color("220") // Gold
		}

		coreStr := fmt.Sprintf("Core %2d: [%s] %5.1f%%", i, lipgloss.NewStyle().Foreground(color).Render(bar), usage)
		cores = append(cores, coreStr)
	}

	// Split cores into columns if there are many
	const coresPerCol = 8
	var cols []string
	for i := 0; i < len(cores); i += coresPerCol {
		end := i + coresPerCol
		if end > len(cores) {
			end = len(cores)
		}
		cols = append(cols, lipgloss.JoinVertical(lipgloss.Left, cores[i:end]...))
	}

	coreList := lipgloss.JoinHorizontal(lipgloss.Top, cols...)

	// Add padding between columns
	if len(cols) > 1 {
		styledCols := []string{cols[0]}
		for i := 1; i < len(cols); i++ {
			styledCols = append(styledCols, lipgloss.NewStyle().PaddingLeft(4).Render(cols[i]))
		}
		coreList = lipgloss.JoinHorizontal(lipgloss.Top, styledCols...)
	}

	coreBox := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(styles.Highlight).
		Padding(1, 2).
		Render(lipgloss.JoinVertical(lipgloss.Left,
			lipgloss.NewStyle().Bold(true).Render("Per-Core Utilization"),
			coreList,
		))

	content := lipgloss.JoinHorizontal(lipgloss.Top, chart, coreBox)

	return lipgloss.JoinVertical(lipgloss.Left,
		header,
		info,
		content,
		lipgloss.NewStyle().Padding(1, 2).Foreground(styles.Subtle).Render("Press 'b' to go back"),
	)
}
