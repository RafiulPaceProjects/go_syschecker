package services

import "context"

// Sensor defines the interface for all system sensors.
type Sensor interface {
	Name() string
	Connect(ctx context.Context) error
	Disconnect(ctx context.Context) error
	Collect(ctx context.Context) (any, error)
}
