package graph

import (
	"context"
	"fmt"

	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
)

// ExecuteCypher executes a raw Cypher query and returns the results.
func (c *Neo4jClient) ExecuteCypher(ctx context.Context, query string) ([]map[string]any, error) {
	session := c.driver.NewSession(ctx, neo4j.SessionConfig{DatabaseName: c.dbName})
	defer session.Close(ctx)

	result, err := session.ExecuteRead(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		res, err := tx.Run(ctx, query, nil)
		if err != nil {
			return nil, err
		}

		records, err := res.Collect(ctx)
		if err != nil {
			return nil, err
		}

		// Convert records to maps
		var results []map[string]any
		for _, record := range records {
			rowMap := make(map[string]any)
			for i, key := range record.Keys {
				rowMap[key] = convertNeo4jValue(record.Values[i])
			}
			results = append(results, rowMap)
		}

		return results, nil
	})
	if err != nil {
		return nil, fmt.Errorf("cypher execution failed: %w", err)
	}

	return result.([]map[string]any), nil
}

// convertNeo4jValue converts Neo4j types to Go native types.
func convertNeo4jValue(val any) any {
	switch v := val.(type) {
	case neo4j.Node:
		return map[string]any{
			"labels":     v.Labels,
			"properties": v.Props,
			"id":         v.ElementId,
		}
	case neo4j.Relationship:
		return map[string]any{
			"type":       v.Type,
			"properties": v.Props,
			"startNode":  v.StartElementId,
			"endNode":    v.EndElementId,
		}
	case []any:
		result := make([]any, len(v))
		for i, item := range v {
			result[i] = convertNeo4jValue(item)
		}
		return result
	case map[string]any:
		result := make(map[string]any)
		for k, v := range v {
			result[k] = convertNeo4jValue(v)
		}
		return result
	default:
		return v
	}
}
