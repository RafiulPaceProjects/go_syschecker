#!/bin/bash
set -e

echo "========================================="
echo "SysChecker MCP Server Setup"
echo "========================================="

# Check for required tools
echo "Checking prerequisites..."

if ! command -v go &> /dev/null; then
    echo "❌ Go is not installed. Please install Go 1.21 or later."
    exit 1
fi
echo "✅ Go installed: $(go version)"

if ! command -v docker &> /dev/null; then
    echo "⚠️  Docker is not installed. You'll need Docker to run Neo4j."
fi

# Install Go dependencies
echo ""
echo "Installing Go dependencies..."
go get github.com/modelcontextprotocol/go-sdk/mcp@latest
go get github.com/google/generative-ai-go/genai@latest
go get google.golang.org/api/option@latest
go get github.com/neo4j/neo4j-go-driver/v5@latest

echo "✅ Dependencies installed"

# Start Neo4j if Docker is available
echo ""
read -p "Do you want to start Neo4j in Docker? (y/n) " -n 1 -r
echo
if [[ $REPLY =~ ^[Yy]$ ]]; then
    echo "Starting Neo4j..."
    docker run -d \
      --name syschecker-neo4j \
      -p 7474:7474 -p 7687:7687 \
      -e NEO4J_AUTH=neo4j/password \
      neo4j:latest
    
    echo "✅ Neo4j started on ports 7474 (HTTP) and 7687 (Bolt)"
    echo "   Browser: http://localhost:7474"
    echo "   Username: neo4j"
    echo "   Password: password"
fi

# Check for Gemini API key
echo ""
if [ -z "$GEMINI_API_KEY" ]; then
    echo "⚠️  GEMINI_API_KEY environment variable is not set"
    echo "   Get your API key from: https://aistudio.google.com/app/apikey"
    echo "   Then run: export GEMINI_API_KEY='your-key-here'"
else
    echo "✅ GEMINI_API_KEY is set"
fi

# Build the MCP server
echo ""
echo "Building MCP server..."
go build -o syschecker-mcp ./cmd/mcp

if [ $? -eq 0 ]; then
    echo "✅ MCP server built successfully: ./syschecker-mcp"
else
    echo "❌ Build failed"
    exit 1
fi

# Print next steps
echo ""
echo "========================================="
echo "Setup Complete!"
echo "========================================="
echo ""
echo "Next steps:"
echo "1. Set environment variables:"
echo "   export GEMINI_API_KEY='your-api-key'"
echo "   export NEO4J_PASSWORD='password'"
echo ""
echo "2. Run the data worker to populate databases:"
echo "   go run main.go"
echo ""
echo "3. Start the MCP server:"
echo "   ./syschecker-mcp"
echo ""
echo "4. Configure Claude Desktop:"
echo "   Add to ~/Library/Application Support/Claude/claude_desktop_config.json"
echo "   (See internal/MCP server/README.md for configuration)"
echo ""
