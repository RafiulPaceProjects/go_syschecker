package services

import (
	"context"
	"fmt"

	"github.com/shirou/gopsutil/v4/net"
)

type NetInterfaceStats struct {
	Name        string
	BytesSent   uint64
	BytesRecv   uint64
	PacketsSent uint64
	PacketsRecv uint64
	ErrIn       uint64
	ErrOut      uint64
	DropIn      uint64
	DropOut     uint64
}

type NetResult struct {
	Interfaces []NetInterfaceStats
}

type NetSensor struct{}

func NewNetSensor() *NetSensor {
	return &NetSensor{}
}

func (s *NetSensor) Name() string {
	return "Network"
}

func (s *NetSensor) Connect(ctx context.Context) error {
	return nil
}

func (s *NetSensor) Disconnect(ctx context.Context) error {
	return nil
}

func (s *NetSensor) Collect(ctx context.Context) (any, error) {
	counters, err := net.IOCountersWithContext(ctx, true)
	if err != nil {
		return nil, fmt.Errorf("failed to get net io counters: %w", err)
	}

	var stats []NetInterfaceStats
	for _, c := range counters {
		stats = append(stats, NetInterfaceStats{
			Name:        c.Name,
			BytesSent:   c.BytesSent,
			BytesRecv:   c.BytesRecv,
			PacketsSent: c.PacketsSent,
			PacketsRecv: c.PacketsRecv,
			ErrIn:       c.Errin,
			ErrOut:      c.Errout,
			DropIn:      c.Dropin,
			DropOut:     c.Dropout,
		})
	}

	return NetResult{Interfaces: stats}, nil
}
