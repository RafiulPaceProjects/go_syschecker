package mcpserver

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/google/generative-ai-go/genai"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"google.golang.org/api/option"

	"syschecker/internal/collector"
	"syschecker/internal/database/graph"
	"syschecker/internal/database/rag"
	"syschecker/internal/database/relational"
	"syschecker/internal/flagger"
	"syschecker/internal/output"
)

// Server wraps the MCP server with SysChecker capabilities.
type Server struct {
	mcpServer      *mcp.Server
	ragEngine      *rag.GraphRAGEngine
	sensorProvider collector.StatsProvider
	duckdbRepo     *relational.Repo
	neo4jClient    graph.GraphClient
	geminiClient   *genai.Client
	flaggerSvc     *flagger.FlaggerService

	// Data ingestion background worker
	ingestMu     sync.Mutex
	ingestCancel context.CancelFunc
	ingestWg     sync.WaitGroup
}

// Config holds configuration for the MCP server.
type Config struct {
	ServerName    string
	ServerVersion string
	GeminiAPIKey  string
	GeminiModel   string // Model key: flash, pro, flash-8b, experimental
	Neo4jURI      string
	Neo4jUser     string
	Neo4jPassword string
	Neo4jDatabase string
}

// NewServer creates a new MCP server instance.
func NewServer(cfg Config, repo *relational.Repo, sensorProvider collector.StatsProvider) (*Server, error) {
	ctx := context.Background()

	// Initialize Gemini client
	geminiClient, err := genai.NewClient(ctx, option.WithAPIKey(cfg.GeminiAPIKey))
	if err != nil {
		return nil, fmt.Errorf("failed to create gemini client: %w", err)
	}

	// Initialize Neo4j client
	neo4jClient, err := graph.NewNeo4jClient(cfg.Neo4jURI, cfg.Neo4jUser, cfg.Neo4jPassword, cfg.Neo4jDatabase)
	if err != nil {
		geminiClient.Close()
		return nil, fmt.Errorf("failed to create neo4j client: %w", err)
	}

	// Initialize RAG Engine with model selection
	modelKey := cfg.GeminiModel
	if modelKey == "" {
		modelKey = "pro" // Default to pro for best reasoning
	}
	fmt.Fprintf(os.Stderr, "Using Gemini model: %s\n", modelKey)
	ragEngine := rag.NewGraphRAGEngine(neo4jClient, geminiClient, modelKey)

	// Initialize Flagger service for data pipeline
	flaggerCfg := flagger.DefaultConfig()
	flaggerSvc := flagger.NewFlaggerService(flaggerCfg)

	// Create MCP server with Implementation
	impl := &mcp.Implementation{
		Name:    cfg.ServerName,
		Version: cfg.ServerVersion,
	}
	mcpServer := mcp.NewServer(impl, nil)

	s := &Server{
		mcpServer:      mcpServer,
		ragEngine:      ragEngine,
		sensorProvider: sensorProvider,
		duckdbRepo:     repo,
		neo4jClient:    neo4jClient,
		geminiClient:   geminiClient,
		flaggerSvc:     flaggerSvc,
	}

	// Register tools
	s.registerTools()

	// Ingest initial data into Neo4j so RAG has something to query
	fmt.Fprintf(os.Stderr, "Ingesting initial system snapshot into Neo4j...\n")
	if err := s.ingestSnapshot(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: initial ingest failed: %v\n", err)
	} else {
		fmt.Fprintf(os.Stderr, "✓ Initial snapshot ingested into Neo4j\n")
	}

	// Start background ingestion (every 30 seconds)
	s.startBackgroundIngest(30 * time.Second)

	return s, nil
}

// AskSysCheckerArgs defines the input for ask_syschecker tool.
type AskSysCheckerArgs struct {
	Question string `json:"question" jsonschema:"the question to ask about system health"`
}

// AskSysCheckerResult defines the output for ask_syschecker tool.
type AskSysCheckerResult struct {
	Answer string `json:"answer" jsonschema:"AI-generated answer"`
}

// MetricsArgs defines the input for get_realtime_metrics tool.
type MetricsArgs struct {
	MetricType string `json:"metric_type" jsonschema:"metrics type: fast or slow"`
}

// MetricsResult wraps RawStats for tool output.
type MetricsResult struct {
	Stats *collector.RawStats `json:"stats" jsonschema:"system metrics"`
}

// QueryGraphArgs defines the input for query_graph tool.
type QueryGraphArgs struct {
	Cypher string `json:"cypher" jsonschema:"Cypher query to execute"`
}

// QueryGraphResult wraps graph query results.
type QueryGraphResult struct {
	Data interface{} `json:"data" jsonschema:"query results"`
}

// HistoricalSnapshotsArgs defines the input for get_historical_snapshots tool.
type HistoricalSnapshotsArgs struct {
	Hostname string `json:"hostname,omitempty" jsonschema:"hostname to filter by"`
	Limit    int    `json:"limit,omitempty" jsonschema:"number of snapshots to return"`
}

// HistoricalSnapshotsResult wraps snapshot results.
type HistoricalSnapshotsResult struct {
	Snapshots []relational.SnapshotSummary `json:"snapshots" jsonschema:"historical snapshots"`
}

// registerTools registers all available MCP tools.
func (s *Server) registerTools() {
	// Tool 1: ask_syschecker - GraphRAG-powered Q&A
	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "ask_syschecker",
		Description: "Ask complex questions about system health, performance issues, and root causes using AI-powered graph analysis. Use this for 'why' questions and causal reasoning about system behavior.",
	}, s.handleAskSysChecker)

	// Tool 2: get_realtime_metrics - Direct sensor access
	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "get_realtime_metrics",
		Description: "Get the absolute latest system metrics directly from sensors. Use this to verify current state or when you need real-time data (not historical). Returns CPU, RAM, disk, network, and process information.",
	}, s.handleGetRealtimeMetrics)

	// Tool 3: query_graph - Direct Cypher access for power users
	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "query_graph",
		Description: "Execute Cypher queries directly on the Neo4j graph database. For advanced users who want to explore the graph structure. Available nodes: Host, Snapshot, Flag, Cause, DiskDevice, NetInterface, Container.",
	}, s.handleQueryGraph)

	// Tool 4: get_historical_snapshots - Query DuckDB for time series
	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "get_historical_snapshots",
		Description: "Query historical snapshots from DuckDB. Use for time-series analysis and trend identification. Returns snapshot summaries with CPU, RAM, disk usage, severity levels, and explanations.",
	}, s.handleGetHistoricalSnapshots)
}

// handleAskSysChecker uses GraphRAG to answer complex questions.
func (s *Server) handleAskSysChecker(ctx context.Context, _ *mcp.CallToolRequest, args AskSysCheckerArgs) (*mcp.CallToolResult, AskSysCheckerResult, error) {
	// Use RAG engine to process the question
	answer, err := s.ragEngine.Query(ctx, args.Question)
	if err != nil {
		return nil, AskSysCheckerResult{}, fmt.Errorf("RAG query failed: %w", err)
	}

	return nil, AskSysCheckerResult{Answer: answer}, nil
}

// handleGetRealtimeMetrics fetches live data from sensors.
func (s *Server) handleGetRealtimeMetrics(ctx context.Context, _ *mcp.CallToolRequest, args MetricsArgs) (*mcp.CallToolResult, *collector.RawStats, error) {
	metricType := args.MetricType
	if metricType == "" {
		metricType = "fast"
	}

	var stats *collector.RawStats
	var err error

	switch metricType {
	case "fast":
		stats, err = s.sensorProvider.GetFastMetrics(ctx)
	case "slow":
		stats, err = s.sensorProvider.GetSlowMetrics(ctx)
	default:
		return nil, nil, fmt.Errorf("invalid metric_type: %s (must be 'fast' or 'slow')", metricType)
	}

	if err != nil {
		return nil, nil, fmt.Errorf("failed to get metrics: %w", err)
	}

	return nil, stats, nil
}

// handleQueryGraph executes Cypher queries.
func (s *Server) handleQueryGraph(ctx context.Context, _ *mcp.CallToolRequest, args QueryGraphArgs) (*mcp.CallToolResult, QueryGraphResult, error) {
	// Execute the query via Neo4j client
	result, err := s.neo4jClient.ExecuteCypher(ctx, args.Cypher)
	if err != nil {
		return nil, QueryGraphResult{}, fmt.Errorf("cypher query failed: %w", err)
	}

	return nil, QueryGraphResult{Data: result}, nil
}

// handleGetHistoricalSnapshots queries DuckDB.
func (s *Server) handleGetHistoricalSnapshots(ctx context.Context, _ *mcp.CallToolRequest, args HistoricalSnapshotsArgs) (*mcp.CallToolResult, HistoricalSnapshotsResult, error) {
	limit := args.Limit
	if limit == 0 {
		limit = 10
	}
	if limit > 100 {
		limit = 100
	}

	// Query snapshots from repo
	snapshots, err := s.duckdbRepo.QuerySnapshots(ctx, args.Hostname, limit)
	if err != nil {
		return nil, HistoricalSnapshotsResult{}, fmt.Errorf("failed to query snapshots: %w", err)
	}

	return nil, HistoricalSnapshotsResult{Snapshots: snapshots}, nil
}

// Start starts the MCP server using stdio transport.
func (s *Server) Start(ctx context.Context) error {
	fmt.Fprintf(os.Stderr, "Starting SysChecker MCP Server on stdio...\n")
	transport := &mcp.StdioTransport{}
	return s.mcpServer.Run(ctx, transport)
}

// Close cleans up resources.
func (s *Server) Close(ctx context.Context) error {
	// Stop background ingestion
	s.stopBackgroundIngest()

	if s.geminiClient != nil {
		s.geminiClient.Close()
	}
	if s.neo4jClient != nil {
		// Note: Not calling Reset() to preserve data between sessions
		// If ephemeral behavior is desired, uncomment: s.neo4jClient.Reset(ctx)
		s.neo4jClient.Close(ctx)
	}
	return nil
}

// ingestSnapshot runs the data pipeline once and ingests into Neo4j.
func (s *Server) ingestSnapshot(ctx context.Context) error {
	// Run the full pipeline: Collect -> Adapt -> Rates -> Flag -> Bundle
	payload, err := output.RunPipeline(
		ctx,
		s.sensorProvider,
		s.flaggerSvc,
		s.duckdbRepo,
		"mcp-server",
		"mcp-host",
		"mcp-session",
	)
	if err != nil {
		return fmt.Errorf("pipeline failed: %w", err)
	}

	// Persist to DuckDB (optional, for historical queries)
	if _, err := s.duckdbRepo.InsertRawStats(ctx, payload.Raw, payload.Derived, payload.Flags); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: DuckDB insert failed: %v\n", err)
	}

	// Ingest into Neo4j for RAG queries
	if err := s.neo4jClient.IngestSnapshot(ctx, payload); err != nil {
		return fmt.Errorf("neo4j ingest failed: %w", err)
	}

	return nil
}

// startBackgroundIngest starts periodic data ingestion.
func (s *Server) startBackgroundIngest(interval time.Duration) {
	s.ingestMu.Lock()
	defer s.ingestMu.Unlock()

	if s.ingestCancel != nil {
		return // Already running
	}

	ctx, cancel := context.WithCancel(context.Background())
	s.ingestCancel = cancel
	s.ingestWg.Add(1)

	go func() {
		defer s.ingestWg.Done()
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := s.ingestSnapshot(ctx); err != nil {
					fmt.Fprintf(os.Stderr, "Background ingest failed: %v\n", err)
				}
			}
		}
	}()

	fmt.Fprintf(os.Stderr, "Background data ingestion started (interval: %v)\n", interval)
}

// stopBackgroundIngest stops the periodic ingestion worker.
func (s *Server) stopBackgroundIngest() {
	s.ingestMu.Lock()
	cancel := s.ingestCancel
	s.ingestCancel = nil
	s.ingestMu.Unlock()

	if cancel != nil {
		cancel()
		s.ingestWg.Wait()
	}
}
