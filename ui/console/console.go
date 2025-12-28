package console

import (
	"fmt"
	"io"
	"strings"

	"syschecker/internal/output"
)

const (
	colorReset  = "\033[0m"
	colorRed    = "\033[31m"
	colorGreen  = "\033[32m"
	colorYellow = "\033[33m"
	colorCyan   = "\033[36m"
)

// Print renders the dashboard view to the writer in a highly compact format.
func Print(w io.Writer, view output.DashboardView) {
	fmt.Fprintf(w, "%s%s %s%s\n", colorCyan, "■", "SYSCHECKER REPORT", colorReset)

	for _, sec := range view.Sections {
		// Section Header
		fmt.Fprintf(w, "%s%s%s\n", colorCyan, "─ "+sec.Title, colorReset)

		for _, it := range sec.Items {
			color := colorFor(it.Status)

			// Compact Label (max 20 chars)
			label := it.Label
			if len(label) > 20 {
				label = label[:17] + "..."
			}

			// Value Formatting
			valStr := ""
			if it.Unit != "" {
				valStr = fmt.Sprintf("%.1f%s", it.Value, it.Unit)
			} else if it.Value != 0 {
				valStr = fmt.Sprintf("%.1f", it.Value)
			} else if it.Note != "" {
				valStr = it.Note
				// Truncate long notes
				if len(valStr) > 25 {
					valStr = valStr[:22] + "..."
				}
			}

			// Status Marker
			statusMarker := ""
			if it.Status != "" {
				// Just a colored dot or short text
				statusMarker = fmt.Sprintf(" %s%s%s", color, it.Status[:1], colorReset) // "O", "W", "C"
				if it.Status == "WARN" {
					statusMarker = fmt.Sprintf(" %s!%s", color, colorReset)
				}
				if it.Status == "CRIT" {
					statusMarker = fmt.Sprintf(" %sX%s", color, colorReset)
				}
				if it.Status == "OK" {
					statusMarker = fmt.Sprintf(" %s✓%s", color, colorReset)
				}
			}

			// Dots leader
			dots := strings.Repeat("·", 22-len(label))

			// Format: "  Label............... ValueStatus"
			fmt.Fprintf(w, "  %s%s%s %10s%s\n", label, colorCyan+dots+colorReset, "", valStr, statusMarker)
		}
	}

	// Single-line Summary
	diskStr := ""
	if view.TotalDiskGB > 0 {
		diskStr = fmt.Sprintf(" | Disk: %dGB", view.TotalDiskGB)
	}
	fmt.Fprintf(w, "%s─ Summary%s: RAM: %dGB%s\n\n", colorCyan, colorReset, view.TotalRAMGB, diskStr)
}

func colorFor(status string) string {
	switch status {
	case "WARN":
		return colorYellow
	case "CRIT":
		return colorRed
	default:
		return colorGreen
	}
}
