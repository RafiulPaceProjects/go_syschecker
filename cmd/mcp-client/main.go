package main

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func main() {
	flag.Parse()
	args := flag.Args()

	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "Usage: mcp-client <server-command> [<args>]")
		fmt.Fprintln(os.Stderr, "Example: mcp-client ./syschecker-mcp")
		os.Exit(2)
	}

	ctx := context.Background()

	// Start the server as a subprocess
	cmd := exec.Command(args[0], args[1:]...)
	transport := &mcp.CommandTransport{Command: cmd}

	// Create MCP client
	client := mcp.NewClient(&mcp.Implementation{
		Name:    "syschecker-client",
		Version: "1.0.0",
	}, nil)

	// Connect to the server
	session, err := client.Connect(ctx, transport, nil)
	if err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}
	defer session.Close()

	fmt.Println("Connected to SysChecker MCP Server!")
	fmt.Println("Available commands:")
	fmt.Println("  /tools        - List available tools")
	fmt.Println("  /metrics      - Get fast realtime metrics")
	fmt.Println("  /metrics-slow - Get detailed realtime metrics")
	fmt.Println("  /history [hostname] [limit] - Get historical snapshots")
	fmt.Println("  /graph <cypher> - Execute Cypher query")
	fmt.Println("  /exit         - Exit the client")
	fmt.Println("  <question>    - Ask a question using GraphRAG")
	fmt.Println()

	// Interactive REPL
	scanner := bufio.NewScanner(os.Stdin)
	for {
		fmt.Print("> ")
		if !scanner.Scan() {
			break
		}
		input := strings.TrimSpace(scanner.Text())
		if input == "" {
			continue
		}

		switch {
		case input == "/exit":
			fmt.Println("Goodbye!")
			return

		case input == "/tools":
			listTools(ctx, session)

		case input == "/metrics":
			callTool(ctx, session, "get_realtime_metrics", map[string]interface{}{
				"metric_type": "fast",
			})

		case input == "/metrics-slow":
			callTool(ctx, session, "get_realtime_metrics", map[string]interface{}{
				"metric_type": "slow",
			})

		case strings.HasPrefix(input, "/history"):
			parts := strings.Fields(input)
			args := map[string]interface{}{}
			if len(parts) > 1 {
				args["hostname"] = parts[1]
			}
			if len(parts) > 2 {
				args["limit"] = parts[2]
			}
			callTool(ctx, session, "get_historical_snapshots", args)

		case strings.HasPrefix(input, "/graph "):
			cypher := strings.TrimPrefix(input, "/graph ")
			callTool(ctx, session, "query_graph", map[string]interface{}{
				"cypher": cypher,
			})

		default:
			// Treat as a question for ask_syschecker
			callTool(ctx, session, "ask_syschecker", map[string]interface{}{
				"question": input,
			})
		}
	}

	if err := scanner.Err(); err != nil {
		log.Printf("Scanner error: %v", err)
	}
}

func listTools(ctx context.Context, session *mcp.ClientSession) {
	fmt.Println("Available Tools:")
	for tool, err := range session.Tools(ctx, nil) {
		if err != nil {
			log.Printf("Error listing tools: %v", err)
			return
		}
		fmt.Printf("  - %s: %s\n", tool.Name, tool.Description)
	}
	fmt.Println()
}

func callTool(ctx context.Context, session *mcp.ClientSession, toolName string, args map[string]interface{}) {
	result, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      toolName,
		Arguments: args,
	})
	if err != nil {
		log.Printf("Error calling tool: %v", err)
		return
	}

	printResult(result)
}

func printResult(result *mcp.CallToolResult) {
	if result.IsError {
		fmt.Printf("❌ Error: ")
	} else {
		fmt.Printf("✅ Result: ")
	}

	// Try to pretty-print the content
	for _, content := range result.Content {
		switch v := content.(type) {
		case *mcp.TextContent:
			fmt.Println(v.Text)
		default:
			// Try JSON marshaling for other types
			jsonData, err := json.MarshalIndent(content, "", "  ")
			if err != nil {
				fmt.Printf("%+v\n", content)
			} else {
				fmt.Println(string(jsonData))
			}
		}
	}
	fmt.Println()
}
