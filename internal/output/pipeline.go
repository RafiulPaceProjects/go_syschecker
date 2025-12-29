package output

import (
	"context"
	"fmt"

	"syschecker/internal/collector"
	"syschecker/internal/database/relational"
)

// PipelinePayload represents the final data object ready for persistence.
// The DataWorker pulls this from the Output layer to push to DuckDB.
type PipelinePayload struct {
	Raw     relational.RawStatsFixed
	Derived relational.DerivedRates
	Flags   relational.SnapshotFlags
}

// DataCollector defines the interface for collecting raw system stats.
type DataCollector interface {
	GetFastMetrics(ctx context.Context) (*collector.RawStats, error)
	GetSlowMetrics(ctx context.Context) (*collector.RawStats, error)
}

// DataFlagger defines the interface for flagging stats.
type DataFlagger interface {
	Flag(s *relational.RawStatsFixed, d *relational.DerivedRates) *relational.SnapshotFlags
}

// RateProvider defines the interface for retrieving derived rates (usually from DB).
type RateProvider interface {
	GetDerivedRates(ctx context.Context, current relational.RawStatsFixed) (*relational.DerivedRates, error)
}

// RunPipeline executes the full data pipeline: Collect -> Adapt -> Rates -> Flag -> Bundle.
// It returns a PipelinePayload ready for persistence.
func RunPipeline(
	ctx context.Context,
	col DataCollector,
	flg DataFlagger,
	rp RateProvider,
	agentID, machineID, bootID string,
) (*PipelinePayload, error) {
	// 1. Collect Fast Metrics
	fast, err := col.GetFastMetrics(ctx)
	if err != nil {
		return nil, fmt.Errorf("collect fast: %w", err)
	}

	// 2. Collect Slow Metrics
	slow, err := col.GetSlowMetrics(ctx)
	if err != nil {
		return nil, fmt.Errorf("collect slow: %w", err)
	}

	// 3. Merge & Adapt to Fixed/Relational Structure
	fixed := relational.MergeStats(fast, slow, agentID, machineID, bootID)

	// 4. Get Derived Rates (requires DB access to previous snapshot)
	derived, err := rp.GetDerivedRates(ctx, fixed)
	if err != nil {
		// If we can't get derived rates (e.g. first run), we proceed with zero values
		// but we should log it if it's not expected.
		// For now, just use empty derived rates.
		derived = &relational.DerivedRates{}
	}

	// 5. Flag the data
	flags := flg.Flag(&fixed, derived)

	// 6. Bundle into Output Payload
	return &PipelinePayload{
		Raw:     fixed,
		Derived: *derived,
		Flags:   *flags,
	}, nil
}
