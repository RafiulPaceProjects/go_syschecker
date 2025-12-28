package output

import (
	"fmt"
	"strings"

	"syschecker/internal/collector"
	"syschecker/internal/engine"
)

// Section constants to avoid hardcoded strings
const (
	SectionCPU     = "cpu"
	SectionRAM     = "ram"
	SectionDisk    = "disk"
	SectionNetwork = "network"
)

// UI/view-model types (no printing here)
type Item struct {
	Key    string
	Label  string
	Value  float64
	Unit   string
	Status string
	Note   string
}

type Section struct {
	ID    string // cpu/ram/disk/network
	Title string
	Items []Item
}

type DashboardView struct {
	Sections    []Section
	TotalRAMGB  int
	TotalDiskGB int
}

// BuildDashboard converts checker + sensors data into UI-ready sections.
func BuildDashboard(results []engine.CheckResult, stats *collector.RawStats) DashboardView {
	// Initialize sections map
	sec := map[string]*Section{
		SectionCPU:     {ID: SectionCPU, Title: "CPU"},
		SectionRAM:     {ID: SectionRAM, Title: "RAM"},
		SectionDisk:    {ID: SectionDisk, Title: "Disk"},
		SectionNetwork: {ID: SectionNetwork, Title: "Network"},
	}

	// Map CheckResults to Sections using the Category field
	for _, r := range results {
		unit := "%"
		if r.Category == engine.CategoryNetwork && strings.Contains(strings.ToLower(r.Name), "latency") {
			unit = "ms"
		}

		it := Item{
			Key:    strings.ReplaceAll(strings.ToLower(r.Name), " ", "_"),
			Label:  r.Name,
			Value:  r.Value,
			Unit:   unit,
			Status: r.Status,
		}

		switch r.Category {
		case engine.CategoryCPU:
			sec[SectionCPU].Items = append(sec[SectionCPU].Items, it)
		case engine.CategoryRAM:
			sec[SectionRAM].Items = append(sec[SectionRAM].Items, it)
		case engine.CategoryDisk:
			sec[SectionDisk].Items = append(sec[SectionDisk].Items, it)
		case engine.CategoryNetwork:
			sec[SectionNetwork].Items = append(sec[SectionNetwork].Items, it)
		}
	}

	// ------------------------------------------------------------------------
	// Inject Informational Metrics (Missing from Health Checks)
	// ------------------------------------------------------------------------

	// CPU Details
	sec[SectionCPU].Items = append(sec[SectionCPU].Items,
		Item{Label: "CPU Model", Note: stats.CPUModel},
		Item{Label: "Cores", Value: float64(stats.CPUCores)},
		Item{Label: "Load Avg (1m)", Value: stats.LoadAvg1},
		Item{Label: "Load Avg (5m)", Value: stats.LoadAvg5},
		Item{Label: "Load Avg (15m)", Value: stats.LoadAvg15},
	)

	for i, usage := range stats.CPUPerCore {
		sec[SectionCPU].Items = append(sec[SectionCPU].Items, Item{
			Label: fmt.Sprintf("Core %d", i),
			Value: usage,
			Unit:  "%",
		})
	}

	// RAM Details
	sec[SectionRAM].Items = append(sec[SectionRAM].Items,
		Item{Label: "Available", Value: float64(stats.RAMAvailable), Unit: "GB"},
		Item{Label: "Used", Value: float64(stats.RAMUsed), Unit: "GB"},
		Item{Label: "Free", Value: float64(stats.RAMFree), Unit: "GB"},
		Item{Label: "Cached", Value: float64(stats.RAMCached), Unit: "GB"},
		Item{Label: "Buffered", Value: float64(stats.RAMBuffered), Unit: "GB"},
		Item{Label: "Swap Usage", Value: stats.SwapUsage, Unit: "%"},
		Item{Label: "Swap Total", Value: float64(stats.SwapTotal), Unit: "GB"},
		Item{Label: "Swap Used", Value: float64(stats.SwapUsed), Unit: "GB"},
	)

	// Disk I/O & Health
	for _, io := range stats.IOCounters {
		sec[SectionDisk].Items = append(sec[SectionDisk].Items,
			Item{Label: io.Device + " Read", Value: float64(io.ReadBytes) / 1024 / 1024, Unit: "MB"},
			Item{Label: io.Device + " Write", Value: float64(io.WriteBytes) / 1024 / 1024, Unit: "MB"},
		)
	}
	for _, h := range stats.DiskHealth {
		if h.Message != "" {
			sec[SectionDisk].Items = append(sec[SectionDisk].Items, Item{
				Label: "Health: " + h.Device,
				Note:  h.Message,
			})
		}
	}

	// Network Traffic Details
	for _, nic := range stats.NetInterfaces {
		sec[SectionNetwork].Items = append(sec[SectionNetwork].Items,
			Item{Label: nic.Name + " Rx", Value: float64(nic.BytesRecv) / 1024 / 1024, Unit: "MB"},
			Item{Label: nic.Name + " Tx", Value: float64(nic.BytesSent) / 1024 / 1024, Unit: "MB"},
		)
	}

	return DashboardView{
		Sections: []Section{
			*sec[SectionCPU],
			*sec[SectionRAM],
			*sec[SectionDisk],
			*sec[SectionNetwork],
		},
		TotalRAMGB:  int(stats.TotalRAM_GB),
		TotalDiskGB: int(stats.TotalDisk_GB),
	}
}

func (v DashboardView) SectionByID(id string) *Section {
	for i := range v.Sections {
		if v.Sections[i].ID == id {
			return &v.Sections[i]
		}
	}
	return nil
}

func (s Section) ItemByKey(key string) *Item {
	for i := range s.Items {
		if s.Items[i].Key == key {
			return &s.Items[i]
		}
	}
	return nil
}