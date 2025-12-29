// Package relational provides interfaces for the data pipeline components.
package relational

import (
	"context"

	"syschecker/internal/collector"
)

// =============================================================================
// CORE INTERFACES
// =============================================================================

// StatsCollector defines the contract for collecting system metrics.
type StatsCollector interface {
	// GetFastMetrics collects high-frequency metrics (CPU, RAM, Disk IO, Net IO, Docker, Processes).
	GetFastMetrics(ctx context.Context) (*collector.RawStats, error)
	// GetSlowMetrics collects low-frequency metrics (Disk Health, Network Latency, Host Info, Physical).
	GetSlowMetrics(ctx context.Context) (*collector.RawStats, error)
}

// StatsFlagger analyzes metrics and attaches severity flags.
type StatsFlagger interface {
	// Flag analyzes the raw stats and returns flagged stats with severity assessment.
	Flag(stats *RawStatsFixed, derived *DerivedRates) *SnapshotFlags
}

// StatsRepository persists flagged metrics to storage.
type StatsRepository interface {
	// Migrate creates or updates the database schema.
	Migrate(ctx context.Context) error
	// GetDerivedRates retrieves the previous snapshot to compute rates of change.
	GetDerivedRates(ctx context.Context, current RawStatsFixed) (*DerivedRates, error)
	// InsertRawStats persists a flagged snapshot and returns the result.
	InsertRawStats(ctx context.Context, stats RawStatsFixed, derived DerivedRates, flags SnapshotFlags) (InsertResult, error)
	// GetCurrentState retrieves the latest state for a host.
	GetCurrentState(ctx context.Context, hostID int64) (map[string]any, error)
	// Close releases database resources.
	Close() error
}

// DataWorkerService orchestrates the data pipeline.
type DataWorkerService interface {
	// Start begins periodic data collection and persistence.
	Start(ctx context.Context) error
	// Stop gracefully stops the worker.
	Stop()
	// PullOnce executes a single collection cycle.
	PullOnce(ctx context.Context) error
}
