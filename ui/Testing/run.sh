#!/bin/bash

# Simple chatbot runner script

set -e

# Change to script directory
SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
cd "$SCRIPT_DIR"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

echo -e "${BLUE}╔════════════════════════════════════════════╗${NC}"
echo -e "${BLUE}║  SysChecker Chatbot Setup                 ║${NC}"
echo -e "${BLUE}╚════════════════════════════════════════════╝${NC}"
echo

# Check if .env exists
if [ ! -f "env/.env" ]; then
    echo -e "${YELLOW}⚠ env/.env not found${NC}"
    echo "Creating from template..."
    cp env/.env.example env/.env
    echo -e "${RED}✗ Please edit env/.env and add your GEMINI_API_KEY${NC}"
    echo "  Get a key from: https://aistudio.google.com/app/apikey"
    exit 1
fi

# Source environment
source env/.env

# Check GEMINI_API_KEY
if [ -z "$GEMINI_API_KEY" ]; then
    echo -e "${RED}✗ GEMINI_API_KEY not set in env/.env${NC}"
    echo "  Get a key from: https://aistudio.google.com/app/apikey"
    exit 1
fi

echo -e "${GREEN}✓ GEMINI_API_KEY loaded${NC}"

# Check Neo4j
echo -n "Checking Neo4j... "
if docker ps | grep -q syschecker-neo4j; then
    echo -e "${GREEN}✓ Running${NC}"
else
    echo -e "${YELLOW}⚠ Not running${NC}"
    echo "Starting Neo4j..."
    docker run -d --name syschecker-neo4j \
        -p 7474:7474 -p 7687:7687 \
        -e NEO4J_AUTH=neo4j/${NEO4J_PASSWORD} \
        neo4j:latest > /dev/null 2>&1
    echo -e "${GREEN}✓ Neo4j started${NC}"
    echo "Waiting for Neo4j to initialize..."
    sleep 5
fi

# Check if server binary exists
SERVER_PATH="./syschecker-mcp"
if [ ! -f "$SERVER_PATH" ]; then
    echo -e "${YELLOW}⚠ Server binary not found${NC}"
    echo "Building MCP server..."
    go build -o syschecker-mcp ../../cmd/mcp
    echo -e "${GREEN}✓ Server built${NC}"
fi

# Check if chatbot binary exists
CHATBOT_PATH="./chatbot"
if [ ! -f "$CHATBOT_PATH" ]; then
    echo -e "${YELLOW}⚠ Chatbot binary not found${NC}"
    echo "Building chatbot..."
    go build -o chatbot ../../cmd/chatbot
    echo -e "${GREEN}✓ Chatbot built${NC}"
fi

# Check if DuckDB exists
if [ -f "../../syschecker.db" ]; then
    echo -e "${GREEN}✓ DuckDB found${NC}"
else
    echo -e "${YELLOW}⚠ DuckDB not found - will be created on first run${NC}"
fi

echo
echo -e "${GREEN}✓ All checks passed!${NC}"
echo -e "${BLUE}Starting chatbot...${NC}"
echo

# Run the chatbot
./chatbot
