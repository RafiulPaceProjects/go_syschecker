package views

import (
	"fmt"
	"syschecker/internal/output"
	"syschecker/ui/tui/state"
	"syschecker/ui/tui/styles"

	"github.com/charmbracelet/lipgloss"
	zone "github.com/lrstanley/bubblezone"
)

type DashboardView struct{}

func (v DashboardView) Render(s state.AppState, props ViewProps) string {
	if s.Err != nil {
		return fmt.Sprintf("Error: %v", s.Err)
	}

	header := lipgloss.JoinHorizontal(lipgloss.Left,
		props.SpinnerView,
		styles.TitleStyle.Render("SysChecker TUI"),
		fmt.Sprintf(" Last Update: %s", s.LastUpdate.Format("15:04:05")),
	)

	dashboard := output.BuildDashboard(s.Results, s.Stats)

	renderSection := func(sec *output.Section) string {
		content := ""
		for _, item := range sec.Items {
			valStr := fmt.Sprintf("%.1f%s", item.Value, item.Unit)
			if item.Status != "" {
				valStr = ColorForStatus(item.Status).Render(fmt.Sprintf("%s [%s]", valStr, item.Status))
			}
			content += fmt.Sprintf("% -15s : %s\n", item.Label, valStr)
		}
		return content
	}

	var cpuCol, ramCol, diskCol, netCol string

	if cpuSec := dashboard.SectionByID("cpu"); cpuSec != nil {
		cpuCol = zone.Mark("cpu_box", styles.CardStyle.Render(
			lipgloss.JoinVertical(lipgloss.Left,
				lipgloss.NewStyle().Bold(true).Render("CPU Metrics"),
				renderSection(cpuSec),
				props.ChartView,
			),
		))
	}

	if ramSec := dashboard.SectionByID("ram"); ramSec != nil {
		ramCol = styles.CardStyle.Render(
			lipgloss.JoinVertical(lipgloss.Left,
				lipgloss.NewStyle().Bold(true).Render("RAM Metrics"),
				renderSection(ramSec),
			),
		)
	}

	if diskSec := dashboard.SectionByID("disk"); diskSec != nil {
		diskCol = styles.CardStyle.Render(
			lipgloss.JoinVertical(lipgloss.Left,
				lipgloss.NewStyle().Bold(true).Render("Disk Metrics"),
				renderSection(diskSec),
			),
		)
	}

	if netSec := dashboard.SectionByID("network"); netSec != nil {
		netCol = styles.CardStyle.Render(
			lipgloss.JoinVertical(lipgloss.Left,
				lipgloss.NewStyle().Bold(true).Render("Network Metrics"),
				renderSection(netSec),
			),
		)
	}

	row1 := lipgloss.JoinHorizontal(lipgloss.Top, cpuCol, ramCol)
	row2 := lipgloss.JoinHorizontal(lipgloss.Top, diskCol, netCol)

	return zone.Scan(lipgloss.JoinVertical(lipgloss.Left,
		header,
		row1,
		row2,
		lipgloss.NewStyle().Foreground(styles.Subtle).Render("\nPress 'q' to quit"),
	))
}
