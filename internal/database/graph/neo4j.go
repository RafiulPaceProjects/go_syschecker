package graph

import (
	"context"
	"fmt"
	"time"

	"syschecker/internal/database/relational"
	"syschecker/internal/output"

	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
)

// GraphClient defines the interface for graph database operations.
type GraphClient interface {
	Close(ctx context.Context) error
	Reset(ctx context.Context) error
	IngestSnapshot(ctx context.Context, payload *output.PipelinePayload) error
	ExecuteCypher(ctx context.Context, query string) ([]map[string]any, error)
}

// Neo4jClient implements GraphClient for Neo4j.
type Neo4jClient struct {
	driver neo4j.DriverWithContext
	dbName string
}

// NewNeo4jClient creates a new Neo4j client.
func NewNeo4jClient(uri, username, password, dbName string) (*Neo4jClient, error) {
	driver, err := neo4j.NewDriverWithContext(uri, neo4j.BasicAuth(username, password, ""))
	if err != nil {
		return nil, fmt.Errorf("failed to create neo4j driver: %w", err)
	}

	// Verify connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := driver.VerifyConnectivity(ctx); err != nil {
		return nil, fmt.Errorf("failed to connect to neo4j: %w", err)
	}

	return &Neo4jClient{
		driver: driver,
		dbName: dbName,
	}, nil
}

func (c *Neo4jClient) Close(ctx context.Context) error {
	return c.driver.Close(ctx)
}

// Reset deletes all data in the graph.
func (c *Neo4jClient) Reset(ctx context.Context) error {
	session := c.driver.NewSession(ctx, neo4j.SessionConfig{DatabaseName: c.dbName})
	defer session.Close(ctx)

	_, err := session.ExecuteWrite(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		return tx.Run(ctx, "MATCH (n) DETACH DELETE n", nil)
	})
	return err
}

// IngestSnapshot pushes the pipeline payload into the graph.
func (c *Neo4jClient) IngestSnapshot(ctx context.Context, payload *output.PipelinePayload) error {
	session := c.driver.NewSession(ctx, neo4j.SessionConfig{DatabaseName: c.dbName})
	defer session.Close(ctx)

	_, err := session.ExecuteWrite(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		// 1. Merge Host
		if err := mergeHost(ctx, tx, payload.Raw); err != nil {
			return nil, err
		}

		// 2. Create Snapshot
		snapID, err := createSnapshot(ctx, tx, payload)
		if err != nil {
			return nil, err
		}

		// 3. Link Host -> Snapshot
		if err := linkHostSnapshot(ctx, tx, payload.Raw.AgentID, snapID); err != nil {
			return nil, err
		}

		// 4. Create Flags & Causes
		if err := createFlagsAndCauses(ctx, tx, snapID, payload.Flags); err != nil {
			return nil, err
		}

		// 5. Create Dimensions (Disks, Interfaces, etc.) & Links
		if err := createDimensions(ctx, tx, snapID, payload.Raw); err != nil {
			return nil, err
		}

		return nil, nil
	})

	return err
}

func mergeHost(ctx context.Context, tx neo4j.ManagedTransaction, raw relational.RawStatsFixed) error {
	query := `
		MERGE (h:Host {agent_id: $agent_id})
		SET h.host_id = $host_id,
			h.machine_id = $machine_id,
			h.boot_id = $boot_id,
			h.hostname = $hostname,
			h.os = $os,
			h.platform = $platform,
			h.kernel_version = $kernel_version
	`
	params := map[string]any{
		"agent_id":       raw.AgentID,
		"host_id":        raw.AgentID, // Using AgentID as HostID for simplicity if int64 not avail
		"machine_id":     raw.MachineID,
		"boot_id":        raw.BootID,
		"hostname":       raw.Hostname,
		"os":             raw.OS,
		"platform":       raw.Platform,
		"kernel_version": raw.KernelVersion,
	}
	_, err := tx.Run(ctx, query, params)
	return err
}

func createSnapshot(ctx context.Context, tx neo4j.ManagedTransaction, p *output.PipelinePayload) (string, error) {
	query := `
		CREATE (s:Snapshot {
			snapshot_id: $snapshot_id,
			collected_at: $collected_at,
			kind: $kind,
			
			cpu_usage_pct: $cpu_usage,
			ram_usage_pct: $ram_usage,
			disk_usage_pct: $disk_usage,
			
			severity_level: $severity,
			risk_score: $risk_score,
			primary_cause: $primary_cause,
			explanation: $explanation
		})
		RETURN elementId(s)
	`
	// Generate a unique ID for snapshot if not present, or use timestamp
	snapID := fmt.Sprintf("%s-%d", p.Raw.AgentID, p.Raw.CollectedAt.UnixNano())

	params := map[string]any{
		"snapshot_id":   snapID,
		"collected_at":  p.Raw.CollectedAt.Format(time.RFC3339),
		"kind":          string(p.Raw.Kind),
		"cpu_usage":     p.Raw.CPUUsagePct,
		"ram_usage":     p.Raw.RAMUsagePct,
		"disk_usage":    p.Raw.DiskUsagePct,
		"severity":      p.Flags.SeverityLevel,
		"risk_score":    p.Flags.RiskScore,
		"primary_cause": p.Flags.PrimaryCause,
		"explanation":   p.Flags.Explanation,
	}

	res, err := tx.Run(ctx, query, params)
	if err != nil {
		return "", err
	}

	rec, err := res.Single(ctx)
	if err != nil {
		return "", err
	}
	return rec.Values[0].(string), nil
}

func linkHostSnapshot(ctx context.Context, tx neo4j.ManagedTransaction, agentID, snapElementID string) error {
	query := `
		MATCH (h:Host {agent_id: $agent_id})
		MATCH (s:Snapshot) WHERE elementId(s) = $snap_id
		CREATE (h)-[:HAS_SNAPSHOT]->(s)
	`
	_, err := tx.Run(ctx, query, map[string]any{
		"agent_id": agentID,
		"snap_id":  snapElementID,
	})
	return err
}

func createFlagsAndCauses(ctx context.Context, tx neo4j.ManagedTransaction, snapElementID string, flags relational.SnapshotFlags) error {
	// 1. Triggered Flags
	flagMap := map[string]bool{
		"cpu_overloaded":      flags.FlagCPUOverloaded,
		"memory_pressure":     flags.FlagMemoryPressure,
		"disk_space_critical": flags.FlagDiskSpaceCritical,
		"network_latency":     flags.FlagNetworkLatencyDegraded,
		"disk_io_saturation":  flags.FlagDiskIOSaturation,
		"docker_unavailable":  flags.FlagDockerUnavailable,
	}

	for name, triggered := range flagMap {
		if triggered {
			query := `
				MATCH (s:Snapshot) WHERE elementId(s) = $snap_id
				MERGE (f:Flag {name: $name})
				CREATE (s)-[:TRIGGERED]->(f)
			`
			if _, err := tx.Run(ctx, query, map[string]any{"snap_id": snapElementID, "name": name}); err != nil {
				return err
			}
		}
	}

	// 2. Cause
	if flags.PrimaryCause != "" {
		query := `
			MATCH (s:Snapshot) WHERE elementId(s) = $snap_id
			CREATE (c:Cause {
				primary_cause: $primary,
				entity_type: $etype,
				entity_key: $ekey,
				explanation: $expl
			})
			CREATE (s)-[:HAS_CAUSE]->(c)
			RETURN elementId(c)
		`
		params := map[string]any{
			"snap_id": snapElementID,
			"primary": flags.PrimaryCause,
			"etype":   flags.CauseEntityType,
			"ekey":    flags.CauseEntityKey,
			"expl":    flags.Explanation,
		}
		res, err := tx.Run(ctx, query, params)
		if err != nil {
			return err
		}

		// Link Cause to Entity if possible
		// This requires creating the entity nodes first or merging them.
		// For simplicity, we'll skip the dynamic linking here or do it in createDimensions if we pass the cause ID.
		// But the prompt asks for: (Cause)-[:CAUSED_BY]->(Entity)

		// We can do a generic merge based on type
		causeElementIDRec, err := res.Single(ctx)
		if err == nil {
			causeElementID := causeElementIDRec.Values[0].(string)
			linkCauseToEntity(ctx, tx, causeElementID, flags.CauseEntityType, flags.CauseEntityKey)
		}
	}
	return nil
}

func linkCauseToEntity(ctx context.Context, tx neo4j.ManagedTransaction, causeID, eType, eKey string) {
	// Helper to link cause to specific entity types
	var query string
	switch eType {
	case "container":
		query = `
			MATCH (c:Cause) WHERE elementId(c) = $cause_id
			MERGE (t:Container {container_id: $key})
			CREATE (c)-[:CAUSED_BY]->(t)
		`
	case "disk":
		query = `
			MATCH (c:Cause) WHERE elementId(c) = $cause_id
			MERGE (t:DiskDevice {device: $key})
			CREATE (c)-[:CAUSED_BY]->(t)
		`
	case "netif":
		query = `
			MATCH (c:Cause) WHERE elementId(c) = $cause_id
			MERGE (t:NetInterface {name: $key})
			CREATE (c)-[:CAUSED_BY]->(t)
		`
	}

	if query != "" {
		tx.Run(ctx, query, map[string]any{"cause_id": causeID, "key": eKey})
	}
}

func createDimensions(ctx context.Context, tx neo4j.ManagedTransaction, snapElementID string, raw relational.RawStatsFixed) error {
	// 1. Disk Devices
	for _, io := range raw.IOCounters {
		query := `
			MATCH (s:Snapshot) WHERE elementId(s) = $snap_id
			MERGE (d:DiskDevice {device: $device, host_id: $agent_id})
			CREATE (s)-[:OBSERVED_DISK_IO {
				read_bytes: $rb, write_bytes: $wb,
				read_count: $rc, write_count: $wc
			}]->(d)
		`
		params := map[string]any{
			"snap_id":  snapElementID,
			"device":   io.Device,
			"agent_id": raw.AgentID,
			"rb":       io.ReadBytes,
			"wb":       io.WriteBytes,
			"rc":       io.ReadCount,
			"wc":       io.WriteCount,
		}
		if _, err := tx.Run(ctx, query, params); err != nil {
			return err
		}
	}

	// 2. Network Interfaces
	for _, net := range raw.NetInterfaces {
		query := `
			MATCH (s:Snapshot) WHERE elementId(s) = $snap_id
			MERGE (n:NetInterface {name: $name, host_id: $agent_id})
			CREATE (s)-[:OBSERVED_INTERFACE {
				bytes_sent: $bs, bytes_recv: $br,
				packets_sent: $ps, packets_recv: $pr,
				err_in: $ei, err_out: $eo
			}]->(n)
		`
		params := map[string]any{
			"snap_id":  snapElementID,
			"name":     net.Name,
			"agent_id": raw.AgentID,
			"bs":       net.BytesSent,
			"br":       net.BytesRecv,
			"ps":       net.PacketsSent,
			"pr":       net.PacketsRecv,
			"ei":       net.ErrIn,
			"eo":       net.ErrOut,
		}
		if _, err := tx.Run(ctx, query, params); err != nil {
			return err
		}
	}

	// 3. Containers
	for _, c := range raw.DockerContainers {
		query := `
			MATCH (s:Snapshot) WHERE elementId(s) = $snap_id
			MERGE (cnt:Container {container_id: $cid})
			SET cnt.name = $name, cnt.image = $image, cnt.host_id = $agent_id
			CREATE (s)-[:OBSERVED_CONTAINER {
				cpu_usage_pct: $cpu,
				mem_usage_bytes: $mem,
				status: $status
			}]->(cnt)
		`
		params := map[string]any{
			"snap_id":  snapElementID,
			"cid":      c.ID,
			"name":     c.Name,
			"image":    c.Image,
			"agent_id": raw.AgentID,
			"cpu":      c.CPUUsagePct,
			"mem":      c.MemUsageBytes,
			"status":   c.Status,
		}
		if _, err := tx.Run(ctx, query, params); err != nil {
			return err
		}
	}

	return nil
}

// ExecuteQuery runs a custom Cypher query and processes results with a callback.
func ExecuteQuery(ctx context.Context, client *Neo4jClient, query string, processRecord func(record map[string]any)) error {
	session := client.driver.NewSession(ctx, neo4j.SessionConfig{DatabaseName: client.dbName})
	defer session.Close(ctx)

	result, err := session.Run(ctx, query, nil)
	if err != nil {
		return fmt.Errorf("failed to run query: %w", err)
	}

	for result.Next(ctx) {
		record := result.Record()
		recordMap := make(map[string]any)
		for i, key := range record.Keys {
			recordMap[key] = record.Values[i]
		}
		processRecord(recordMap)
	}

	return result.Err()
}
