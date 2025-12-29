package main

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func main() {
	// Load environment variables
	loadEnvFile("env/.env")
	loadEnvFile("ui/Testing/env/.env")

	if os.Getenv("GEMINI_API_KEY") == "" {
		log.Fatal("❌ GEMINI_API_KEY not set in env/.env")
	}

	fmt.Println("🧪 Testing MCP Server and Tool Calling")
	fmt.Println("=======================================")
	fmt.Println()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Build path to the MCP server binary
	serverPath := findServerBinary()
	if serverPath == "" {
		log.Fatal("❌ MCP server binary not found. Run: go build -o syschecker-mcp ./cmd/mcp")
	}
	fmt.Println("✅ Test 1: MCP server binary found")

	// Start the MCP server
	cmd := exec.Command(serverPath)
	cmd.Env = append(os.Environ(),
		"GEMINI_API_KEY="+os.Getenv("GEMINI_API_KEY"),
		"GEMINI_MODEL="+os.Getenv("GEMINI_MODEL"),
		"NEO4J_URI="+os.Getenv("NEO4J_URI"),
		"NEO4J_PASSWORD="+os.Getenv("NEO4J_PASSWORD"),
		"DUCKDB_PATH="+os.Getenv("DUCKDB_PATH"),
	)
	cmd.Stderr = os.Stderr
	transport := &mcp.CommandTransport{Command: cmd}

	// Create client
	client := mcp.NewClient(&mcp.Implementation{
		Name:    "test-client",
		Version: "1.0.0",
	}, nil)

	// Connect to server
	session, err := client.Connect(ctx, transport, nil)
	if err != nil {
		log.Fatalf("❌ Failed to connect to MCP server: %v", err)
	}
	defer session.Close()
	fmt.Println("✅ Test 2: Connected to MCP server")

	// List available tools
	fmt.Println("\n✓ Test 3: Listing available tools")
	listResult, err := session.ListTools(ctx, nil)
	if err != nil {
		log.Fatalf("❌ Failed to list tools: %v", err)
	}
	fmt.Printf("  Found %d tools:\n", len(listResult.Tools))
	for _, tool := range listResult.Tools {
		fmt.Printf("  - %s: %s\n", tool.Name, tool.Description)
	}

	// Test 1: get_realtime_metrics
	fmt.Println("\n✓ Test 4: Testing get_realtime_metrics tool")
	metricsResult, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name: "get_realtime_metrics",
		Arguments: map[string]interface{}{
			"metric_type": "fast",
		},
	})
	if err != nil {
		fmt.Printf("  ❌ Metrics tool failed: %v\n", err)
	} else {
		fmt.Println("  ✅ Metrics tool called successfully")
		if len(metricsResult.Content) > 0 {
			fmt.Println("  ✅ Received metrics data:")
			for i, content := range metricsResult.Content {
				if i >= 3 {
					fmt.Printf("  ... and %d more content items\n", len(metricsResult.Content)-i)
					break
				}
				switch v := content.(type) {
				case *mcp.TextContent:
					preview := v.Text
					if len(preview) > 200 {
						preview = preview[:200] + "..."
					}
					fmt.Printf("    %s\n", preview)
				default:
					fmt.Printf("    [%T]\n", content)
				}
			}
		}
	}

	// Test 2: ask_syschecker (with timeout)
	fmt.Println("\n✓ Test 5: Testing ask_syschecker tool")
	askCtx, askCancel := context.WithTimeout(ctx, 15*time.Second)
	defer askCancel()

	askResult, err := session.CallTool(askCtx, &mcp.CallToolParams{
		Name: "ask_syschecker",
		Arguments: map[string]interface{}{
			"question": "What is my system's hostname?",
		},
	})
	if err != nil {
		if askCtx.Err() == context.DeadlineExceeded {
			fmt.Println("  ⚠️  Ask tool timed out (may need Neo4j to be running)")
		} else {
			fmt.Printf("  ❌ Ask tool failed: %v\n", err)
		}
	} else {
		fmt.Println("  ✅ Ask tool called successfully")
		if len(askResult.Content) > 0 {
			fmt.Println("  ✅ Received answer:")
			for _, content := range askResult.Content {
				switch v := content.(type) {
				case *mcp.TextContent:
					fmt.Printf("    %s\n", v.Text)
				default:
					fmt.Printf("    [%T]\n", content)
				}
			}
		}
	}

	// Test 3: get_historical_snapshots
	fmt.Println("\n✓ Test 6: Testing get_historical_snapshots tool")
	snapshotsResult, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name: "get_historical_snapshots",
		Arguments: map[string]interface{}{
			"limit": 5,
		},
	})
	if err != nil {
		fmt.Printf("  ⚠️  Snapshots tool failed (may be empty database): %v\n", err)
	} else {
		fmt.Println("  ✅ Snapshots tool called successfully")
		if len(snapshotsResult.Content) > 0 {
			fmt.Printf("  ✅ Received %d content items\n", len(snapshotsResult.Content))
		}
	}

	fmt.Println("\n=======================================")
	fmt.Println("✅ All MCP tool calling tests complete!")
	fmt.Println("\n💡 To test interactively, run: go run ./cmd/chatbot")
}

func findServerBinary() string {
	candidates := []string{
		"./syschecker-mcp",
		"../../syschecker-mcp",
		"../../../syschecker-mcp",
	}
	for _, p := range candidates {
		if abs, err := filepath.Abs(p); err == nil {
			if _, err := os.Stat(abs); err == nil {
				return abs
			}
		}
	}
	return ""
}

func loadEnvFile(path string) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return
	}

	file, err := os.Open(absPath)
	if err != nil {
		return
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			value := strings.TrimSpace(parts[1])
			value = strings.Trim(value, `"'`)
			os.Setenv(key, value)
		}
	}
}
