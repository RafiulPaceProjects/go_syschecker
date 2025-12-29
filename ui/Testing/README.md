# SysChecker Testing Chatbot

A simple terminal-based chatbot for testing the MCP server integration.

## Quick Start

```bash
# 1. Setup environment
cp env/.env.example env/.env
# Edit env/.env and add your GEMINI_API_KEY

# 2. Start Neo4j (if not running)
docker run -d --name syschecker-neo4j \
  -p 7474:7474 -p 7687:7687 \
  -e NEO4J_AUTH=neo4j/password \
  neo4j:latest

# 3. Populate data (run once)
cd ../..
go run main.go
# Wait a few seconds for data collection, then Ctrl+C

# 4. Run the chatbot
cd ui/Testing
chmod +x run.sh
./run.sh
```

## Manual Run

```bash
# Make sure you have the server built
cd ../..
go build -o syschecker-mcp ./cmd/mcp

# Run chatbot from Testing folder
cd ui/Testing
go run chatbot.go
```

## Usage

### Commands

- `/metrics` - Show current CPU, RAM, disk usage
- `/help` - Show help message
- `/exit` - Exit the chatbot

### Examples

```
💬 You: Why is my CPU high?
🤖 Bot: Thinking...
The CPU is high because container 'worker-1' is consuming 80% CPU...

💬 You: /metrics
🤖 Bot: Fetching metrics...
{
  "cpu_percent": 15.2,
  "ram_percent": 68.4,
  "disk_usage": {
    "/": 78.5
  }
}

💬 You: What containers are running?
🤖 Bot: Thinking...
There are 3 containers currently running: worker-1, db-replica, and nginx-proxy...
```

## Configuration

Edit `env/.env`:

```bash
# Required
GEMINI_API_KEY=your-api-key-here

# Optional (defaults shown)
NEO4J_URI=bolt://localhost:7687
NEO4J_PASSWORD=password
DUCKDB_PATH=../../syschecker.db
```

## Architecture

```
Terminal (chatbot.go)
    ↓
SimpleMCPClient (stdio pipes)
    ↓
MCP Server (../../syschecker-mcp)
    ↓
{Neo4j, DuckDB, Sensors, Gemini}
```

## Troubleshooting

### "GEMINI_API_KEY not set"
Edit `env/.env` and add your API key from https://aistudio.google.com/app/apikey

### "Server binary not found"
Build it: `cd ../.. && go build -o syschecker-mcp ./cmd/mcp`

### "Connection failed"
- Check Neo4j is running: `docker ps | grep neo4j`
- Check server logs in terminal

### "No data returned"
Run main syschecker to populate databases: `cd ../.. && go run main.go`

## Features

- ✅ Simple terminal interface
- ✅ MCP server communication via stdio
- ✅ Environment configuration via .env file
- ✅ GraphRAG-powered Q&A
- ✅ Real-time metrics
- ✅ Auto-setup script
- ✅ Error handling

## Development

Add new commands in `chatbot.go`:

```go
case "/mycommand":
    result, err := client.client.CallTool(ctx, "tool_name", map[string]interface{}{
        "param": "value",
    })
    // Handle result
```

The chatbot automatically loads environment from `env/.env` and validates configuration before starting.
