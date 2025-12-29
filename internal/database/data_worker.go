package database

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"syschecker/internal/database/graph"
	"syschecker/internal/database/relational"
	"syschecker/internal/output"
)

const defaultPollInterval = 20 * time.Second

// DataWorker orchestrates the data pipeline: Collector -> Flagger -> Repo.
type DataWorker struct {
	collector   relational.StatsCollector
	flagger     relational.StatsFlagger
	repo        relational.StatsRepository
	graphClient graph.GraphClient
	interval    time.Duration
	agentID     string
	machineID   string
	bootID      string

	mu      sync.Mutex
	cancel  context.CancelFunc
	running bool
	wg      sync.WaitGroup
}

// NewDataWorker creates a new worker instance.
func NewDataWorker(
	c relational.StatsCollector,
	f relational.StatsFlagger,
	r relational.StatsRepository,
	g graph.GraphClient,
	agentID, machineID, bootID string,
) (*DataWorker, error) {
	if c == nil || f == nil || r == nil {
		return nil, errors.New("collector, flagger, and repo are required")
	}
	return &DataWorker{
		collector:   c,
		flagger:     f,
		repo:        r,
		graphClient: g,
		interval:    defaultPollInterval,
		agentID:     agentID,
		machineID:   machineID,
		bootID:      bootID,
	}, nil
}

// Start begins the periodic data collection loop.
func (w *DataWorker) Start(ctx context.Context) error {
	w.mu.Lock()
	if w.running {
		w.mu.Unlock()
		return errors.New("worker already running")
	}
	ctx, cancel := context.WithCancel(ctx)
	w.cancel = cancel
	w.running = true
	w.wg.Add(1)
	w.mu.Unlock()

	go w.loop(ctx)
	return nil
}

// Stop gracefully stops the worker.
func (w *DataWorker) Stop() {
	w.mu.Lock()
	cancel := w.cancel
	w.cancel = nil
	w.running = false
	w.mu.Unlock()

	if cancel != nil {
		cancel()
	}
	w.wg.Wait()

	// Reset graph data on stop (ephemeral session)
	if w.graphClient != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		w.graphClient.Reset(ctx)
		w.graphClient.Close(ctx)
	}
}

// PullOnce executes a single collection cycle immediately.
func (w *DataWorker) PullOnce(ctx context.Context) error {
	return w.execute(ctx)
}

func (w *DataWorker) loop(ctx context.Context) {
	defer w.wg.Done()
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := w.execute(ctx); err != nil {
				// In a real app, use a logger
				fmt.Printf("Worker execution failed: %v\n", err)
			}
		}
	}
}

func (w *DataWorker) execute(ctx context.Context) error {
	// Run the pipeline via the Output layer (the "lever")
	payload, err := output.RunPipeline(
		ctx,
		w.collector,
		w.flagger,
		w.repo,
		w.agentID,
		w.machineID,
		w.bootID,
	)
	if err != nil {
		return fmt.Errorf("pipeline execution failed: %w", err)
	}

	// Persist the final payload to DuckDB
	_, err = w.repo.InsertRawStats(ctx, payload.Raw, payload.Derived, payload.Flags)
	if err != nil {
		return fmt.Errorf("persist stats: %w", err)
	}

	// Push to Graph DB asynchronously
	if w.graphClient != nil {
		w.wg.Add(1)
		go func() {
			defer w.wg.Done()
			// Use a separate context or the worker context?
			// If worker context is canceled, we might want to abort graph push.
			// But usually we want to finish the push.
			// Let's use a detached context with timeout to ensure it finishes or times out.
			pushCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			if err := w.graphClient.IngestSnapshot(pushCtx, payload); err != nil {
				fmt.Printf("Graph ingest failed: %v\n", err)
			}
		}()
	}

	return nil
}
