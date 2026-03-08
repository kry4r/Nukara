package admin

import (
	"encoding/json"
	"net/http"
	"sort"
	"strings"
	"time"

	"nukara/backend/internal/store"
)

type adminUserItem struct {
	UserID    string `json:"user_id"`
	Email     string `json:"email"`
	Nickname  string `json:"nickname"`
	CreatedAt string `json:"created_at,omitempty"`
}

type adminBotItem struct {
	BotID     string `json:"bot_id"`
	Name      string `json:"name"`
	AvatarURL string `json:"avatar_url,omitempty"`
	UpdatedAt string `json:"updated_at,omitempty"`
	CreatedAt string `json:"created_at,omitempty"`
}

type memoryGraphNode struct {
	ID         string    `json:"id"`
	Type       string    `json:"type"`
	Label      string    `json:"label"`
	Content    string    `json:"content,omitempty"`
	Owner      string    `json:"owner,omitempty"`
	Importance int       `json:"importance,omitempty"`
	Topics     []string  `json:"topics,omitempty"`
	OccurredAt time.Time `json:"occurred_at,omitempty"`
}

type memoryGraphEdge struct {
	ID     string `json:"id"`
	Source string `json:"source"`
	Target string `json:"target"`
	Type   string `json:"type"`
}

type memoryGraphSummary struct {
	MemoryCount int    `json:"memory_count"`
	TopicCount  int    `json:"topic_count"`
	GraphSource string `json:"graph_source"`
}

type memoryGraphResponse struct {
	Nodes   []memoryGraphNode  `json:"nodes"`
	Edges   []memoryGraphEdge  `json:"edges"`
	Summary memoryGraphSummary `json:"summary"`
}

func buildMemoryGraphResponse(items []store.MemoryItem) memoryGraphResponse {
	nodes := make([]memoryGraphNode, 0, len(items)*2)
	edges := make([]memoryGraphEdge, 0, len(items)*2)
	topicSet := map[string]struct{}{}
	for _, item := range items {
		label := strings.TrimSpace(item.Content)
		if len([]rune(label)) > 24 {
			runes := []rune(label)
			label = string(runes[:24]) + "…"
		}
		nodes = append(nodes, memoryGraphNode{
			ID:         item.ID,
			Type:       "memory",
			Label:      label,
			Content:    item.Content,
			Owner:      item.Owner,
			Importance: item.Importance,
			Topics:     append([]string(nil), item.Topics...),
			OccurredAt: item.OccurredAt,
		})
		for _, topic := range item.Topics {
			topic = strings.TrimSpace(topic)
			if topic == "" {
				continue
			}
			topicID := "topic-" + topic
			if _, ok := topicSet[topicID]; !ok {
				topicSet[topicID] = struct{}{}
				nodes = append(nodes, memoryGraphNode{
					ID:    topicID,
					Type:  "topic",
					Label: topic,
				})
			}
			edges = append(edges, memoryGraphEdge{
				ID:     item.ID + "-" + topicID,
				Source: item.ID,
				Target: topicID,
				Type:   "memory_topic",
			})
		}
	}
	sort.Slice(nodes, func(i, j int) bool {
		if nodes[i].Type != nodes[j].Type {
			return nodes[i].Type < nodes[j].Type
		}
		return nodes[i].ID < nodes[j].ID
	})
	return memoryGraphResponse{
		Nodes: nodes,
		Edges: edges,
		Summary: memoryGraphSummary{
			MemoryCount: len(items),
			TopicCount:  len(topicSet),
			GraphSource: "store",
		},
	}
}

func (s *Server) handleAdminUsers(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.db == nil {
		http.Error(w, "Database is not configured", http.StatusServiceUnavailable)
		return
	}
	query := strings.TrimSpace(r.URL.Query().Get("q"))
	limit := parseBoundedInt(r.URL.Query().Get("limit"), 50, 1, 200)
	offset := parseBoundedInt(r.URL.Query().Get("offset"), 0, 0, 100000)
	rows, err := s.db.Query(`
		SELECT u.id::text, u.email, u.nickname, u.created_at
		FROM users u
		WHERE (
			$1 = ''
			OR u.email ILIKE '%' || $1 || '%'
			OR u.nickname ILIKE '%' || $1 || '%'
			OR u.id::text ILIKE '%' || $1 || '%'
		)
		ORDER BY u.created_at DESC
		LIMIT $2 OFFSET $3
	`, query, limit, offset)
	if err != nil {
		http.Error(w, "Failed to list users: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()
	items := make([]adminUserItem, 0, limit)
	for rows.Next() {
		var item adminUserItem
		var createdAt time.Time
		if err := rows.Scan(&item.UserID, &item.Email, &item.Nickname, &createdAt); err != nil {
			http.Error(w, "Failed to scan users: "+err.Error(), http.StatusInternalServerError)
			return
		}
		item.CreatedAt = createdAt.UTC().Format(time.RFC3339)
		items = append(items, item)
	}
	var total int
	if err := s.db.QueryRow(`
		SELECT COUNT(*)
		FROM users u
		WHERE (
			$1 = ''
			OR u.email ILIKE '%' || $1 || '%'
			OR u.nickname ILIKE '%' || $1 || '%'
			OR u.id::text ILIKE '%' || $1 || '%'
		)
	`, query).Scan(&total); err != nil {
		http.Error(w, "Failed to count users: "+err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"items": items, "total": total})
}

func (s *Server) handleAdminUserGraphRoutes(w http.ResponseWriter, r *http.Request) {
	if s.db == nil {
		http.Error(w, "Database is not configured", http.StatusServiceUnavailable)
		return
	}
	path := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/admin/users/"), "/")
	parts := strings.Split(path, "/")
	if len(parts) == 2 && parts[1] == "bots" {
		s.handleAdminUserBots(w, r, parts[0])
		return
	}
	if len(parts) == 4 && parts[1] == "bots" && parts[3] == "memory-graph" {
		s.handleAdminMemoryGraph(w, r, parts[0], parts[2])
		return
	}
	http.NotFound(w, r)
}

func (s *Server) handleAdminUserBots(w http.ResponseWriter, r *http.Request, userID string) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	rows, err := s.db.Query(`
		SELECT id::text, name, COALESCE(avatar_url, ''), created_at, updated_at
		FROM bots
		WHERE user_id = $1
		ORDER BY updated_at DESC, created_at DESC
	`, strings.TrimSpace(userID))
	if err != nil {
		http.Error(w, "Failed to list bots: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()
	items := make([]adminBotItem, 0, 16)
	for rows.Next() {
		var item adminBotItem
		var createdAt time.Time
		var updatedAt time.Time
		if err := rows.Scan(&item.BotID, &item.Name, &item.AvatarURL, &createdAt, &updatedAt); err != nil {
			http.Error(w, "Failed to scan bots: "+err.Error(), http.StatusInternalServerError)
			return
		}
		item.CreatedAt = createdAt.UTC().Format(time.RFC3339)
		item.UpdatedAt = updatedAt.UTC().Format(time.RFC3339)
		items = append(items, item)
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"items": items})
}

func (s *Server) handleAdminMemoryGraph(w http.ResponseWriter, r *http.Request, userID, botID string) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	rows, err := s.db.Query(`
		SELECT id, user_id, bot_id, kind, owner, content, importance, occurred_at, status, topics, created_at, updated_at
		FROM memory_items
		WHERE user_id = $1 AND bot_id = $2 AND status = 'active'
		ORDER BY importance DESC, occurred_at DESC
		LIMIT 200
	`, strings.TrimSpace(userID), strings.TrimSpace(botID))
	if err != nil {
		http.Error(w, "Failed to load memory graph: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()
	items := make([]store.MemoryItem, 0, 64)
	for rows.Next() {
		var item store.MemoryItem
		var topicsRaw []byte
		if err := rows.Scan(&item.ID, &item.UserID, &item.BotID, &item.Kind, &item.Owner, &item.Content, &item.Importance, &item.OccurredAt, &item.Status, &topicsRaw, &item.CreatedAt, &item.UpdatedAt); err != nil {
			http.Error(w, "Failed to scan memory graph: "+err.Error(), http.StatusInternalServerError)
			return
		}
		_ = json.Unmarshal(topicsRaw, &item.Topics)
		items = append(items, item)
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(buildMemoryGraphResponse(items))
}
