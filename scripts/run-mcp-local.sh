#!/bin/bash

# run-mcp-local.sh - Run local MCP server and client

set -e

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

echo -e "${BLUE}╔════════════════════════════════════════════╗${NC}"
echo -e "${BLUE}║  SysChecker Local MCP Setup               ║${NC}"
echo -e "${BLUE}╚════════════════════════════════════════════╝${NC}"
echo

# Check environment variables
if [ -z "$GEMINI_API_KEY" ]; then
    echo -e "${RED}✗ GEMINI_API_KEY not set${NC}"
    echo "  Set it with: export GEMINI_API_KEY='your-key'"
    echo "  Get a key from: https://aistudio.google.com/app/apikey"
    exit 1
fi

echo -e "${GREEN}✓ GEMINI_API_KEY set${NC}"

# Set defaults
export NEO4J_PASSWORD="${NEO4J_PASSWORD:-password}"
export NEO4J_URI="${NEO4J_URI:-bolt://localhost:7687}"
export DUCKDB_PATH="${DUCKDB_PATH:-syschecker.db}"

# Check Neo4j
echo -n "Checking Neo4j connection... "
if docker ps | grep -q syschecker-neo4j; then
    echo -e "${GREEN}✓ Running${NC}"
else
    echo -e "${YELLOW}⚠ Not running${NC}"
    echo "Starting Neo4j..."
    docker run -d --name syschecker-neo4j \
        -p 7474:7474 -p 7687:7687 \
        -e NEO4J_AUTH=neo4j/$NEO4J_PASSWORD \
        neo4j:latest
    echo -e "${GREEN}✓ Neo4j started${NC}"
    sleep 5
fi

# Check if DuckDB has data
if [ -f "$DUCKDB_PATH" ]; then
    echo -e "${GREEN}✓ DuckDB found at $DUCKDB_PATH${NC}"
else
    echo -e "${YELLOW}⚠ DuckDB not found${NC}"
    echo "  Run the main syschecker first to populate data:"
    echo "  go run main.go"
fi

echo

# Build binaries
echo -e "${BLUE}Building binaries...${NC}"

echo -n "  Building MCP server... "
if go build -o syschecker-mcp ./cmd/mcp; then
    echo -e "${GREEN}✓${NC}"
else
    echo -e "${RED}✗ Failed${NC}"
    exit 1
fi

echo -n "  Building MCP client... "
if go build -o syschecker-client ./cmd/mcp-client; then
    echo -e "${GREEN}✓${NC}"
else
    echo -e "${RED}✗ Failed${NC}"
    exit 1
fi

echo

# Run mode selection
MODE="${1:-interactive}"

if [ "$MODE" = "server" ]; then
    echo -e "${BLUE}Starting MCP server (stdio mode)...${NC}"
    ./syschecker-mcp
elif [ "$MODE" = "client" ]; then
    echo -e "${BLUE}Starting MCP client...${NC}"
    ./syschecker-client ./syschecker-mcp
else
    echo -e "${BLUE}Starting interactive client...${NC}"
    echo
    ./syschecker-client ./syschecker-mcp
fi
