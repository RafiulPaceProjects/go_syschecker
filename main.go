package main

import (
	"fmt"
	"os"

	"syschecker/internal/collector"
	"syschecker/ui/tui"
)

func main() {
	// Use the interface to allow for different collector implementations
	var provider collector.StatsProvider = collector.SystemCollector{}

	// Start the TUI application directly
	if err := tui.Start(provider); err != nil {
		fmt.Printf("Error running TUI: %v\n", err)
		os.Exit(1)
	}
}
