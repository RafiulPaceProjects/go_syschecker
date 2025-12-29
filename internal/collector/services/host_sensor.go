package services

import (
	"context"
	"fmt"

	"github.com/shirou/gopsutil/v4/host"
)

type HostResult struct {
	Hostname           string
	OS                 string
	Platform           string
	PlatformFamily     string
	PlatformVersion    string
	KernelVersion      string
	KernelArch         string
	Virtualization     string
	VirtualizationRole string
	HostID             string
	BootTime           uint64
	Uptime             uint64
	Procs              uint64
}

type HostSensor struct{}

func NewHostSensor() *HostSensor {
	return &HostSensor{}
}

func (s *HostSensor) Name() string {
	return "Host"
}

func (s *HostSensor) Connect(ctx context.Context) error {
	return nil
}

func (s *HostSensor) Disconnect(ctx context.Context) error {
	return nil
}

func (s *HostSensor) Collect(ctx context.Context) (any, error) {
	info, err := host.InfoWithContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get host info: %w", err)
	}

	return HostResult{
		Hostname:           info.Hostname,
		OS:                 info.OS,
		Platform:           info.Platform,
		PlatformFamily:     info.PlatformFamily,
		PlatformVersion:    info.PlatformVersion,
		KernelVersion:      info.KernelVersion,
		KernelArch:         info.KernelArch,
		Virtualization:     info.VirtualizationSystem,
		VirtualizationRole: info.VirtualizationRole,
		HostID:             info.HostID,
		BootTime:           info.BootTime,
		Uptime:             info.Uptime,
		Procs:              info.Procs,
	}, nil
}
