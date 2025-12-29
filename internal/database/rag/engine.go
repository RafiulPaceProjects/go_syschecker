package rag

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"syschecker/internal/database/graph"

	"github.com/google/generative-ai-go/genai"
)

// ModelConfig defines configuration for a Gemini model.
type ModelConfig struct {
	Name        string
	Temperature float32
	TopP        float32
	TopK        int32
}

// AvailableModels defines the available Gemini models and their configurations.
var AvailableModels = map[string]ModelConfig{
	"flash": {
		Name:        "gemini-flash-latest",
		Temperature: 0.7,
		TopP:        0.95,
		TopK:        40,
	},
	"pro": {
		Name:        "gemini-pro-latest",
		Temperature: 0.7,
		TopP:        0.95,
		TopK:        40,
	},
	"flash-2": {
		Name:        "gemini-2.0-flash",
		Temperature: 0.7,
		TopP:        0.95,
		TopK:        40,
	},
	"experimental": {
		Name:        "gemini-2.0-flash-exp",
		Temperature: 0.7,
		TopP:        0.95,
		TopK:        40,
	},
}

// GraphRAGEngine handles retrieval augmented generation using graph structures.
type GraphRAGEngine struct {
	neo4jClient  graph.GraphClient
	geminiClient *genai.Client
	modelName    string
	config       ModelConfig
}

// NewGraphRAGEngine constructs a new engine backed by the provided graph wrapper.
func NewGraphRAGEngine(neo4j graph.GraphClient, gemini *genai.Client, modelKey string) *GraphRAGEngine {
	if modelKey == "" {
		modelKey = "pro" // Default to pro for best quality
	}

	config, ok := AvailableModels[modelKey]
	if !ok {
		// Fallback to pro if unknown model
		config = AvailableModels["pro"]
	}

	return &GraphRAGEngine{
		neo4jClient:  neo4j,
		geminiClient: gemini,
		modelName:    config.Name,
		config:       config,
	}
}

// getModel returns a configured GenerativeModel instance.
func (e *GraphRAGEngine) getModel() *genai.GenerativeModel {
	model := e.geminiClient.GenerativeModel(e.modelName)
	model.SetTemperature(e.config.Temperature)
	model.SetTopP(e.config.TopP)
	model.SetTopK(e.config.TopK)
	return model
}

// Query performs a GraphRAG search over the owned graph.
func (e *GraphRAGEngine) Query(ctx context.Context, question string) (string, error) {
	// Step 1: Generate Cypher query using Gemini
	cypher, err := e.generateCypher(ctx, question)
	if err != nil {
		return "", fmt.Errorf("failed to generate cypher: %w", err)
	}

	// Step 2: Execute query on Neo4j to retrieve relevant subgraph
	graphData, err := e.neo4jClient.ExecuteCypher(ctx, cypher)
	if err != nil || len(graphData) == 0 {
		// If query fails or returns empty, try a comprehensive fallback
		// This gets the latest snapshot with all related data
		cypher = `
			MATCH (h:Host)-[:HAS_SNAPSHOT]->(s:Snapshot)
			OPTIONAL MATCH (s)-[:TRIGGERED]->(f:Flag)
			OPTIONAL MATCH (s)-[:HAS_CAUSE]->(c:Cause)
			OPTIONAL MATCH (s)-[:OBSERVED_CONTAINER]->(cont:Container)
			WITH h, s, 
				 collect(DISTINCT f.name) as flags,
				 collect(DISTINCT {cause: c.primary_cause, explanation: c.explanation}) as causes,
				 collect(DISTINCT {name: cont.name, running: cont.running}) as containers
			RETURN h.hostname as host,
				   s.cpu_usage_pct as cpu_pct,
				   s.ram_usage_pct as ram_pct,
				   s.disk_usage_pct as disk_pct,
				   s.severity_level as severity,
				   s.collected_at as timestamp,
				   flags,
				   causes,
				   containers
			ORDER BY s.collected_at DESC
			LIMIT 5
		`
		graphData, err = e.neo4jClient.ExecuteCypher(ctx, cypher)
		if err != nil {
			return "", fmt.Errorf("failed to execute graph query: %w", err)
		}
	}

	// Step 3: Synthesize answer using Gemini with the graph context
	answer, err := e.synthesizeAnswer(ctx, question, graphData)
	if err != nil {
		return "", fmt.Errorf("failed to synthesize answer: %w", err)
	}

	return answer, nil
}

// generateCypher uses Gemini to convert a natural language question into a Cypher query.
func (e *GraphRAGEngine) generateCypher(ctx context.Context, question string) (string, error) {
	model := e.getModel()

	prompt := fmt.Sprintf(`You are a Neo4j Cypher query expert. Convert the following question into a Cypher query for a system monitoring graph database.

Graph Schema:
- Nodes: Host, Snapshot, Flag, Cause, DiskDevice, NetInterface, Container
- Relationships: 
  - (Host)-[:HAS_SNAPSHOT]->(Snapshot)
  - (Snapshot)-[:TRIGGERED]->(Flag)
  - (Snapshot)-[:HAS_CAUSE]->(Cause)
  - (Cause)-[:CAUSED_BY]->(DiskDevice|NetInterface|Container)
  - (Snapshot)-[:OBSERVED_DISK_IO]->(DiskDevice)
  - (Snapshot)-[:OBSERVED_INTERFACE]->(NetInterface)
  - (Snapshot)-[:OBSERVED_CONTAINER]->(Container)

Snapshot properties: snapshot_id, collected_at, cpu_usage_pct, ram_usage_pct, disk_usage_pct, severity_level, risk_score, primary_cause, explanation
Flag properties: name (e.g., "cpu_overloaded", "memory_pressure", "disk_space_critical")
Cause properties: primary_cause, entity_type, entity_key, explanation

Question: %s

Return ONLY the Cypher query, no explanation. Limit results to 10.`, question)

	resp, err := model.GenerateContent(ctx, genai.Text(prompt))
	if err != nil {
		return "", err
	}

	if len(resp.Candidates) == 0 || len(resp.Candidates[0].Content.Parts) == 0 {
		return "", fmt.Errorf("no response from Gemini")
	}

	cypher := fmt.Sprintf("%v", resp.Candidates[0].Content.Parts[0])
	// Clean up markdown code blocks if present
	cypher = cleanCypherQuery(cypher)

	return cypher, nil
}

// synthesizeAnswer uses Gemini to generate a natural language answer from graph data.
func (e *GraphRAGEngine) synthesizeAnswer(ctx context.Context, question string, graphData []map[string]any) (string, error) {
	model := e.getModel()

	// Convert graph data to JSON for context
	graphJSON, err := json.MarshalIndent(graphData, "", "  ")
	if err != nil {
		return "", err
	}

	prompt := fmt.Sprintf(`You are a system monitoring expert. Answer the following question based on the graph database results.

Question: %s

Graph Data (from Neo4j):
%s

Provide a clear, concise answer explaining:
1. What the data shows
2. Root causes if applicable
3. Severity and impact
4. Recommended actions if relevant

If the graph data is empty or insufficient, say so clearly.`, question, string(graphJSON))

	resp, err := model.GenerateContent(ctx, genai.Text(prompt))
	if err != nil {
		return "", err
	}

	if len(resp.Candidates) == 0 || len(resp.Candidates[0].Content.Parts) == 0 {
		return "Unable to generate response from the available data.", nil
	}

	answer := fmt.Sprintf("%v", resp.Candidates[0].Content.Parts[0])
	return answer, nil
}

// cleanCypherQuery removes markdown code blocks from Cypher queries.
func cleanCypherQuery(query string) string {
	// Remove ```cypher and ``` markers
	query = strings.TrimSpace(query)
	query = strings.TrimPrefix(query, "```cypher")
	query = strings.TrimPrefix(query, "```")
	query = strings.TrimSuffix(query, "```")
	return strings.TrimSpace(query)
}
