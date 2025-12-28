package components

import (
	"syschecker/ui/tui/styles"

	"github.com/NimbleMarkets/ntcharts/canvas"
	"github.com/NimbleMarkets/ntcharts/linechart"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type CPUWidget struct {
	Chart   linechart.Model
	History []float64
	Width   int
	Height  int
}

func NewCPUWidget(width, height int) *CPUWidget {
	// width, height, minX, maxX, minY, maxY
	lc := linechart.New(width, height, 0, 30, 0, 100)
	return &CPUWidget{
		Chart:   lc,
		History: make([]float64, 0, 31),
		Width:   width,
		Height:  height,
	}
}

func (c *CPUWidget) Init() tea.Cmd {
	return nil
}

func (c *CPUWidget) Push(value float64) {
	c.History = append(c.History, value)
	if len(c.History) > 31 {
		c.History = c.History[1:]
	}
}

func (c *CPUWidget) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// The chart model doesn't implement tea.Model fully in a way we want to just delegate usually,
	// but here we are just updating the data.
	// Actually ntcharts models usually don't have Update logic that does much unless it's interactive.
	return c, nil
}

func (c *CPUWidget) Resize(w, h int) {
	c.Width = w
	c.Height = h
	c.Chart.Resize(w, h)
}

func (c *CPUWidget) View() string {
	c.Chart.Clear()
	for i := 0; i < len(c.History)-1; i++ {
		y1 := c.History[i]
		y2 := c.History[i+1]
		c.Chart.DrawBrailleLine(
			canvas.Float64Point{X: float64(i), Y: y1},
			canvas.Float64Point{X: float64(i + 1), Y: y2},
		)
	}
	c.Chart.DrawXYAxisAndLabel()

	return styles.CardStyle.Render(
		lipgloss.JoinVertical(lipgloss.Left,
			lipgloss.NewStyle().Bold(true).Render("CPU History"),
			c.Chart.View(),
		),
	)
}
