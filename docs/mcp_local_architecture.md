# SysChecker Local MCP Architecture

## Overview

A local Model Context Protocol (MCP) implementation using **stdio transport** for process-to-process communication. The server runs as a subprocess, communicating via stdin/stdout pipes.

## Architecture Diagram

```
┌─────────────────────────────────────────────────────────────┐
│  Terminal / User Interface                                  │
└───────────────────────────┬─────────────────────────────────┘
                            │
                            │ User Input
                            │
┌───────────────────────────▼─────────────────────────────────┐
│  MCP Client (syschecker-client)                             │
│  ┌────────────────────────────────────────────────────────┐ │
│  │  Interactive CLI                                       │ │
│  │  • Parse commands                                      │ │
│  │  • Format output                                       │ │
│  │  • Handle user interaction                             │ │
│  └────────────────────────────────────────────────────────┘ │
│                                                              │
│  ┌────────────────────────────────────────────────────────┐ │
│  │  MCP Client SDK                                        │ │
│  │  • CallTool()                                          │ │
│  │  • ListTools()                                         │ │
│  │  • Manage connection                                   │ │
│  └────────────────────────────────────────────────────────┘ │
└───────────────────────────┬─────────────────────────────────┘
                            │
                            │ stdio (pipes)
                            │ JSON-RPC messages
                            │
┌───────────────────────────▼─────────────────────────────────┐
│  MCP Server (syschecker-mcp)                                │
│  ┌────────────────────────────────────────────────────────┐ │
│  │  MCP Server SDK                                        │ │
│  │  • Handle requests via stdin                           │ │
│  │  • Send responses via stdout                           │ │
│  │  • Tool registration                                   │ │
│  └────────────────────────────────────────────────────────┘ │
│                                                              │
│  ┌────────────────────────────────────────────────────────┐ │
│  │  4 Tools:                                              │ │
│  │  1. ask_syschecker       - GraphRAG Q&A               │ │
│  │  2. get_realtime_metrics - Live sensor data           │ │
│  │  3. query_graph          - Cypher queries             │ │
│  │  4. get_historical_snapshots - Time-series analysis   │ │
│  └────────────────────────────────────────────────────────┘ │
│                                                              │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐     │
│  │  GraphRAG    │  │   Sensor     │  │  Relational  │     │
│  │   Engine     │  │ Orchestrator │  │     Repo     │     │
│  └──────┬───────┘  └──────┬───────┘  └──────┬───────┘     │
└─────────┼──────────────────┼──────────────────┼─────────────┘
          │                  │                  │
          │                  │                  │
    ┌─────▼──────┐    ┌─────▼────────┐  ┌─────▼───────┐
    │   Gemini   │    │   gopsutil   │  │   DuckDB    │
    │    API     │    │   (system)   │  │  (local)    │
    └─────┬──────┘    └──────────────┘  └─────────────┘
          │
    ┌─────▼──────┐
    │   Neo4j    │
    │  (Docker)  │
    └────────────┘
```

## Why stdio?

1. **Simplicity**: No network configuration, ports, or HTTP servers
2. **Security**: Process-local communication, no network exposure
3. **Standard**: stdio is the recommended MCP transport for local use
4. **Isolation**: Server runs as subprocess, clean lifecycle management
5. **Debugging**: Easy to inspect messages via stderr logging

## Components

### 1. MCP Server (`cmd/mcp/main.go`)

**Purpose**: Standalone server binary that communicates via stdio

**Responsibilities**:
- Initialize all dependencies (DuckDB, Neo4j, Gemini, sensors)
- Create MCP server with stdio transport
- Register 4 tools
- Read JSON-RPC requests from stdin
- Write JSON-RPC responses to stdout
- Log diagnostics to stderr

**Lifecycle**:
```bash
# Started by client as subprocess
./syschecker-mcp

# Stdin: Receives JSON-RPC requests
# Stdout: Sends JSON-RPC responses
# Stderr: Server logs (initialization, errors)
```

### 2. MCP Client (`cmd/mcp-client/main.go`)

**Purpose**: Interactive CLI that spawns server and communicates

**Responsibilities**:
- Spawn server as subprocess
- Create stdin/stdout pipes
- Wrap MCP client SDK
- Provide interactive CLI interface
- Format and display results
- Handle graceful shutdown

**Features**:
- **Interactive Mode**: Read user commands, execute tools
- **List Tools**: Discover available capabilities
- **Call Tools**: Execute with parameters
- **Pretty Output**: JSON formatting for readability

### 3. Server Package (`internal/MCP server/server.go`)

**Core Logic**: Tool implementations and business logic

**Tools**:

#### ask_syschecker
- Uses GraphRAG engine
- Gemini generates Cypher query
- Executes on Neo4j
- Synthesizes natural language answer

#### get_realtime_metrics
- Direct sensor orchestrator access
- Fast mode: CPU, RAM, disk (< 1s)
- Slow mode: Network, disk health (5-30s)

#### query_graph
- Raw Cypher execution
- Direct Neo4j access
- Power user tool

#### get_historical_snapshots
- DuckDB time-series queries
- Hostname filtering
- Pagination support

## Communication Protocol

### JSON-RPC Over stdio

**Request** (stdin):
```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "tools/call",
  "params": {
    "name": "ask_syschecker",
    "arguments": {
      "question": "Why is CPU high?"
    }
  }
}
```

**Response** (stdout):
```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "result": {
    "content": [
      {
        "type": "text",
        "text": "CPU is high due to container xyz consuming 80%..."
      }
    ]
  }
}
```

**Logs** (stderr):
```
Initializing DuckDB at syschecker.db...
Initializing sensor orchestrator...
Initializing MCP server...
Starting SysChecker MCP Server...
```

## Usage

### Quick Start

```bash
# Set environment
export GEMINI_API_KEY='your-key'

# Run everything
chmod +x scripts/run-mcp-local.sh
./scripts/run-mcp-local.sh
```

### Manual Start

```bash
# Build binaries
go build -o syschecker-mcp ./cmd/mcp
go build -o syschecker-client ./cmd/mcp-client

# Start interactive client (spawns server automatically)
./syschecker-client ./syschecker-mcp
```

### Interactive Commands

```
> help                       # Show help
> tools                      # List available tools
> metrics                    # Get fast metrics
> metrics-slow               # Get detailed metrics
> history                    # Last 5 snapshots
> graph                      # Enter Cypher query
> Why is my server slow?     # Ask question (GraphRAG)
> exit                       # Quit
```

### Examples

**Get Current Metrics**:
```
> metrics

📊 Fetching fast metrics...
✅ Result:
{
  "cpu_percent": 12.5,
  "ram_percent": 68.2,
  "disk_usage": {
    "/": 78.4
  }
}
```

**Ask Question**:
```
> What caused the memory spike?

🤖 Processing question with GraphRAG...
✅ Result:
The memory spike at 2025-12-28 14:23:00 was caused by container 
'worker-1' which had a memory leak in the data processing pipeline. 
The container was consuming 4.2GB RAM, triggering the 'mem_overloaded' 
flag with severity level 4.
```

**Query Graph**:
```
> graph
Enter Cypher query: MATCH (c:Container) WHERE c.memory_percent > 80 RETURN c.name, c.memory_percent

🔍 Executing Cypher query...
✅ Result:
[
  {
    "c.name": "worker-1",
    "c.memory_percent": 95.2
  },
  {
    "c.name": "db-replica",
    "c.memory_percent": 87.6
  }
]
```

## Development

### Adding New Tools

1. Define tool in `internal/MCP server/server.go`:
```go
func (s *Server) registerTools() error {
    // Add new tool
    if err := s.mcpServer.AddTool(mcp.Tool{
        Name: "my_new_tool",
        Description: "Does something cool",
        InputSchema: mcp.ToolInputSchema{
            Type: "object",
            Properties: map[string]interface{}{
                "param": map[string]interface{}{
                    "type": "string",
                    "description": "A parameter",
                },
            },
            Required: []string{"param"},
        },
    }, s.handleMyNewTool); err != nil {
        return err
    }
    return nil
}

func (s *Server) handleMyNewTool(ctx context.Context, params map[string]interface{}) (*mcp.CallToolResult, error) {
    param, _ := params["param"].(string)
    // Do something
    return mcp.NewToolResultText("Result"), nil
}
```

2. Rebuild binaries:
```bash
go build -o syschecker-mcp ./cmd/mcp
```

3. Client automatically discovers new tools via `ListTools()`

### Testing Server Standalone

```bash
# Start server manually (stdio mode)
./syschecker-mcp

# Send JSON-RPC request
echo '{"jsonrpc":"2.0","id":1,"method":"tools/list"}' | ./syschecker-mcp
```

### Debugging

**Server logs** (stderr):
```bash
./syschecker-mcp 2> server.log
```

**Client with verbose output**:
```bash
# Modify client to log requests/responses
// In client code:
log.Printf("Request: %+v", request)
log.Printf("Response: %+v", response)
```

## Performance

| Operation | Latency | Notes |
|-----------|---------|-------|
| Client → Server startup | 2-3s | One-time initialization |
| get_realtime_metrics (fast) | < 1s | Direct sensor read |
| get_realtime_metrics (slow) | 5-30s | Network tests, SMART checks |
| query_graph | < 1s | Depends on Cypher complexity |
| get_historical_snapshots | < 500ms | DuckDB is fast |
| ask_syschecker | 2-5s | Gemini API + Neo4j query |

## Security

- **No network exposure**: All communication via process pipes
- **Read-only queries**: Cypher execution restricted (no WRITE/DELETE)
- **Local credentials**: API keys in environment, never logged
- **Process isolation**: Server runs as unprivileged subprocess

## Comparison: stdio vs HTTP/SSE

| Feature | stdio (Current) | HTTP/SSE |
|---------|-----------------|----------|
| Setup complexity | Simple | Moderate |
| Network required | No | Yes |
| Port management | No | Yes (must allocate) |
| Authentication | Not needed | Required |
| Firewall issues | No | Possible |
| Use case | Local CLI | Multi-user, remote |

## Future Enhancements

1. **Batch Requests**: Send multiple tool calls in parallel
2. **Streaming**: Real-time updates for long queries
3. **Caching**: Cache GraphRAG responses
4. **History**: Command history in client
5. **Autocomplete**: Tab completion for commands
6. **Config Files**: Client configuration for preferences

## Troubleshooting

### Client can't connect
```
Failed to start server: exec: "./syschecker-mcp": no such file or directory
```
**Solution**: Build server first: `go build -o syschecker-mcp ./cmd/mcp`

### Server initialization fails
```
Failed to create neo4j client: connection refused
```
**Solution**: Start Neo4j: `docker start syschecker-neo4j` or run setup script

### No data returned
```
✅ Result:
[]
```
**Solution**: Populate databases by running main syschecker: `go run main.go`

### Gemini API errors
```
Failed to generate cypher: API key not valid
```
**Solution**: Check `GEMINI_API_KEY` is set correctly

## Summary

✅ **Local architecture**: Client spawns server as subprocess  
✅ **stdio transport**: Standard MCP pattern for local use  
✅ **Interactive CLI**: User-friendly command interface  
✅ **4 powerful tools**: GraphRAG, sensors, graph queries, history  
✅ **Simple deployment**: Single binary, no network config  
✅ **Production ready**: Proper error handling, logging, shutdown  

The local MCP architecture provides a robust, secure, and user-friendly way to interact with SysChecker's monitoring capabilities without the complexity of network-based solutions.
