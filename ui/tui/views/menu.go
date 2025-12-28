package views

import (
	"fmt"
	"math"

	"syschecker/ui/tui/state"

	"github.com/charmbracelet/lipgloss"
	zone "github.com/lrstanley/bubblezone"
)

type MenuView struct{}

func (v MenuView) Render(s state.AppState, props ViewProps) string {
	// 1. Header
	header := MenuHeaderStyle.Width(props.Width).Render("SYSCHECKER // SYSTEM INTELLIGENCE")

	// 2. Menu Items
	options := []string{
		"Console Output View",
		"Full System Dashboard",
		"CPU Telemetry & Analysis",
		"Disk I/O & Partition Health",
		"Network Traffic & Latency",
		"Memory (RAM) Allocation",
	}

	var menuItems []string
	listStartY := 6

	for i, option := range options {
		// Animation Logic
		dist := math.Abs(float64(i) - props.AnimCursor)
		selectionStrength := 0.0
		if dist < 1.0 {
			selectionStrength = 1.0 - dist
		}

		// Mouse Gradient Logic
		itemCenterY := listStartY + (i * 3) + 1
		mouseDistY := math.Abs(float64(props.MouseY - itemCenterY))

		borderColor := BaseColor
		if mouseDistY < 10 {
			ratio := 1.0 - (mouseDistY / 10.0)
			if ratio > 0.5 {
				borderColor = lipgloss.Color("#aaa")
			}
		}

		if selectionStrength > 0.1 || i == props.MenuCursor {
			borderColor = BrandColor
		}

		// Style & Render
		popOut := int(selectionStrength * 2)

		boxStyle := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(borderColor).
			Padding(0, 1).
			MarginLeft(2 + popOut).
			Width(40)

		if i == props.MenuCursor {
			boxStyle = boxStyle.Bold(true).Foreground(lipgloss.Color("#FFF"))
		} else {
			boxStyle = boxStyle.Foreground(lipgloss.Color("#AAA"))
		}

		text := fmt.Sprintf("%02d. %s", i+1, option)
		renderedItem := boxStyle.Render(text)

		zoneID := fmt.Sprintf("menu_%d", i)
		menuItems = append(menuItems, zone.Mark(zoneID, renderedItem))
	}

	// 3. Construct Menu Box
	menuList := lipgloss.JoinVertical(lipgloss.Left, menuItems...)

	menuContent := lipgloss.JoinVertical(lipgloss.Left,
		lipgloss.NewStyle().Bold(true).PaddingLeft(2).Foreground(BrandColor).Render("DIAGNOSTIC MODULES"),
		CopyStyle.Render("Select a telemetry vector to begin analysis."),
		menuList,
	)

	menuBox := MenuBoxStyle.Render(menuContent)

	// 4. Footer
	authorText := lipgloss.NewStyle().Foreground(lipgloss.Color("#666")).Render("Architected by Rafiul Haider • Pace University")
	contactText := lipgloss.NewStyle().Foreground(lipgloss.Color("#444")).Render("Inquiries: rafiul.haider@pace.edu")
	controlsText := lipgloss.NewStyle().Foreground(lipgloss.Color("#333")).Render("\n[↑/↓] Navigate • [Enter] Select • [Q] Quit")

	footer := lipgloss.JoinVertical(lipgloss.Left,
		authorText,
		contactText,
		controlsText,
	)

	footerStyled := lipgloss.NewStyle().PaddingLeft(2).Render(footer)

	body := lipgloss.JoinVertical(lipgloss.Left,
		menuBox,
		footerStyled,
	)

	return zone.Scan(lipgloss.JoinVertical(lipgloss.Left, header, body))
}

// Keep styles global or move to theme.go if preferred, but keeping here for now.
var (
	BrandColor = lipgloss.Color("#f27b24")
	BaseColor  = lipgloss.Color("#444")

	MenuHeaderStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FFFFFF")).
			Background(BrandColor).
			Align(lipgloss.Left).
			Padding(1, 2)

	MenuBoxStyle = lipgloss.NewStyle().
			Padding(1, 0).
			MarginTop(1)

	CopyStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#888")).
			Italic(true).
			MarginBottom(1).
			PaddingLeft(2)
)
