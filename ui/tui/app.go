package tui

import (
	"bytes"
	"fmt"
	"time"

	"syschecker/internal/collector"
	"syschecker/internal/engine"
	"syschecker/internal/output"
	"syschecker/ui/console"
	"syschecker/ui/tui/state"
	"syschecker/ui/tui/views"

	"github.com/NimbleMarkets/ntcharts/canvas"
	"github.com/NimbleMarkets/ntcharts/linechart"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/harmonica"
	"github.com/charmbracelet/lipgloss"
	zone "github.com/lrstanley/bubblezone"
)

// MainModel is the Bubble Tea Model acting as the Controller
type MainModel struct {
	provider       collector.StatsProvider
	state          state.AppState
	spinner        spinner.Model
	cpuChart       linechart.Model
	menuCursor     int
	animCursor     float64
	velocity       float64 // Physics velocity
	spring         harmonica.Spring
	consoleScrollY int
	mouseX         int
	mouseY         int
	quitting       bool
	width          int
	height         int
}

// Messages
type TickMsg time.Time
type AnimateMsg time.Time
type MetricsLoadedMsg struct {
	Stats collector.RawStats
	Err   error
}

func InitialModel(provider collector.StatsProvider) MainModel {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))

	lc := linechart.New(30, 10, 0, 30, 0, 100)
	history := make([]float64, 0, 31)

	// Initialize physics spring for smooth cursor animation
	// Increased frequency (12.0) for faster response and damping (0.9) to prevent overshoot
	spring := harmonica.NewSpring(harmonica.FPS(60), 12.0, 0.9)

	return MainModel{
		provider: provider,
		spinner:  s,
		cpuChart: lc,
		spring:   spring,
		state: state.AppState{
			CPUHistory:  history,
			CurrentPage: state.PageMenu,
		},
	}
}

func (m MainModel) Init() tea.Cmd {
	zone.NewGlobal()
	return tea.Batch(
		m.spinner.Tick,
		tickCmd(),
		animateCmd(),
	)
}

// Commands
func tickCmd() tea.Cmd {
	return tea.Tick(time.Second*1, func(t time.Time) tea.Msg {
		return TickMsg(t)
	})
}

func animateCmd() tea.Cmd {
	return tea.Tick(time.Millisecond*16, func(t time.Time) tea.Msg {
		return AnimateMsg(t)
	})
}

func fetchMetricsCmd(p collector.StatsProvider) tea.Cmd {
	return func() tea.Msg {
		stats, err := p.GetRawMetrics()
		return MetricsLoadedMsg{Stats: stats, Err: err}
	}
}

func (m MainModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			m.quitting = true
			return m, tea.Quit
		}

		if m.state.CurrentPage == state.PageMenu {
			switch msg.String() {
			case "up", "k":
				if m.menuCursor > 0 {
					m.menuCursor--
				}
			case "down", "j":
				if m.menuCursor < 5 {
					m.menuCursor++
				}
			case "enter":
				switch m.menuCursor {
				case 0:
					m.state.CurrentPage = state.PageConsole
				case 1:
					m.state.CurrentPage = state.PageDashboard
				case 2:
					m.state.CurrentPage = state.PageCPU
				case 3:
					m.state.CurrentPage = state.PageDisk
				case 4:
					m.state.CurrentPage = state.PageNetwork
				case 5:
					m.state.CurrentPage = state.PageRAM
				}
			}
			return m, nil
		}

		if m.state.CurrentPage == state.PageConsole {
			switch msg.String() {
			case "up", "k":
				if m.consoleScrollY > 0 {
					m.consoleScrollY--
				}
			case "down", "j":
				m.consoleScrollY++
			}
		}

		if msg.String() == "b" || msg.String() == "esc" || msg.String() == "backspace" {
			m.state.CurrentPage = state.PageMenu
			m.consoleScrollY = 0
			return m, nil
		}

	case AnimateMsg:
		var v float64 = m.velocity
		m.animCursor, v = m.spring.Update(m.animCursor, float64(m.menuCursor), v)
		m.velocity = v
		return m, animateCmd()

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		newW := msg.Width/2 - 6
		if newW > 10 {
			m.cpuChart.Resize(newW, 10)
		}

	case TickMsg:
		// Trigger async fetch, and schedule next tick
		return m, tea.Batch(
			fetchMetricsCmd(m.provider),
			tickCmd(),
		)

	case MetricsLoadedMsg:
		if msg.Err != nil {
			m.state.Err = msg.Err
			return m, nil
		}

		// Update State
		stats := msg.Stats
		m.state.Stats = stats
		m.state.Results = engine.Evaluate(stats)
		m.state.LastUpdate = time.Now()

		// Update History
		m.state.CPUHistory = append(m.state.CPUHistory, stats.CPUUsage)
		if len(m.state.CPUHistory) > 31 {
			m.state.CPUHistory = m.state.CPUHistory[1:]
		}

		// Update Chart
		m.cpuChart.Clear()
		for i := 0; i < len(m.state.CPUHistory)-1; i++ {
			y1 := m.state.CPUHistory[i]
			y2 := m.state.CPUHistory[i+1]
			m.cpuChart.DrawBrailleLine(
				canvas.Float64Point{X: float64(i), Y: y1},
				canvas.Float64Point{X: float64(i + 1), Y: y2},
			)
		}
		m.cpuChart.DrawXYAxisAndLabel()

		// Update Logs
		logLine := fmt.Sprintf("[%s] CPU: %.1f%% | RAM: %.1f%% | Disk: %.1f%%",
			time.Now().Format("15:04:05"),
			stats.CPUUsage,
			stats.RAMUsage,
			stats.DiskUsage,
		)
		m.state.ConsoleLogs = append(m.state.ConsoleLogs, logLine)
		if len(m.state.ConsoleLogs) > 100 {
			m.state.ConsoleLogs = m.state.ConsoleLogs[1:]
		}
		return m, nil

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case tea.MouseMsg:
		m.mouseX = msg.X
		m.mouseY = msg.Y

		if msg.Action == tea.MouseActionRelease && m.state.CurrentPage == state.PageMenu {
			for i := 0; i <= 5; i++ {
				if zone.Get(fmt.Sprintf("menu_%d", i)).InBounds(msg) {
					m.menuCursor = i
					switch m.menuCursor {
					case 0:
						m.state.CurrentPage = state.PageConsole
					case 1:
						m.state.CurrentPage = state.PageDashboard
					case 2:
						m.state.CurrentPage = state.PageCPU
					case 3:
						m.state.CurrentPage = state.PageDisk
					case 4:
						m.state.CurrentPage = state.PageNetwork
					case 5:
						m.state.CurrentPage = state.PageRAM
					}
					return m, nil
				}
			}
		}
	}

	return m, nil
}

func (m MainModel) View() string {
	if m.quitting {
		return "Bye!\n"
	}

	switch m.state.CurrentPage {
	case state.PageMenu:
		return views.RenderMenu(m.width, m.height, m.menuCursor, m.animCursor, m.mouseX, m.mouseY)
	case state.PageDashboard:
		return views.RenderDashboard(m.state, m.spinner.View(), m.cpuChart.View())
	case state.PageConsole:
		dashboard := output.BuildDashboard(m.state.Results, m.state.Stats)
		var buf bytes.Buffer
		console.Print(&buf, dashboard)
		return views.RenderRawConsole(buf.String(), m.width, m.height, m.consoleScrollY)
	case state.PageCPU:
		return views.RenderCPU(m.state, m.cpuChart.View(), m.width, m.height)
	default:
		return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center,
			lipgloss.NewStyle().Bold(true).Render("Detailed View Under Construction\n\nPress 'b' to go back"),
		)
	}
}

func Start(provider collector.StatsProvider) error {
	p := tea.NewProgram(
		InitialModel(provider),
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	)
	_, err := p.Run()
	return err
}
