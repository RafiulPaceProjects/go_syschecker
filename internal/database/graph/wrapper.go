package graph

import (
	"syschecker/internal/database/relational"
)

// Node represents a vertex in the relational graph.
type Node struct {
	ID         string
	Properties map[string]interface{}
}

// Edge represents a relationship between two nodes.
type Edge struct {
	ID         string
	FromID     string
	ToID       string
	Properties map[string]interface{}
}

// RelationalGraphWrapper implements a thin graph interface backed by DuckDB.
type RelationalGraphWrapper struct {
	relational *relational.DuckDBClient
}

// NewRelationalGraphWrapper returns a wrapper that can traverse the relational graph tables.
func NewRelationalGraphWrapper(rel *relational.DuckDBClient) *RelationalGraphWrapper {
	return &RelationalGraphWrapper{relational: rel}
}

// GetNeighbors queries DuckDB for neighbors of the provided node ID.
func (w *RelationalGraphWrapper) GetNeighbors(nodeID string) ([]Node, error) {
	// Placeholder: expand this function once tables are defined.
	return nil, nil
}
