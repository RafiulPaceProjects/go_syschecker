# SysChecker

**SysChecker** is an AI-powered system monitoring platform that leverages the **Model Context Protocol (MCP)** to provide intelligent insights into system health, performance, and historical trends.

![License](https://img.shields.io/badge/license-MIT-blue.svg)
![Go Version](https://img.shields.io/badge/go-1.25.5-cyan)
![MCP](https://img.shields.io/badge/MCP-ready-green)
![Gemini](https://img.shields.io/badge/Gemini-powered-orange)

## Features

- **AI-Powered Q&A**: Ask natural language questions like "Why is my server slow?" or "Show me memory trends" using GraphRAG.
- **Real-Time Monitoring**: Instant access to CPU, Memory, Disk, and Network metrics via MCP tools.
- **Historical Analysis**: Query time-series data stored in DuckDB for deep performance analysis.
- **Graph-Based Insights**: Relationships between system components and alerts are stored in Neo4j for complex correlation.
- **MCP Integration**: Seamlessly connects with Claude Desktop or any MCP-compatible client.
- **Interactive Chatbot**: A dedicated terminal-based chatbot for direct interaction with the system.

## Architecture

SysChecker consists of:
1.  **MCP Server**: The core engine exposing tools for metrics, graph queries, and AI analysis.
2.  **GraphRAG Engine**: Uses Google Gemini to synthesize answers from system data.
3.  **Storage Layer**: Neo4j for graph relationships and DuckDB for high-performance time-series data.
4.  **Sensors**: Low-level collectors for real-time system metrics.

## Prerequisites

- **Go 1.25+**
- **Docker** (for Neo4j)
- **Google Gemini API Key**: [Get one here](https://aistudio.google.com/app/apikey)

## Quick Start

### 1. Setup Environment

Create a `.env` file in `ui/Testing/env/` (or set environment variables):

```bash
GEMINI_API_KEY=your_api_key_here
NEO4J_PASSWORD=password
```

### 2. Start Neo4j

```bash
docker run -d --name syschecker-neo4j \
    -p 7474:7474 -p 7687:7687 \
    -e NEO4J_AUTH=neo4j/password \
    neo4j:latest
```

### 3. Build and Run

You can use the provided scripts for a quick setup:

#### Run the Chatbot
```bash
./ui/Testing/run.sh
```

#### Run MCP Locally (Server + Client)
```bash
./scripts/run-mcp-local.sh
```

## Usage

### Chatbot Commands
Once the chatbot is running, you can ask:
- "What is the current CPU usage?"
- "Are there any disk space issues?"
- "Show me the memory trend for the last hour."
- "Why did the system flag a high load earlier?"

### Claude Desktop Integration
To use SysChecker with Claude Desktop, add the configuration from `configs/claude_desktop_config.json` to your Claude configuration file.

## Documentation

Detailed documentation is available in the `docs/` directory:
- [MCP Implementation](docs/mcp_implementation.md)
- [Local MCP Architecture](docs/mcp_local_architecture.md)
- [Architecture Overview](docs/architecture/overview.md)
- [Sensor Report](docs/sensor_report.md)
- [Refactoring Solutions](docs/refactoring_solutions.md)
