package memory

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRecallBuilderQdrantThenNeo4jExpansion(t *testing.T) {
	qdrant := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && strings.Contains(r.URL.Path, "/search") {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"result": []map[string]any{
					{
						"id": "mem-1",
						"payload": map[string]any{
							"user_id": "u1",
							"bot_id":  "b1",
							"content": "用户喜欢咖啡",
							"topics":  []string{"咖啡"},
						},
					},
				},
			})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer qdrant.Close()

	neo4j := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"topics": []map[string]any{
				{"name": "咖啡豆", "weight": 0.8},
			},
		})
	}))
	defer neo4j.Close()

	builder := NewRecallBuilder(RecallDeps{
		Qdrant: NewQdrantClient(qdrant.URL, "test-key", "agent_memory_v1", qdrant.Client()),
		Neo4j:  NewNeo4jClient(neo4j.URL, "neo4j", "pwd", neo4j.Client()),
	})

	items, err := builder.Build(context.Background(), RecallInput{
		UserID:     "u1",
		BotID:      "b1",
		QueryText:  "你记得我喜欢什么吗",
		Limit:      4,
		WithExpand: true,
	})
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}
	if len(items) < 2 {
		t.Fatalf("expected qdrant + neo4j expanded items, got=%d", len(items))
	}
}

