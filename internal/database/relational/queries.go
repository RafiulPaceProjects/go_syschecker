package relational

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// SnapshotSummary represents a simplified snapshot for queries.
type SnapshotSummary struct {
	SnapshotID    int64     `json:"snapshot_id"`
	HostID        int64     `json:"host_id"`
	Hostname      string    `json:"hostname"`
	CollectedAt   time.Time `json:"collected_at"`
	Kind          string    `json:"kind"`
	CPUUsagePct   float64   `json:"cpu_usage_pct"`
	RAMUsagePct   float64   `json:"ram_usage_pct"`
	DiskUsagePct  float64   `json:"disk_usage_pct"`
	SeverityLevel int32     `json:"severity_level"`
	RiskScore     int32     `json:"risk_score"`
	PrimaryCause  string    `json:"primary_cause"`
	Explanation   string    `json:"explanation"`
}

// QuerySnapshots retrieves recent snapshots with optional filtering.
func (r *Repo) QuerySnapshots(ctx context.Context, hostname string, limit int) ([]SnapshotSummary, error) {
	if limit <= 0 {
		limit = 10
	}
	if limit > 100 {
		limit = 100 // Safety limit
	}

	query := `
		SELECT 
			s.snapshot_id,
			s.host_id,
			COALESCE(h.hostname, 'unknown') as hostname,
			s.collected_at,
			s.kind,
			s.cpu_usage_pct,
			s.ram_usage_pct,
			s.disk_usage_pct,
			s.severity_level,
			s.risk_score,
			COALESCE(s.primary_cause, '') as primary_cause,
			COALESCE(s.explanation, '') as explanation
		FROM snapshots s
		LEFT JOIN hosts h ON s.host_id = h.host_id
		WHERE 1=1
	`

	args := []interface{}{}
	if hostname != "" {
		query += " AND h.hostname = ?"
		args = append(args, hostname)
	}

	query += " ORDER BY s.collected_at DESC LIMIT ?"
	args = append(args, limit)

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query snapshots failed: %w", err)
	}
	defer rows.Close()

	snapshots := []SnapshotSummary{} // Initialize as empty slice, not nil
	for rows.Next() {
		var s SnapshotSummary
		var primaryCause, explanation sql.NullString

		err := rows.Scan(
			&s.SnapshotID,
			&s.HostID,
			&s.Hostname,
			&s.CollectedAt,
			&s.Kind,
			&s.CPUUsagePct,
			&s.RAMUsagePct,
			&s.DiskUsagePct,
			&s.SeverityLevel,
			&s.RiskScore,
			&primaryCause,
			&explanation,
		)
		if err != nil {
			return nil, fmt.Errorf("scan snapshot failed: %w", err)
		}

		if primaryCause.Valid {
			s.PrimaryCause = primaryCause.String
		}
		if explanation.Valid {
			s.Explanation = explanation.String
		}

		snapshots = append(snapshots, s)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration error: %w", err)
	}

	return snapshots, nil
}

// GetLatestSnapshot retrieves the most recent snapshot for a host.
func (r *Repo) GetLatestSnapshot(ctx context.Context, hostname string) (*SnapshotSummary, error) {
	snapshots, err := r.QuerySnapshots(ctx, hostname, 1)
	if err != nil {
		return nil, err
	}
	if len(snapshots) == 0 {
		return nil, fmt.Errorf("no snapshots found")
	}
	return &snapshots[0], nil
}
