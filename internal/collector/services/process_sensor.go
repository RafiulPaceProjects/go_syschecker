package services

import (
	"context"
	"fmt"

	"github.com/shirou/gopsutil/v4/process"
)

type ProcessInfo struct {
	PID    int32   `json:"pid"`
	Name   string  `json:"name,omitempty"`
	CPU    float64 `json:"cpu_percent,omitempty"`
	Memory float32 `json:"memory_percent,omitempty"`
}

type ProcessResult struct {
	Processes []ProcessInfo `json:"processes"`
}

type ProcessSensor struct{}

func NewProcessSensor() *ProcessSensor {
	return &ProcessSensor{}
}

func (s *ProcessSensor) Name() string {
	return "Process"
}

func (s *ProcessSensor) Connect(ctx context.Context) error {
	return nil
}

func (s *ProcessSensor) Disconnect(ctx context.Context) error {
	return nil
}

func (s *ProcessSensor) Collect(ctx context.Context) (any, error) {
	pids, err := process.PidsWithContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list pids: %w", err)
	}

	processes := make([]ProcessInfo, 0, len(pids))
	limit := 50 // safety limit to avoid long runs
	count := 0

	for _, pid := range pids {
		if count >= limit {
			break
		}
		p, err := process.NewProcessWithContext(ctx, pid)
		if err != nil {
			continue
		}
		name, _ := p.NameWithContext(ctx)
		cpuPct, _ := p.CPUPercentWithContext(ctx)
		memPct, _ := p.MemoryPercentWithContext(ctx)

		processes = append(processes, ProcessInfo{
			PID:    pid,
			Name:   name,
			CPU:    cpuPct,
			Memory: memPct,
		})
		count++
	}

	return ProcessResult{Processes: processes}, nil
}
