package console

import (
	"bytes"
	"syschecker/internal/output"
	"testing"
)

func TestColorFor(t *testing.T) {
	tests := []struct {
		status   string
		expected string
	}{
		{"WARN", colorYellow},
		{"CRIT", colorRed},
		{"OK", colorGreen},
		{"", colorGreen},
		{"UNKNOWN", colorGreen},
	}

	for _, tt := range tests {
		result := colorFor(tt.status)
		if result != tt.expected {
			t.Errorf("colorFor(%q) = %q; want %q", tt.status, result, tt.expected)
		}
	}
}

func TestPrint(t *testing.T) {
	// Simple smoke test to ensure Print doesn't crash with various data
	view := output.DashboardView{
		Sections: []output.Section{
			{
				Title: "Test Section",
				Items: []output.Item{
					{Label: "Healthy", Value: 10, Unit: "%", Status: "OK"},
					{Label: "Warning", Value: 80, Unit: "%", Status: "WARN"},
					{Label: "Critical", Value: 95, Unit: "%", Status: "CRIT"},
					{Label: "No Status", Value: 50, Unit: "GB"},
					{Label: "With Note", Note: "Info only"},
				},
			},
		},
		TotalRAMGB: 16,
	}

	var buf bytes.Buffer
	// We can't easily check terminal output, but we can ensure it doesn't panic
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("Print panicked: %v", r)
		}
	}()
	Print(&buf, view)
}
