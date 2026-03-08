package admin

import (
	"encoding/json"
	"fmt"
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
	Kind       string    `json:"kind,omitempty"`
	Status     string    `json:"status,omitempty"`
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

type memoryGraphRuntimeState struct {
	ActivityText    string    `json:"activity_text"`
	BasisTags       []string  `json:"basis_tags,omitempty"`
	SourceMemoryIDs []string  `json:"source_memory_ids,omitempty"`
	UpdatedAt       time.Time `json:"updated_at,omitempty"`
}

type memoryGraphMemoryItem struct {
	ID         string    `json:"id"`
	Kind       string    `json:"kind"`
	Owner      string    `json:"owner,omitempty"`
	Content    string    `json:"content"`
	Importance int       `json:"importance,omitempty"`
	Status     string    `json:"status,omitempty"`
	Topics     []string  `json:"topics,omitempty"`
	OccurredAt time.Time `json:"occurred_at,omitempty"`
	UpdatedAt  time.Time `json:"updated_at,omitempty"`
}

type memoryGraphPersonaChange struct {
	ID            string    `json:"id"`
	Field         string    `json:"field"`
	ChangeType    string    `json:"change_type"`
	ProposedValue string    `json:"proposed_value"`
	SummaryText   string    `json:"summary_text,omitempty"`
	Risk          string    `json:"risk,omitempty"`
	Status        string    `json:"status"`
	ReviewerNote  string    `json:"reviewer_note,omitempty"`
	SourceTurnID  string    `json:"source_turn_id,omitempty"`
	CreatedAt     time.Time `json:"created_at,omitempty"`
	UpdatedAt     time.Time `json:"updated_at,omitempty"`
}

type memoryGraphContext struct {
	KindFilter            string
	StatusFilter          string
	RuntimeState          *store.BotRuntimeState
	RecentImpressions     []store.MemoryItem
	RecentChanges         []store.PersonaChangeEvent
	PendingPersonaChanges []store.PersonaChangeEvent
}

type memoryGraphSummary struct {
	MemoryCount  int    `json:"memory_count"`
	TopicCount   int    `json:"topic_count"`
	GraphSource  string `json:"graph_source"`
	KindFilter   string `json:"kind_filter,omitempty"`
	StatusFilter string `json:"status_filter,omitempty"`
}

type memoryGraphResponse struct {
	Nodes                 []memoryGraphNode          `json:"nodes"`
	Edges                 []memoryGraphEdge          `json:"edges"`
	Summary               memoryGraphSummary         `json:"summary"`
	RuntimeState          *memoryGraphRuntimeState   `json:"runtime_state,omitempty"`
	RecentImpressions     []memoryGraphMemoryItem    `json:"recent_impressions,omitempty"`
	RecentChanges         []memoryGraphPersonaChange `json:"recent_changes,omitempty"`
	PendingPersonaChanges []memoryGraphPersonaChange `json:"pending_persona_changes,omitempty"`
}

func buildMemoryGraphResponse(items []store.MemoryItem, ctx memoryGraphContext) memoryGraphResponse {
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
			Kind:       strings.TrimSpace(item.Kind),
			Status:     strings.TrimSpace(item.Status),
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

	response := memoryGraphResponse{
		Nodes: nodes,
		Edges: edges,
		Summary: memoryGraphSummary{
			MemoryCount:  len(items),
			TopicCount:   len(topicSet),
			GraphSource:  "store",
			KindFilter:   strings.TrimSpace(ctx.KindFilter),
			StatusFilter: strings.TrimSpace(ctx.StatusFilter),
		},
		RecentImpressions:     buildMemoryGraphMemoryItems(ctx.RecentImpressions),
		RecentChanges:         buildMemoryGraphPersonaChanges(ctx.RecentChanges),
		PendingPersonaChanges: buildMemoryGraphPersonaChanges(ctx.PendingPersonaChanges),
	}
	if ctx.RuntimeState != nil {
		response.RuntimeState = &memoryGraphRuntimeState{
			ActivityText:    strings.TrimSpace(ctx.RuntimeState.ActivityText),
			BasisTags:       append([]string(nil), ctx.RuntimeState.BasisTags...),
			SourceMemoryIDs: append([]string(nil), ctx.RuntimeState.SourceMemoryIDs...),
			UpdatedAt:       ctx.RuntimeState.UpdatedAt,
		}
	}
	return response
}

func buildMemoryGraphMemoryItems(items []store.MemoryItem) []memoryGraphMemoryItem {
	out := make([]memoryGraphMemoryItem, 0, len(items))
	for _, item := range items {
		content := strings.TrimSpace(item.Content)
		if content == "" {
			continue
		}
		out = append(out, memoryGraphMemoryItem{
			ID:         item.ID,
			Kind:       strings.TrimSpace(item.Kind),
			Owner:      strings.TrimSpace(item.Owner),
			Content:    content,
			Importance: item.Importance,
			Status:     strings.TrimSpace(item.Status),
			Topics:     append([]string(nil), item.Topics...),
			OccurredAt: item.OccurredAt,
			UpdatedAt:  item.UpdatedAt,
		})
	}
	return out
}

func buildMemoryGraphPersonaChanges(items []store.PersonaChangeEvent) []memoryGraphPersonaChange {
	out := make([]memoryGraphPersonaChange, 0, len(items))
	for _, item := range items {
		out = append(out, memoryGraphPersonaChange{
			ID:            item.ID,
			Field:         strings.TrimSpace(item.Field),
			ChangeType:    strings.TrimSpace(item.ChangeType),
			ProposedValue: strings.TrimSpace(item.ProposedValue),
			SummaryText:   summarizeMemoryGraphPersonaChange(item.Field, item.ProposedValue),
			Risk:          strings.TrimSpace(item.Risk),
			Status:        strings.TrimSpace(item.Status),
			ReviewerNote:  strings.TrimSpace(item.ReviewerNote),
			SourceTurnID:  strings.TrimSpace(item.SourceTurnID),
			CreatedAt:     item.CreatedAt,
			UpdatedAt:     item.UpdatedAt,
		})
	}
	return out
}

func summarizeMemoryGraphPersonaChange(field, value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	prefix := map[string]string{
		"identity":               "我更明确了自己的身份：",
		"personality":            "我更意识到自己的性格一面：",
		"expression_style":       "我最近说话会更偏向：",
		"life_context":           "我最近的生活状态更像是：",
		"taboos_and_preferences": "我更清楚自己的边界是：",
	}[strings.TrimSpace(field)]
	text := value
	if prefix != "" {
		text = prefix + value
	}
	return truncateMemoryGraphText(text, 42)
}

func truncateMemoryGraphText(value string, maxRunes int) string {
	value = strings.TrimSpace(value)
	if value == "" || maxRunes <= 0 {
		return value
	}
	runes := []rune(value)
	if len(runes) <= maxRunes {
		return value
	}
	return string(runes[:maxRunes]) + "…"
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

	kindFilter := strings.TrimSpace(r.URL.Query().Get("kind"))
	statusFilter := strings.TrimSpace(r.URL.Query().Get("status"))
	if strings.EqualFold(kindFilter, "all") {
		kindFilter = ""
	}
	if strings.EqualFold(statusFilter, "all") {
		statusFilter = ""
	}
	if statusFilter == "" {
		statusFilter = "active"
	}

	items, err := s.loadAdminMemoryItems(userID, botID, kindFilter, statusFilter, 200)
	if err != nil {
		http.Error(w, "Failed to load memory graph: "+err.Error(), http.StatusInternalServerError)
		return
	}
	runtimeState, err := s.loadAdminRuntimeState(userID, botID)
	if err != nil {
		http.Error(w, "Failed to load runtime state: "+err.Error(), http.StatusInternalServerError)
		return
	}
	recentImpressions, err := s.loadAdminRecentImpressions(userID, botID, 5)
	if err != nil {
		http.Error(w, "Failed to load recent impressions: "+err.Error(), http.StatusInternalServerError)
		return
	}
	recentChanges, err := s.loadAdminPersonaChangeEvents(userID, botID, "accepted", 6)
	if err != nil {
		http.Error(w, "Failed to load recent changes: "+err.Error(), http.StatusInternalServerError)
		return
	}
	pendingChanges, err := s.loadAdminPersonaChangeEvents(userID, botID, "pending", 6)
	if err != nil {
		http.Error(w, "Failed to load pending changes: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(buildMemoryGraphResponse(items, memoryGraphContext{
		KindFilter:            kindFilter,
		StatusFilter:          statusFilter,
		RuntimeState:          runtimeState,
		RecentImpressions:     recentImpressions,
		RecentChanges:         recentChanges,
		PendingPersonaChanges: pendingChanges,
	}))
}

func (s *Server) loadAdminRecentImpressions(userID, botID string, limit int) ([]store.MemoryItem, error) {
	items, err := s.loadAdminMemoryItems(userID, botID, "impression", "active", limit)
	if err != nil {
		return nil, err
	}
	if len(items) > 0 {
		return items, nil
	}
	return s.loadAdminMemoryItems(userID, botID, "user_fact", "active", limit)
}

func (s *Server) loadAdminMemoryItems(userID, botID, kind, status string, limit int) ([]store.MemoryItem, error) {
	if limit <= 0 {
		limit = 200
	}
	query := `
		SELECT id, user_id, bot_id, kind, owner, content, importance, occurred_at, status, topics, created_at, updated_at
		FROM memory_items
		WHERE user_id = $1 AND bot_id = $2`
	args := []any{strings.TrimSpace(userID), strings.TrimSpace(botID)}
	if kind = strings.TrimSpace(kind); kind != "" {
		query += fmt.Sprintf(" AND kind = $%d", len(args)+1)
		args = append(args, kind)
	}
	if status = strings.TrimSpace(status); status != "" {
		query += fmt.Sprintf(" AND status = $%d", len(args)+1)
		args = append(args, status)
	}
	query += fmt.Sprintf(" ORDER BY importance DESC, occurred_at DESC LIMIT $%d", len(args)+1)
	args = append(args, limit)

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]store.MemoryItem, 0, limit)
	for rows.Next() {
		var item store.MemoryItem
		var topicsRaw []byte
		if err := rows.Scan(&item.ID, &item.UserID, &item.BotID, &item.Kind, &item.Owner, &item.Content, &item.Importance, &item.OccurredAt, &item.Status, &topicsRaw, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, err
		}
		_ = json.Unmarshal(topicsRaw, &item.Topics)
		items = append(items, item)
	}
	return items, nil
}

func (s *Server) loadAdminRuntimeState(userID, botID string) (*store.BotRuntimeState, error) {
	var state store.BotRuntimeState
	var basisTagsRaw []byte
	var sourceMemoryIDsRaw []byte
	err := s.db.QueryRow(`
		SELECT user_id, bot_id, activity_text, basis_tags, source_memory_ids, updated_at
		FROM bot_runtime_states
		WHERE user_id=$1 AND bot_id=$2
	`, strings.TrimSpace(userID), strings.TrimSpace(botID)).Scan(
		&state.UserID, &state.BotID, &state.ActivityText, &basisTagsRaw, &sourceMemoryIDsRaw, &state.UpdatedAt,
	)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "no rows") {
			return nil, nil
		}
		return nil, err
	}
	_ = json.Unmarshal(basisTagsRaw, &state.BasisTags)
	_ = json.Unmarshal(sourceMemoryIDsRaw, &state.SourceMemoryIDs)
	return &state, nil
}

func (s *Server) loadAdminPersonaChangeEvents(userID, botID, status string, limit int) ([]store.PersonaChangeEvent, error) {
	if limit <= 0 {
		limit = 20
	}
	query := `
		SELECT id, user_id, bot_id, field, change_type, proposed_value, COALESCE(source_turn_id::text, ''), risk, status, COALESCE(reviewer_note, ''), created_at, updated_at
		FROM persona_change_events
		WHERE user_id=$1 AND bot_id=$2`
	args := []any{strings.TrimSpace(userID), strings.TrimSpace(botID)}
	if status = strings.TrimSpace(status); status != "" {
		query += fmt.Sprintf(" AND status=$%d", len(args)+1)
		args = append(args, status)
	}
	query += fmt.Sprintf(" ORDER BY created_at DESC, updated_at DESC LIMIT $%d", len(args)+1)
	args = append(args, limit)

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]store.PersonaChangeEvent, 0, limit)
	for rows.Next() {
		var item store.PersonaChangeEvent
		if err := rows.Scan(&item.ID, &item.UserID, &item.BotID, &item.Field, &item.ChangeType, &item.ProposedValue, &item.SourceTurnID, &item.Risk, &item.Status, &item.ReviewerNote, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, nil
}
