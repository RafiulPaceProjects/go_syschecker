package main

import (
	"fmt"
	"os"

	"syschecker/internal/collector"
	"syschecker/internal/engine"
	"syschecker/ui/tui"
)

func main() {
	// Use the interface to allow for different collector implementations
	var provider collector.StatsProvider = collector.SystemCollector{}

	cfg := engine.DefaultConfig()

	// Start the TUI application directly
	if err := tui.Start(provider, cfg); err != nil {
		fmt.Printf("Error running TUI: %v\n", err)
		os.Exit(1)
	}
}
