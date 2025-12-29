package services

import (
	"context"
	"fmt"

	"github.com/shirou/gopsutil/v4/cpu"
)

type CPUResult struct {
	TotalUsage float64
	PerCore    []float64
	Model      string
	Cores      int
}

type CPUSensor struct{}

func NewCPUSensor() *CPUSensor {
	return &CPUSensor{}
}

func (s *CPUSensor) Name() string {
	return "CPU"
}

func (s *CPUSensor) Connect(ctx context.Context) error {
	return nil
}

func (s *CPUSensor) Disconnect(ctx context.Context) error {
	return nil
}

func (s *CPUSensor) Collect(ctx context.Context) (any, error) {
	total, err := cpu.PercentWithContext(ctx, 0, false)
	if err != nil || len(total) == 0 {
		return nil, fmt.Errorf("failed to get total cpu percent: %w", err)
	}

	perCore, err := cpu.PercentWithContext(ctx, 0, true)
	if err != nil {
		return nil, fmt.Errorf("failed to get per-core cpu percent: %w", err)
	}

	info, err := cpu.InfoWithContext(ctx)
	model := "Unknown"
	if err == nil && len(info) > 0 {
		model = info[0].ModelName
	}

	cores, _ := cpu.CountsWithContext(ctx, true)

	return CPUResult{
		TotalUsage: total[0],
		PerCore:    perCore,
		Model:      model,
		Cores:      cores,
	}, nil
}
