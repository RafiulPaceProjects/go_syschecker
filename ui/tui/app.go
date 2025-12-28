package tui

import (
	"context"
	"fmt"
	"time"

	"syschecker/internal/collector"
	"syschecker/internal/engine"
	"syschecker/ui/tui/components"
	"syschecker/ui/tui/state"
	"syschecker/ui/tui/views"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/harmonica"
	"github.com/charmbracelet/lipgloss"
	zone "github.com/lrstanley/bubblezone"
)

// MainModel is the Bubble Tea Model acting as the Controller
type MainModel struct {
	provider       collector.StatsProvider
	config         engine.Config
	state          state.AppState
	spinner        spinner.Model
	cpuWidget      *components.CPUWidget
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
type FastTickMsg time.Time
type SlowTickMsg time.Time
type AnimateMsg time.Time
type FastMetricsLoadedMsg struct {
	Stats *collector.RawStats
	Err   error
}
type SlowMetricsLoadedMsg struct {
	Stats *collector.RawStats
	Err   error
}

func InitialModel(provider collector.StatsProvider, cfg engine.Config) MainModel {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))

	cpuWidget := components.NewCPUWidget(30, 10)

	// Initialize physics spring for smooth cursor animation
	// Increased frequency (12.0) for faster response and damping (0.9) to prevent overshoot
	spring := harmonica.NewSpring(harmonica.FPS(60), 12.0, 0.9)

	// Initial empty stats
	emptyStats := &collector.RawStats{}

	return MainModel{
		provider:  provider,
		config:    cfg,
		spinner:   s,
		cpuWidget: cpuWidget,
		spring:    spring,
		state: state.AppState{
			Stats:       emptyStats,
			CPUHistory:  make([]float64, 0, 31),
			CurrentPage: state.PageMenu,
		},
	}
}

func (m *MainModel) Init() tea.Cmd {
	zone.NewGlobal()
	return tea.Batch(
		m.spinner.Tick,
		fastTickCmd(),
		slowTickCmd(),
		animateCmd(),
	)
}

// Commands
func fastTickCmd() tea.Cmd {
	return tea.Tick(time.Second*1, func(t time.Time) tea.Msg {
		return FastTickMsg(t)
	})
}

func slowTickCmd() tea.Cmd {
	// 30 seconds for slow metrics (network, disk health)
	return tea.Tick(time.Second*30, func(t time.Time) tea.Msg {
		return SlowTickMsg(t)
	})
}

func animateCmd() tea.Cmd {
	return tea.Tick(time.Millisecond*16, func(t time.Time) tea.Msg {
		return AnimateMsg(t)
	})
}

func fetchFastMetricsCmd(p collector.StatsProvider) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		stats, err := p.GetFastMetrics(ctx)
		return FastMetricsLoadedMsg{Stats: stats, Err: err}
	}
}

func fetchSlowMetricsCmd(p collector.StatsProvider) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
		defer cancel()
		stats, err := p.GetSlowMetrics(ctx)
		return SlowMetricsLoadedMsg{Stats: stats, Err: err}
	}
}

func (m *MainModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		return m.handleKeyMsg(msg)

	case AnimateMsg:
		return m.handleAnimateMsg(msg)

	case tea.WindowSizeMsg:
		return m.handleWindowSizeMsg(msg)

	case FastTickMsg:
		return m.handleFastTickMsg(msg)

	case SlowTickMsg:
		return m.handleSlowTickMsg(msg)

	case FastMetricsLoadedMsg:
		return m.handleFastMetricsLoadedMsg(msg)

	case SlowMetricsLoadedMsg:
		return m.handleSlowMetricsLoadedMsg(msg)

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case tea.MouseMsg:
		return m.handleMouseMsg(msg)
	}

	return m, nil
}

func (m *MainModel) handleKeyMsg(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
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
			m.navigateTo(m.menuCursor)
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

	return m, nil
}

func (m *MainModel) navigateTo(cursor int) {
	switch cursor {
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

func (m *MainModel) handleAnimateMsg(msg AnimateMsg) (tea.Model, tea.Cmd) {
	var v float64 = m.velocity
	m.animCursor, v = m.spring.Update(m.animCursor, float64(m.menuCursor), v)
	m.velocity = v
	return m, animateCmd()
}

func (m *MainModel) handleWindowSizeMsg(msg tea.WindowSizeMsg) (tea.Model, tea.Cmd) {
	m.width = msg.Width
	m.height = msg.Height
	newW := msg.Width/2 - 6
	if newW > 10 {
		m.cpuWidget.Resize(newW, 10)
	}
	return m, nil
}

func (m *MainModel) handleFastTickMsg(msg FastTickMsg) (tea.Model, tea.Cmd) {
	return m, tea.Batch(
		fetchFastMetricsCmd(m.provider),
		fastTickCmd(),
	)
}

func (m *MainModel) handleSlowTickMsg(msg SlowTickMsg) (tea.Model, tea.Cmd) {
	return m, tea.Batch(
		fetchSlowMetricsCmd(m.provider),
		slowTickCmd(),
	)
}

func (m *MainModel) handleFastMetricsLoadedMsg(msg FastMetricsLoadedMsg) (tea.Model, tea.Cmd) {
	if msg.Err != nil {
		m.state.Err = msg.Err
		return m, nil
	}

	// Update Fast Stats
	newStats := msg.Stats

	// Preserve slow stats
	newStats.NetLatency_ms = m.state.Stats.NetLatency_ms
	newStats.IsConnected = m.state.Stats.IsConnected
	newStats.ActiveTCP = m.state.Stats.ActiveTCP
	newStats.DiskHealth = m.state.Stats.DiskHealth

	m.state.Stats = newStats
	m.state.Results = engine.Evaluate(newStats, m.config)
	m.state.LastUpdate = time.Now()

	// Update History (handled by widget now? Or we push to it)
	m.state.CPUHistory = append(m.state.CPUHistory, newStats.CPUUsage)
	if len(m.state.CPUHistory) > 31 {
		m.state.CPUHistory = m.state.CPUHistory[1:]
	}

	// Update Widget
	m.cpuWidget.Push(newStats.CPUUsage)

	// Update Logs
	logLine := fmt.Sprintf("[%s] CPU: %.1f%% | RAM: %.1f%% | Disk: %.1f%%",
		time.Now().Format("15:04:05"),
		newStats.CPUUsage,
		newStats.RAMUsage,
		newStats.DiskUsage,
	)
	m.state.ConsoleLogs = append(m.state.ConsoleLogs, logLine)
	if len(m.state.ConsoleLogs) > 100 {
		m.state.ConsoleLogs = m.state.ConsoleLogs[1:]
	}
	return m, nil
}

func (m *MainModel) handleSlowMetricsLoadedMsg(msg SlowMetricsLoadedMsg) (tea.Model, tea.Cmd) {
	if msg.Err != nil {
		// Log error but don't fail hard, slow metrics might fail occasionally
		m.state.ConsoleLogs = append(m.state.ConsoleLogs, fmt.Sprintf("Error fetching slow metrics: %v", msg.Err))
		return m, nil
	}

	// Merge slow stats
	m.state.Stats.NetLatency_ms = msg.Stats.NetLatency_ms
	m.state.Stats.IsConnected = msg.Stats.IsConnected
	m.state.Stats.ActiveTCP = msg.Stats.ActiveTCP
	m.state.Stats.DiskHealth = msg.Stats.DiskHealth

	// Re-evaluate (some checks depend on slow metrics)
	m.state.Results = engine.Evaluate(m.state.Stats, m.config)

	return m, nil
}

func (m *MainModel) handleMouseMsg(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	m.mouseX = msg.X
	m.mouseY = msg.Y

	if msg.Action == tea.MouseActionRelease && m.state.CurrentPage == state.PageMenu {
		for i := 0; i <= 5; i++ {
			if zone.Get(fmt.Sprintf("menu_%d", i)).InBounds(msg) {
				m.menuCursor = i
				m.navigateTo(i)
				return m, nil
			}
		}
	}
	return m, nil
}


func (m *MainModel) View() string {
	if m.quitting {
		return "Bye!\n"
	}

	switch m.state.CurrentPage {
	case state.PageMenu:
		return views.RenderMenu(m.width, m.height, m.menuCursor, m.animCursor, m.mouseX, m.mouseY)
	case state.PageDashboard:
		return views.RenderDashboard(m.state, m.spinner.View(), m.cpuWidget.View())
	case state.PageConsole:
		return views.RenderRawConsole(m.state, m.width, m.height, m.consoleScrollY)
	case state.PageCPU:
		return views.RenderCPU(m.state, m.cpuWidget.View(), m.width, m.height)
	default:
		return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center,
			lipgloss.NewStyle().Bold(true).Render("Detailed View Under Construction\n\nPress 'b' to go back"),
		)
	}
}

func Start(provider collector.StatsProvider, cfg engine.Config) error {
	m := InitialModel(provider, cfg)
	p := tea.NewProgram(
		&m,
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	)
	_, err := p.Run()
	return err
}
