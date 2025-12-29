package services

import (
	"context"
	"fmt"

	"github.com/shirou/gopsutil/v4/disk"
)

type PartitionStat struct {
	Device     string
	Mountpoint string
	Fstype     string
	Opts       []string
}

type UsageStat struct {
	Path              string
	Fstype            string
	Total             uint64
	Free              uint64
	Used              uint64
	UsedPercent       float64
	InodesTotal       uint64
	InodesUsed        uint64
	InodesFree        uint64
	InodesUsedPercent float64
}

type IOCountersStat struct {
	ReadCount        uint64
	MergedReadCount  uint64
	WriteCount       uint64
	MergedWriteCount uint64
	ReadBytes        uint64
	WriteBytes       uint64
	ReadTime         uint64
	WriteTime        uint64
	IopsInProgress   uint64
	IoTime           uint64
	WeightedIO       uint64
	Name             string
	SerialNumber     string
	Label            string
}

type DiskResult struct {
	Partitions []PartitionStat
	Usage      []UsageStat
	IOCounters map[string]IOCountersStat
}

type DiskSensor struct{}

func NewDiskSensor() *DiskSensor {
	return &DiskSensor{}
}

func (s *DiskSensor) Name() string {
	return "Disk"
}

func (s *DiskSensor) Connect(ctx context.Context) error {
	return nil
}

func (s *DiskSensor) Disconnect(ctx context.Context) error {
	return nil
}

func (s *DiskSensor) Collect(ctx context.Context) (any, error) {
	partitions, err := disk.PartitionsWithContext(ctx, true)
	if err != nil {
		return nil, fmt.Errorf("failed to get partitions: %w", err)
	}

	var partStats []PartitionStat
	var usageStats []UsageStat

	for _, p := range partitions {
		partStats = append(partStats, PartitionStat{
			Device:     p.Device,
			Mountpoint: p.Mountpoint,
			Fstype:     p.Fstype,
			Opts:       p.Opts,
		})

		// Collect usage for each partition
		u, err := disk.UsageWithContext(ctx, p.Mountpoint)
		if err == nil {
			usageStats = append(usageStats, UsageStat{
				Path:              u.Path,
				Fstype:            u.Fstype,
				Total:             u.Total,
				Free:              u.Free,
				Used:              u.Used,
				UsedPercent:       u.UsedPercent,
				InodesTotal:       u.InodesTotal,
				InodesUsed:        u.InodesUsed,
				InodesFree:        u.InodesFree,
				InodesUsedPercent: u.InodesUsedPercent,
			})
		}
	}

	ioCounters, err := disk.IOCountersWithContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get IO counters: %w", err)
	}

	ioStats := make(map[string]IOCountersStat)
	for k, v := range ioCounters {
		ioStats[k] = IOCountersStat{
			ReadCount:        v.ReadCount,
			MergedReadCount:  v.MergedReadCount,
			WriteCount:       v.WriteCount,
			MergedWriteCount: v.MergedWriteCount,
			ReadBytes:        v.ReadBytes,
			WriteBytes:       v.WriteBytes,
			ReadTime:         v.ReadTime,
			WriteTime:        v.WriteTime,
			IopsInProgress:   v.IopsInProgress,
			IoTime:           v.IoTime,
			WeightedIO:       v.WeightedIO,
			Name:             v.Name,
			SerialNumber:     v.SerialNumber,
			Label:            v.Label,
		}
	}

	return DiskResult{
		Partitions: partStats,
		Usage:      usageStats,
		IOCounters: ioStats,
	}, nil
}
