package state

import (
	"time"

	"syschecker/internal/collector"
	"syschecker/internal/engine"
)

type Page int

const (
	PageMenu Page = iota
	PageDashboard
	PageConsole // "Use Console"
	PageCPU     // "Detailed CPU Check"
	PageDisk    // "Detailed Disk Check"
	PageNetwork // "Detailed Network Check"
	PageRAM     // "Detailed RAM Check"
)

// AppState holds the current snapshot of the system
type AppState struct {
	Stats       *collector.RawStats
	Results     []engine.CheckResult
	LastUpdate  time.Time
	Err         error
	CPUHistory  []float64
	ConsoleLogs []string
	CurrentPage Page
}
