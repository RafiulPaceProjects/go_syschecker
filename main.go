package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"syschecker/internal/collector"
	"syschecker/internal/database"
	"syschecker/internal/database/relational"
	"syschecker/internal/flagger"
	"syschecker/ui/tui"
	"time"
)

func main() {
	// 1. Initialize Collector
	// Use the interface to allow for different collector implementations
	var provider collector.StatsProvider = collector.NewSystemCollector()

	// 2. Initialize Config
	cfg := flagger.DefaultConfig()

	// 3. Initialize Database (DuckDB)
	// Use a file-based DB for persistence, or ":memory:" for ephemeral
	dbClient, err := relational.NewDuckDBClient("syschecker.db", relational.WithThreads(4))
	if err != nil {
		log.Fatalf("Failed to initialize DuckDB: %v", err)
	}
	defer dbClient.Close()

	// 4. Initialize Repository
	repo := relational.NewRepo(dbClient.DB())
	// Ensure schema exists
	if err := repo.Migrate(context.Background()); err != nil {
		log.Fatalf("Failed to migrate database: %v", err)
	}

	// 5. Initialize Flagger
	flaggerSvc := flagger.NewFlaggerService(cfg)

	// 6. Get Host Info for Worker Identity
	// We do a quick fetch to get stable IDs
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// We need to cast provider to *SystemCollector to access GetSlowMetrics directly if needed,
	// or just use the provider if it exposes what we need.
	// However, DataWorker expects relational.StatsCollector which provider implements.
	// But we need IDs first.
	// Let's just let the worker handle it? No, worker needs them in constructor.
	// Let's fetch them.
	sysCol, ok := provider.(relational.StatsCollector)
	if !ok {
		log.Fatalf("Provider does not implement StatsCollector")
	}

	slowStats, err := sysCol.GetSlowMetrics(ctx)
	if err != nil {
		log.Printf("Warning: could not fetch initial host info: %v", err)
		// Proceed with empty IDs, they might be filled later or cause issues?
		// Ideally we want them.
	}

	agentID := "default-agent"
	machineID := ""
	bootID := ""
	if slowStats != nil && slowStats.Hostname != "" {
		agentID = slowStats.Hostname
	}

	// 8. Initialize Data Worker
	worker, err := database.NewDataWorker(sysCol, flaggerSvc, repo, nil, agentID, machineID, bootID)
	if err != nil {
		log.Fatalf("Failed to create data worker: %v", err)
	}

	// 9. Start Data Worker
	if err := worker.Start(context.Background()); err != nil {
		log.Fatalf("Failed to start data worker: %v", err)
	}
	defer worker.Stop()

	// 10. Start TUI
	if err := tui.Start(provider, cfg); err != nil {
		fmt.Printf("Error running TUI: %v\n", err)
		os.Exit(1)
	}
}
