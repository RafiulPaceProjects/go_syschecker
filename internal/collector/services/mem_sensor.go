package services

import (
	"context"
	"fmt"

	"github.com/shirou/gopsutil/v4/mem"
)

type MemResult struct {
	UsedPercent    float64
	Available      uint64
	Used           uint64
	Free           uint64
	Cached         uint64
	Buffers        uint64
	Total          uint64
	SwapUsage      float64
	SwapTotal      uint64
	SwapUsed       uint64
	Active         uint64
	Inactive       uint64
	Wired          uint64
	Laundry        uint64
	WriteBack      uint64
	Dirty          uint64
	WriteBackTmp   uint64
	Shared         uint64
	Slab           uint64
	Sreclaimable   uint64
	Sunreclaim     uint64
	PageTables     uint64
	SwapCached     uint64
	CommitLimit    uint64
	CommittedAS    uint64
	HighTotal      uint64
	HighFree       uint64
	LowTotal       uint64
	LowFree        uint64
	Mapped         uint64
	VmallocTotal   uint64
	VmallocUsed    uint64
	VmallocChunk   uint64
	HugePagesTotal uint64
	HugePagesFree  uint64
	HugePagesRsvd  uint64
	HugePagesSurp  uint64
	HugePageSize   uint64
	AnonHugePages  uint64
}

type MemSensor struct{}

func NewMemSensor() *MemSensor {
	return &MemSensor{}
}

func (s *MemSensor) Name() string {
	return "Memory"
}

func (s *MemSensor) Connect(ctx context.Context) error {
	return nil
}

func (s *MemSensor) Disconnect(ctx context.Context) error {
	return nil
}

func (s *MemSensor) Collect(ctx context.Context) (any, error) {
	v, err := mem.VirtualMemoryWithContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get virtual memory: %w", err)
	}

	swapStat, swapErr := mem.SwapMemoryWithContext(ctx)
	swapUsage := 0.0
	swapTotal := v.SwapTotal
	swapUsed := v.SwapTotal - v.SwapFree
	if swapErr == nil && swapStat != nil {
		swapUsage = swapStat.UsedPercent
		swapTotal = swapStat.Total
		swapUsed = swapStat.Used
	}

	return MemResult{
		UsedPercent:    v.UsedPercent,
		Available:      v.Available,
		Used:           v.Used,
		Free:           v.Free,
		Cached:         v.Cached,
		Buffers:        v.Buffers,
		Total:          v.Total,
		SwapUsage:      swapUsage,
		SwapTotal:      swapTotal,
		SwapUsed:       swapUsed,
		Active:         v.Active,
		Inactive:       v.Inactive,
		Wired:          v.Wired,
		Laundry:        v.Laundry,
		WriteBack:      v.WriteBack,
		Dirty:          v.Dirty,
		WriteBackTmp:   v.WriteBackTmp,
		Shared:         v.Shared,
		Slab:           v.Slab,
		Sreclaimable:   v.Sreclaimable,
		Sunreclaim:     v.Sunreclaim,
		PageTables:     v.PageTables,
		SwapCached:     v.SwapCached,
		CommitLimit:    v.CommitLimit,
		CommittedAS:    v.CommittedAS,
		HighTotal:      v.HighTotal,
		HighFree:       v.HighFree,
		LowTotal:       v.LowTotal,
		LowFree:        v.LowFree,
		Mapped:         v.Mapped,
		VmallocTotal:   v.VmallocTotal,
		VmallocUsed:    v.VmallocUsed,
		VmallocChunk:   v.VmallocChunk,
		HugePagesTotal: v.HugePagesTotal,
		HugePagesFree:  v.HugePagesFree,
		HugePagesRsvd:  v.HugePagesRsvd,
		HugePagesSurp:  v.HugePagesSurp,
		HugePageSize:   v.HugePageSize,
		AnonHugePages:  v.AnonHugePages,
	}, nil
}
