package services

import (
	"context"
	"encoding/json"
	"testing"
)

type sensorTestCase struct {
	name     string
	factory  func() Sensor
	optional bool
}

var sensorCases = []sensorTestCase{
	{name: "CPU", factory: func() Sensor { return NewCPUSensor() }},
	{name: "Memory", factory: func() Sensor { return NewMemSensor() }},
	{name: "Disk", factory: func() Sensor { return NewDiskSensor() }},
	{name: "Network", factory: func() Sensor { return NewNetSensor() }},
	{name: "Host", factory: func() Sensor { return NewHostSensor() }},
	{name: "Process", factory: func() Sensor { return NewProcessSensor() }},
	{name: "Physical", factory: func() Sensor { return NewPhysicalSensor() }, optional: true},
	{name: "Docker", factory: func() Sensor { return NewDockerSensor() }, optional: true},
}

func TestSensorsSuite(t *testing.T) {
	ctx := context.Background()

	for _, tc := range sensorCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			sensor := tc.factory()

			if err := sensor.Connect(ctx); err != nil {
				t.Fatalf("%s Connect failed: %v", tc.name, err)
			}
			defer sensor.Disconnect(ctx)

			result, err := sensor.Collect(ctx)
			if err != nil {
				if tc.optional {
					t.Logf("%s Collect skipped (optional): %v", tc.name, err)
					return
				}
				t.Fatalf("%s Collect failed: %v", tc.name, err)
			}
			if result == nil {
				t.Fatalf("%s Collect returned nil result", tc.name)
			}

			logSensorResult(t, tc.name, result)
		})
	}
}

func logSensorResult(t *testing.T, name string, result any) {
	t.Helper()

	payload, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		t.Logf("%s result: %+v", name, result)
		return
	}

	t.Logf("%s result:\n%s", name, payload)
}
