package neo4jadapter

import (
	"context"
	"fmt"
	"strings"

	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
)

type BoltStore struct {
	driver   neo4j.DriverWithContext
	database string
}

func NewBoltStore(uri, username, password, database string) (*BoltStore, error) {
	uri = strings.TrimSpace(uri)
	if uri == "" {
		return nil, fmt.Errorf("neo4j bolt url is required")
	}
	driver, err := neo4j.NewDriverWithContext(uri, neo4j.BasicAuth(strings.TrimSpace(username), strings.TrimSpace(password), ""))
	if err != nil {
		return nil, err
	}
	store := &BoltStore{driver: driver, database: strings.TrimSpace(database)}
	if store.database == "" {
		store.database = "neo4j"
	}
	return store, nil
}

func (s *BoltStore) Close(ctx context.Context) error {
	if s == nil || s.driver == nil {
		return nil
	}
	return s.driver.Close(ctx)
}

func (s *BoltStore) Ping(ctx context.Context) error {
	if s == nil || s.driver == nil {
		return fmt.Errorf("neo4j driver is not configured")
	}
	return s.driver.VerifyConnectivity(ctx)
}

func (s *BoltStore) ExpandTopics(ctx context.Context, topics []string, limit int) ([]Topic, error) {
	if s == nil || s.driver == nil {
		return nil, fmt.Errorf("neo4j driver is not configured")
	}
	if len(topics) == 0 {
		return nil, nil
	}
	if limit <= 0 {
		limit = defaultTopicLimit
	}
	session := s.driver.NewSession(ctx, neo4j.SessionConfig{DatabaseName: s.database})
	defer session.Close(ctx)

	query := `
UNWIND $topics AS seedName
MATCH (seed:Topic {name: seedName})
MATCH (seed)-[rel:RELATED]-(candidate:Topic)
WHERE NOT candidate.name IN $topics
WITH candidate.name AS name, sum(coalesce(rel.weight, 1.0)) AS weight
ORDER BY weight DESC, name ASC
LIMIT $limit
RETURN name, weight
`

	resultAny, err := session.ExecuteRead(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		result, err := tx.Run(ctx, query, map[string]any{"topics": topics, "limit": limit})
		if err != nil {
			return nil, err
		}
		out := make([]Topic, 0, limit)
		for result.Next(ctx) {
			record := result.Record()
			name, _ := record.Get("name")
			weight, _ := record.Get("weight")
			out = append(out, Topic{Name: strings.TrimSpace(fmt.Sprintf("%v", name)), Weight: toFloat64(weight)})
		}
		if err := result.Err(); err != nil {
			return nil, err
		}
		return out, nil
	})
	if err != nil {
		return nil, err
	}
	if resultAny == nil {
		return nil, nil
	}
	result, _ := resultAny.([]Topic)
	return result, nil
}

func (s *BoltStore) UpsertMemoryTopics(ctx context.Context, memoryID string, topics []string) error {
	if s == nil || s.driver == nil {
		return fmt.Errorf("neo4j driver is not configured")
	}
	memoryID = strings.TrimSpace(memoryID)
	if memoryID == "" {
		return fmt.Errorf("memory id is required")
	}
	topics = normalizeTopics(topics)

	session := s.driver.NewSession(ctx, neo4j.SessionConfig{DatabaseName: s.database})
	defer session.Close(ctx)

	_, err := session.ExecuteWrite(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		clearQuery := `
MERGE (m:Memory {id: $memory_id})
WITH m
OPTIONAL MATCH (m)-[rel:HAS_TOPIC]->(:Topic)
DELETE rel
RETURN m.id AS id
`
		if _, err := tx.Run(ctx, clearQuery, map[string]any{"memory_id": memoryID}); err != nil {
			return nil, err
		}
		if len(topics) == 0 {
			return nil, nil
		}
		linkQuery := `
MERGE (m:Memory {id: $memory_id})
WITH m
UNWIND $topics AS topicName
MERGE (t:Topic {name: topicName})
MERGE (m)-[:HAS_TOPIC]->(t)
WITH collect(DISTINCT t) AS topicNodes
UNWIND range(0, size(topicNodes) - 1) AS i
UNWIND range(i + 1, size(topicNodes) - 1) AS j
WITH topicNodes[i] AS leftTopic, topicNodes[j] AS rightTopic
WHERE leftTopic.name <> rightTopic.name
MERGE (leftTopic)-[rel:RELATED]-(rightTopic)
ON CREATE SET rel.weight = 1.0
ON MATCH SET rel.weight = coalesce(rel.weight, 0.0) + 1.0
RETURN count(*) AS pairs
`
		result, err := tx.Run(ctx, linkQuery, map[string]any{"memory_id": memoryID, "topics": topics})
		if err != nil {
			return nil, err
		}
		for result.Next(ctx) {
		}
		return nil, result.Err()
	})
	return err
}

func toFloat64(value any) float64 {
	switch typed := value.(type) {
	case float64:
		return typed
	case float32:
		return float64(typed)
	case int64:
		return float64(typed)
	case int32:
		return float64(typed)
	case int:
		return float64(typed)
	default:
		return 0
	}
}
