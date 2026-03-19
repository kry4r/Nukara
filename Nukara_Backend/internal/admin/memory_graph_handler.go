package admin

import (
	"database/sql"
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

func buildTemporalMemoryGraphResponse(nodesInput []store.TemporalMemoryNode, edgesInput []store.TemporalMemoryEdge, ctx memoryGraphContext) memoryGraphResponse {
	nodes := make([]memoryGraphNode, 0, len(nodesInput))
	edges := make([]memoryGraphEdge, 0, len(edgesInput))
	for _, node := range nodesInput {
		label := strings.TrimSpace(node.Title)
		if label == "" {
			label = strings.TrimSpace(node.Summary)
		}
		if len([]rune(label)) > 24 {
			runes := []rune(label)
			label = string(runes[:24]) + "…"
		}
		importance := int(node.Salience * 100)
		if importance <= 0 {
			importance = int(node.Confidence * 100)
		}
		if importance <= 0 {
			importance = int(node.Stability * 100)
		}
		nodes = append(nodes, memoryGraphNode{
			ID:         strings.TrimSpace(node.ID),
			Type:       "memory",
			Label:      label,
			Kind:       strings.TrimSpace(node.NodeType),
			Status:     strings.TrimSpace(node.Status),
			Content:    strings.TrimSpace(node.Summary),
			Importance: importance,
			OccurredAt: node.OccurredAt,
		})
	}
	for _, edge := range edgesInput {
		edges = append(edges, memoryGraphEdge{
			ID:     strings.TrimSpace(edge.ID),
			Source: strings.TrimSpace(edge.SourceID),
			Target: strings.TrimSpace(edge.TargetID),
			Type:   strings.TrimSpace(edge.EdgeType),
		})
	}
	sort.Slice(nodes, func(i, j int) bool {
		if nodes[i].OccurredAt.Equal(nodes[j].OccurredAt) {
			return nodes[i].ID < nodes[j].ID
		}
		return nodes[i].OccurredAt.After(nodes[j].OccurredAt)
	})
	response := memoryGraphResponse{
		Nodes: nodes,
		Edges: edges,
		Summary: memoryGraphSummary{
			MemoryCount:  len(nodes),
			TopicCount:   0,
			GraphSource:  "temporal_memory_graph",
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
	if len(parts) == 5 && parts[1] == "bots" && parts[3] == "memories" {
		s.handleAdminDeleteMemory(w, r, parts[0], parts[2], parts[4])
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

	temporalNodes, temporalEdges, temporalErr := s.loadAdminTemporalMemoryGraph(userID, botID, kindFilter, statusFilter, 200)
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

	ctx := memoryGraphContext{
		KindFilter:            kindFilter,
		StatusFilter:          statusFilter,
		RuntimeState:          runtimeState,
		RecentImpressions:     recentImpressions,
		RecentChanges:         recentChanges,
		PendingPersonaChanges: pendingChanges,
	}
	response := memoryGraphResponse{}
	if temporalErr == nil && len(temporalNodes) > 0 {
		response = buildTemporalMemoryGraphResponse(temporalNodes, temporalEdges, ctx)
	} else {
		items, fallbackErr := s.loadAdminMemoryItems(userID, botID, kindFilter, statusFilter, 200)
		if fallbackErr != nil {
			if temporalErr != nil {
				http.Error(w, "Failed to load memory graph: "+temporalErr.Error()+"; fallback failed: "+fallbackErr.Error(), http.StatusInternalServerError)
				return
			}
			http.Error(w, "Failed to load memory graph: "+fallbackErr.Error(), http.StatusInternalServerError)
			return
		}
		response = buildMemoryGraphResponse(items, ctx)
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(response)
}

func temporalNodeTypesForKind(kind string) []string {
	switch strings.TrimSpace(kind) {
	case "promise":
		return []string{"promise"}
	case "event":
		return []string{"episode"}
	case "self_fact":
		return []string{"self_model", "state_snapshot"}
	case "user_fact":
		return []string{"user_fact"}
	case "habit":
		return []string{"habit"}
	default:
		return nil
	}
}

func (s *Server) loadAdminTemporalMemoryGraph(userID, botID, kind, status string, limit int) ([]store.TemporalMemoryNode, []store.TemporalMemoryEdge, error) {
	nodes, err := s.loadAdminTemporalMemoryNodes(userID, botID, kind, status, limit)
	if err != nil || len(nodes) == 0 {
		return nodes, nil, err
	}
	nodeIDs := make([]string, 0, len(nodes))
	for _, node := range nodes {
		if id := strings.TrimSpace(node.ID); id != "" {
			nodeIDs = append(nodeIDs, id)
		}
	}
	edges, err := s.loadAdminTemporalMemoryEdges(nodeIDs, status, limit*3)
	return nodes, edges, err
}

func (s *Server) loadAdminTemporalMemoryNodes(userID, botID, kind, status string, limit int) ([]store.TemporalMemoryNode, error) {
	if limit <= 0 {
		limit = 200
	}
	query := `
		SELECT id::text, user_id::text, bot_id::text, COALESCE(session_id::text, ''), node_type, title, summary, body_json,
			salience, affect_weight, confidence, stability, status, occurred_at, observed_at,
			valid_from, valid_to, last_accessed_at, COALESCE(source_turn_id::text, ''), created_at, updated_at
		FROM memory_nodes
		WHERE user_id = $1 AND bot_id = $2`
	args := []any{strings.TrimSpace(userID), strings.TrimSpace(botID)}
	if status = strings.TrimSpace(status); status != "" {
		query += fmt.Sprintf(" AND status = $%d", len(args)+1)
		args = append(args, status)
	}
	nodeTypes := temporalNodeTypesForKind(kind)
	if len(nodeTypes) > 0 {
		placeholders := make([]string, 0, len(nodeTypes))
		for _, nodeType := range nodeTypes {
			args = append(args, nodeType)
			placeholders = append(placeholders, fmt.Sprintf("$%d", len(args)))
		}
		query += " AND node_type IN (" + strings.Join(placeholders, ", ") + ")"
	}
	query += fmt.Sprintf(" ORDER BY occurred_at DESC, updated_at DESC LIMIT $%d", len(args)+1)
	args = append(args, limit)

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]store.TemporalMemoryNode, 0, limit)
	for rows.Next() {
		var item store.TemporalMemoryNode
		var bodyRaw []byte
		var validTo sql.NullTime
		if err := rows.Scan(&item.ID, &item.UserID, &item.BotID, &item.SessionID, &item.NodeType, &item.Title, &item.Summary, &bodyRaw, &item.Salience, &item.AffectWeight, &item.Confidence, &item.Stability, &item.Status, &item.OccurredAt, &item.ObservedAt, &item.ValidFrom, &validTo, &item.LastAccessedAt, &item.SourceTurnID, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, err
		}
		item.BodyJSON = strings.TrimSpace(string(bodyRaw))
		if validTo.Valid {
			value := validTo.Time.UTC()
			item.ValidTo = &value
		}
		items = append(items, item)
	}
	return items, nil
}

func (s *Server) loadAdminTemporalMemoryEdges(nodeIDs []string, status string, limit int) ([]store.TemporalMemoryEdge, error) {
	if len(nodeIDs) == 0 {
		return nil, nil
	}
	if limit <= 0 {
		limit = 600
	}
	args := make([]any, 0, len(nodeIDs)*2+2)
	placeholdersLeft := make([]string, 0, len(nodeIDs))
	placeholdersRight := make([]string, 0, len(nodeIDs))
	for _, nodeID := range nodeIDs {
		args = append(args, nodeID)
		placeholdersLeft = append(placeholdersLeft, fmt.Sprintf("$%d", len(args)))
	}
	for _, nodeID := range nodeIDs {
		args = append(args, nodeID)
		placeholdersRight = append(placeholdersRight, fmt.Sprintf("$%d", len(args)))
	}
	query := `
		SELECT id::text, source_id::text, target_id::text, edge_type, weight, evidence_count, status, created_at, updated_at
		FROM memory_edges
		WHERE (source_id::text IN (` + strings.Join(placeholdersLeft, ", ") + `) OR target_id::text IN (` + strings.Join(placeholdersRight, ", ") + `))`
	if status = strings.TrimSpace(status); status != "" {
		args = append(args, status)
		query += fmt.Sprintf(" AND status = $%d", len(args))
	}
	args = append(args, limit)
	query += fmt.Sprintf(" ORDER BY updated_at DESC LIMIT $%d", len(args))

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]store.TemporalMemoryEdge, 0, limit)
	for rows.Next() {
		var item store.TemporalMemoryEdge
		if err := rows.Scan(&item.ID, &item.SourceID, &item.TargetID, &item.EdgeType, &item.Weight, &item.EvidenceCount, &item.Status, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, nil
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

func (s *Server) handleAdminDeleteMemory(w http.ResponseWriter, r *http.Request, userID, botID, memoryID string) {
	if r.Method != http.MethodDelete {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	userID = strings.TrimSpace(userID)
	botID = strings.TrimSpace(botID)
	memoryID = strings.TrimSpace(memoryID)
	if userID == "" || botID == "" || memoryID == "" {
		http.Error(w, "missing parameters", http.StatusBadRequest)
		return
	}
	result, err := s.db.Exec(
		`DELETE FROM memory_nodes WHERE id = $1 AND user_id = $2 AND bot_id = $3`,
		memoryID, userID, botID,
	)
	if err != nil {
		http.Error(w, "Failed to delete memory: "+err.Error(), http.StatusInternalServerError)
		return
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		http.Error(w, "memory not found", http.StatusNotFound)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
