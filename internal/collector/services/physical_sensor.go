package services

import (
	"context"
	"fmt"

	"github.com/shirou/gopsutil/v4/sensors"
)

type TempStat struct {
	SensorKey   string
	Temperature float64
}

type PhysicalResult struct {
	Temperatures []TempStat
}

type PhysicalSensor struct{}

func NewPhysicalSensor() *PhysicalSensor {
	return &PhysicalSensor{}
}

func (s *PhysicalSensor) Name() string {
	return "Physical"
}

func (s *PhysicalSensor) Connect(ctx context.Context) error {
	return nil
}

func (s *PhysicalSensor) Disconnect(ctx context.Context) error {
	return nil
}

func (s *PhysicalSensor) Collect(ctx context.Context) (any, error) {
	data, err := sensors.TemperaturesWithContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get temperatures: %w", err)
	}

	var temps []TempStat
	for _, t := range data {
		temps = append(temps, TempStat{
			SensorKey:   t.SensorKey,
			Temperature: t.Temperature,
		})
	}

	return PhysicalResult{Temperatures: temps}, nil
}
