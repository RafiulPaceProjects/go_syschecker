package flagger

// Thresholds defines warning and critical levels for metrics
type Thresholds struct {
	Warning  float64
	Critical float64
}

type Config struct {
	CPU       Thresholds
	RAM       Thresholds
	Disk      Thresholds
	Inode     Thresholds
	Net       Thresholds // ms
	ActiveTCP Thresholds
}

func DefaultConfig() Config {
	return Config{
		CPU:       Thresholds{Warning: 70.0, Critical: 90.0},
		RAM:       Thresholds{Warning: 70.0, Critical: 90.0},
		Disk:      Thresholds{Warning: 80.0, Critical: 90.0},
		Inode:     Thresholds{Warning: 80.0, Critical: 90.0},
		Net:       Thresholds{Warning: 150.0, Critical: 500.0},
		ActiveTCP: Thresholds{Warning: 200.0, Critical: 500.0},
	}
}
