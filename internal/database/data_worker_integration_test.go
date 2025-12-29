package database_test

import (
	"context"
	"database/sql"
	"fmt"
	"testing"
	"time"

	"syschecker/internal/collector"
	"syschecker/internal/database"
	"syschecker/internal/database/relational"
	"syschecker/internal/flagger"
	"syschecker/internal/output"
)

// TestDataWorkerPullAndPersist tests end-to-end: sensors -> DataWorker -> DuckDB
func TestDataWorkerPullAndPersist(t *testing.T) {
	ctx := context.Background()

	// 1. Create in-memory DuckDB
	client, err := relational.NewDuckDBClient("")
	if err != nil {
		t.Fatalf("failed to create duckdb client: %v", err)
	}
	defer client.Close()

	repo := relational.NewRepo(client.DB())

	// 2. Run migrations to create schema
	if err := repo.Migrate(ctx); err != nil {
		t.Fatalf("failed to migrate schema: %v", err)
	}
	t.Log("✓ Schema migrated successfully")

	// 3. Create Components
	col := collector.NewSystemCollector()
	cfg := flagger.DefaultConfig()
	flaggerSvc := flagger.NewFlaggerService(cfg)

	// 4. Create DataWorker
	// We use "test-agent" as ID
	mockGraph := &MockGraphClient{}
	worker, err := database.NewDataWorker(col, flaggerSvc, repo, mockGraph, "test-agent", "test-machine", "test-boot")
	if err != nil {
		t.Fatalf("failed to create data worker: %v", err)
	}

	// 5. Execute PullOnce (runs the pipeline)
	t.Log("Pulling sensor data...")
	if err := worker.PullOnce(ctx); err != nil {
		t.Fatalf("PullOnce failed: %v", err)
	}

	// 6. Verify data was inserted
	t.Log("\n========== VERIFICATION ==========")

	// Verify snapshot exists
	var snapCount int
	if err := client.DB().QueryRowContext(ctx, "SELECT COUNT(*) FROM snapshots").Scan(&snapCount); err != nil {
		t.Fatalf("Failed to verify snapshot: %v", err)
	} else if snapCount != 1 {
		t.Errorf("Expected 1 snapshot row, got %d", snapCount)
	} else {
		t.Log("✓ Snapshot row exists")
	}

	// Verify scalar values in snapshot
	var cpuPct, ramPct sql.NullFloat64
	var os, platform sql.NullString
	err = client.DB().QueryRowContext(ctx, `
		SELECT cpu_usage_pct, ram_usage_pct, os, platform 
		FROM snapshots LIMIT 1
	`).Scan(&cpuPct, &ramPct, &os, &platform)

	if err != nil {
		t.Errorf("Failed to read snapshot scalars: %v", err)
	} else {
		t.Logf("✓ Snapshot scalars - CPU: %.2f%%, RAM: %.2f%%, OS: %s, Platform: %s",
			cpuPct.Float64, ramPct.Float64, os.String, platform.String)
	}

	// Verify partitions were recorded
	var partCount int
	if err := client.DB().QueryRowContext(ctx, "SELECT COUNT(*) FROM snapshot_partition_usage").Scan(&partCount); err != nil {
		t.Errorf("Failed to count partitions: %v", err)
	} else {
		t.Logf("✓ Partition usage rows recorded: %d", partCount)
	}

	// Verify disk IO was recorded
	var diskIOCount int
	if err := client.DB().QueryRowContext(ctx, "SELECT COUNT(*) FROM snapshot_disk_io").Scan(&diskIOCount); err != nil {
		t.Errorf("Failed to count disk IO: %v", err)
	} else {
		t.Logf("✓ Disk IO counters recorded: %d", diskIOCount)
	}

	// Verify net interfaces were recorded
	var netCount int
	if err := client.DB().QueryRowContext(ctx, "SELECT COUNT(*) FROM snapshot_net_interface_stats").Scan(&netCount); err != nil {
		t.Errorf("Failed to count net interfaces: %v", err)
	} else {
		t.Logf("✓ Network interfaces recorded: %d", netCount)
	}

	// 7. Show schema and head for each table (Debug Output)
	t.Log("\n========== DATABASE SCHEMA & HEAD ==========")
	tables := []string{
		"hosts",
		"snapshots",
		"snapshot_partition_usage",
		"snapshot_disk_io",
		"snapshot_net_interface_stats",
	}

	for _, table := range tables {
		t.Logf("\n----- TABLE: %s -----", table)

		// Get row count
		var count int
		if err := client.DB().QueryRowContext(ctx, fmt.Sprintf("SELECT COUNT(*) FROM %s", table)).Scan(&count); err != nil {
			t.Logf("  [count error: %v]", err)
			continue
		}
		t.Logf("  ROW COUNT: %d", count)

		// Get head (first 3 rows)
		headRows, err := client.DB().QueryContext(ctx, fmt.Sprintf("SELECT * FROM %s LIMIT 3", table))
		if err != nil {
			t.Logf("  [head error: %v]", err)
			continue
		}

		cols, _ := headRows.Columns()
		t.Logf("  HEAD (columns: %v):", cols)

		values := make([]interface{}, len(cols))
		valuePtrs := make([]interface{}, len(cols))
		for i := range values {
			valuePtrs[i] = &values[i]
		}

		for headRows.Next() {
			if err := headRows.Scan(valuePtrs...); err != nil {
				t.Logf("    [row scan error: %v]", err)
				continue
			}

			rowStr := "    ["
			for i, v := range values {
				if i > 0 {
					rowStr += ", "
				}
				switch val := v.(type) {
				case nil:
					rowStr += "NULL"
				case []byte:
					rowStr += string(val)
				case time.Time:
					rowStr += val.Format("2006-01-02 15:04:05")
				default:
					rowStr += fmt.Sprintf("%v", val)
				}
			}
			rowStr += "]"
			t.Log(rowStr)
		}
		headRows.Close()
	}
	t.Log("\n========== TEST COMPLETE ==========")
}

// MockGraphClient
type MockGraphClient struct{}

func (m *MockGraphClient) Close(ctx context.Context) error { return nil }
func (m *MockGraphClient) Reset(ctx context.Context) error { return nil }
func (m *MockGraphClient) IngestSnapshot(ctx context.Context, payload *output.PipelinePayload) error {
	return nil
}

func (m *MockGraphClient) ExecuteCypher(ctx context.Context, query string) ([]map[string]any, error) {
	return nil, nil
}
