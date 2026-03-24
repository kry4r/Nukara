package memorygraph

import (
	"math"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"nukara/backend/internal/store"
)

type SeedOptions struct {
	Limit int
	Now   time.Time
}

func BuildCue(query string, recentTexts []string) Cue {
	entityHints := make([]string, 0, len(recentTexts)+4)
	entityHints = append(entityHints, splitCueHints(query)...)
	for _, text := range recentTexts {
		entityHints = append(entityHints, splitCueHints(text)...)
	}
	return Cue{QueryText: strings.TrimSpace(query), EntityHints: uniqueIDs(entityHints)}
}

func SelectSeeds(cue Cue, nodes []store.TemporalMemoryNode, opts SeedOptions, embSvc EmbeddingService) []store.TemporalMemoryNode {
	limit := opts.Limit
	if limit <= 0 {
		limit = 4
	}
	now := effectiveNow(opts.Now)
	scored := make([]ActivatedNode, 0, len(nodes))
	for _, node := range nodes {
		status := strings.TrimSpace(node.Status)
		if status != "" && !strings.EqualFold(status, "active") {
			continue
		}
		scored = append(scored, ActivatedNode{Node: node, Score: scoreNode(node, cue, embSvc, now)})
	}
	sort.Slice(scored, func(i, j int) bool {
		if math.Abs(scored[i].Score-scored[j].Score) > 1e-9 {
			return scored[i].Score > scored[j].Score
		}
		return scored[i].Node.OccurredAt.After(scored[j].Node.OccurredAt)
	})
	out := make([]store.TemporalMemoryNode, 0, minInt(limit, len(scored)))
	for _, item := range scored {
		if len(out) >= limit {
			break
		}
		out = append(out, item.Node)
	}
	return out
}

func BaseRecallNodes(nodes []store.TemporalMemoryNode) []store.TemporalMemoryNode {
	byType := map[string][]store.TemporalMemoryNode{}
	for _, node := range nodes {
		if status := strings.TrimSpace(node.Status); status != "" && !strings.EqualFold(status, "active") {
			continue
		}
		byType[strings.ToLower(strings.TrimSpace(node.NodeType))] = append(byType[strings.ToLower(strings.TrimSpace(node.NodeType))], node)
	}
	out := make([]store.TemporalMemoryNode, 0, 4)
	for _, key := range []string{"self_model", "state_snapshot", "session_summary"} {
		if node := latestNode(byType[key]); node != nil {
			out = append(out, *node)
		}
	}
	for _, node := range byType["promise"] {
		out = append(out, node)
		if len(out) >= 5 {
			break
		}
	}
	return out
}

func MergeSeeds(groups ...[]store.TemporalMemoryNode) []store.TemporalMemoryNode {
	seen := map[string]struct{}{}
	out := make([]store.TemporalMemoryNode, 0)
	for _, group := range groups {
		for _, node := range group {
			id := strings.TrimSpace(node.ID)
			if id == "" {
				continue
			}
			if _, ok := seen[id]; ok {
				continue
			}
			seen[id] = struct{}{}
			out = append(out, node)
		}
	}
	return out
}

func BuildActivationSet(cue Cue, seeds []store.TemporalMemoryNode, nodes []store.TemporalMemoryNode, edges []store.TemporalMemoryEdge, opts ActivationOptions, embSvc EmbeddingService) []ActivatedNode {
	limit := opts.Limit
	if limit <= 0 {
		limit = 6
	}
	maxDepth := opts.MaxDepth
	if maxDepth <= 0 {
		maxDepth = 1
	}
	now := effectiveNow(opts.Now)
	nodeByID := make(map[string]store.TemporalMemoryNode, len(nodes)+len(seeds))
	for _, node := range nodes {
		if strings.TrimSpace(node.ID) == "" {
			continue
		}
		nodeByID[node.ID] = node
	}
	for _, seed := range seeds {
		if strings.TrimSpace(seed.ID) == "" {
			continue
		}
		nodeByID[seed.ID] = seed
	}
	adjacency := buildAdjacency(edges)
	type queuedNode struct {
		id         string
		depth      int
		propagated float64
	}
	queue := make([]queuedNode, 0, len(seeds))
	seenDepth := map[string]int{}
	best := map[string]ActivatedNode{}
	for _, seed := range seeds {
		if strings.TrimSpace(seed.ID) == "" {
			continue
		}
		queue = append(queue, queuedNode{id: seed.ID, depth: 0, propagated: 1.35})
	}
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		node, ok := nodeByID[current.id]
		if !ok {
			continue
		}
		if priorDepth, ok := seenDepth[current.id]; ok && current.depth > priorDepth {
			continue
		}
		seenDepth[current.id] = current.depth
		score := scoreNode(node, cue, embSvc, now) + current.propagated
		reason := "seed"
		if current.depth > 0 {
			reason = "graph_activation"
		}
		if existing, ok := best[node.ID]; !ok || score > existing.Score {
			best[node.ID] = ActivatedNode{Node: node, Score: score, Depth: current.depth, Reason: reason}
		}
		if current.depth >= maxDepth {
			continue
		}
		for _, next := range adjacency[current.id] {
			nextNode, ok := nodeByID[next.NodeID]
			if !ok {
				continue
			}
			nextScore := current.propagated*0.58*maxFloat(next.Weight, 0.35) + associativeBonus(nextNode, now)
			queue = append(queue, queuedNode{id: next.NodeID, depth: current.depth + 1, propagated: nextScore})
		}
	}
	out := make([]ActivatedNode, 0, len(best))
	for _, item := range best {
		out = append(out, item)
	}
	sort.Slice(out, func(i, j int) bool {
		if math.Abs(out[i].Score-out[j].Score) > 1e-9 {
			return out[i].Score > out[j].Score
		}
		if !out[i].Node.OccurredAt.Equal(out[j].Node.OccurredAt) {
			return out[i].Node.OccurredAt.After(out[j].Node.OccurredAt)
		}
		return out[i].Node.ID < out[j].Node.ID
	})
	if len(out) > limit {
		out = out[:limit]
	}
	return out
}

func BuildRecallChains(activated []ActivatedNode, edges []store.TemporalMemoryEdge) []RecallChain {
	if len(activated) == 0 {
		return nil
	}
	nodeByID := make(map[string]store.TemporalMemoryNode, len(activated))
	scoreByID := make(map[string]float64, len(activated))
	for _, item := range activated {
		nodeByID[item.Node.ID] = item.Node
		scoreByID[item.Node.ID] = item.Score
	}
	adjacency := map[string][]string{}
	for _, edge := range edges {
		if !strings.EqualFold(strings.TrimSpace(edge.Status), "") && !strings.EqualFold(strings.TrimSpace(edge.Status), "active") {
			continue
		}
		if !strings.EqualFold(strings.TrimSpace(edge.EdgeType), "follows") {
			continue
		}
		if _, ok := nodeByID[edge.SourceID]; !ok {
			continue
		}
		if _, ok := nodeByID[edge.TargetID]; !ok {
			continue
		}
		adjacency[edge.SourceID] = append(adjacency[edge.SourceID], edge.TargetID)
		adjacency[edge.TargetID] = append(adjacency[edge.TargetID], edge.SourceID)
	}
	visited := map[string]struct{}{}
	chains := make([]RecallChain, 0, len(activated))
	for _, item := range activated {
		if _, ok := visited[item.Node.ID]; ok {
			continue
		}
		componentIDs := collectComponent(item.Node.ID, adjacency)
		for _, id := range componentIDs {
			visited[id] = struct{}{}
		}
		timeline := make([]store.TemporalMemoryNode, 0, len(componentIDs))
		anchor := item.Node
		anchorScore := item.Score
		for _, id := range componentIDs {
			node := nodeByID[id]
			timeline = append(timeline, node)
			if scoreByID[id] > anchorScore {
				anchor = node
				anchorScore = scoreByID[id]
			}
		}
		sortNodesByOccurredAsc(timeline)
		backingIDs := make([]string, 0, len(timeline))
		for _, node := range timeline {
			backingIDs = append(backingIDs, node.ID)
		}
		chains = append(chains, RecallChain{
			Anchor:         anchor,
			Timeline:       timeline,
			WhyRelevant:    explainChain(anchor),
			ChainType:      inferChainType(anchor),
			BackingNodeIDs: backingIDs,
		})
	}
	sort.Slice(chains, func(i, j int) bool {
		if len(chains[i].Timeline) != len(chains[j].Timeline) {
			return len(chains[i].Timeline) > len(chains[j].Timeline)
		}
		left := scoreByID[chains[i].Anchor.ID]
		right := scoreByID[chains[j].Anchor.ID]
		if math.Abs(left-right) > 1e-9 {
			return left > right
		}
		return chains[i].Anchor.OccurredAt.After(chains[j].Anchor.OccurredAt)
	})
	return chains
}

type adjacentEdge struct {
	NodeID string
	Weight float64
}

func buildAdjacency(edges []store.TemporalMemoryEdge) map[string][]adjacentEdge {
	adjacency := map[string][]adjacentEdge{}
	for _, edge := range edges {
		status := strings.TrimSpace(edge.Status)
		if status != "" && !strings.EqualFold(status, "active") {
			continue
		}
		adjacency[edge.SourceID] = append(adjacency[edge.SourceID], adjacentEdge{NodeID: edge.TargetID, Weight: edge.Weight})
		adjacency[edge.TargetID] = append(adjacency[edge.TargetID], adjacentEdge{NodeID: edge.SourceID, Weight: edge.Weight})
	}
	return adjacency
}

func collectComponent(start string, adjacency map[string][]string) []string {
	queue := []string{start}
	seen := map[string]struct{}{start: {}}
	out := []string{start}
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		for _, next := range adjacency[current] {
			if _, ok := seen[next]; ok {
				continue
			}
			seen[next] = struct{}{}
			queue = append(queue, next)
			out = append(out, next)
		}
	}
	return out
}

func scoreNode(node store.TemporalMemoryNode, cue Cue, embSvc EmbeddingService, now time.Time) float64 {
	embScore := embeddingScore(node, cue, embSvc)
	typePrior := nodeTypePrior(node)
	recency := recencyScore(node, now)
	openLoop := 0.0
	if strings.EqualFold(node.NodeType, "promise") && strings.EqualFold(node.Status, "active") {
		openLoop = 1.15
	}
	resolvedPenalty := 0.0
	if strings.EqualFold(node.Status, "resolved") {
		resolvedPenalty = 1.05
	}
	return embScore*1.2 + typePrior + recency + maxFloat(node.Salience, 0)*0.35 - resolvedPenalty + openLoop
}

func embeddingScore(node store.TemporalMemoryNode, cue Cue, embSvc EmbeddingService) float64 {
	if embSvc == nil || cue.QueryEmbedding == nil {
		text := nodeCorpus(node)
		hints := append([]string{cue.QueryText}, cue.EntityHints...)
		return float64(lexicalHits(text, hints)) * 0.72
	}

	nodeEmb, err := embSvc.GetEmbedding(node.ID)
	if err != nil {
		text := nodeCorpus(node)
		hints := append([]string{cue.QueryText}, cue.EntityHints...)
		return float64(lexicalHits(text, hints)) * 0.72
	}

	return CosineSimilarity(nodeEmb, cue.QueryEmbedding)
}

func associativeBonus(node store.TemporalMemoryNode, now time.Time) float64 {
	bonus := 0.1
	if strings.EqualFold(node.NodeType, "promise") && strings.EqualFold(node.Status, "active") {
		bonus += 0.35
	}
	if strings.EqualFold(node.NodeType, "habit") {
		bonus += 0.2
	}
	if strings.EqualFold(node.NodeType, "episode") {
		bonus += 0.08
	}
	return bonus + recencyScore(node, now)*0.15
}

func nodeTypePrior(node store.TemporalMemoryNode) float64 {
	switch strings.ToLower(strings.TrimSpace(node.NodeType)) {
	case "self_model":
		return 0.95
	case "state_snapshot":
		return 0.25
	case "promise":
		return 0.92
	case "habit":
		return 0.72
	case "user_fact":
		return 0.4
	case "session_summary":
		return 0.36
	case "episode":
		return 0.48
	default:
		return 0.2
	}
}

func recencyScore(node store.TemporalMemoryNode, now time.Time) float64 {
	occurredAt := node.OccurredAt
	if occurredAt.IsZero() {
		occurredAt = now
	}
	ageHours := now.Sub(occurredAt).Hours()
	if ageHours < 0 {
		ageHours = 0
	}
	switch strings.ToLower(strings.TrimSpace(node.NodeType)) {
	case "state_snapshot":
		return boundedDecay(ageHours, 18)
	case "promise":
		if strings.EqualFold(node.Status, "active") {
			return 0.45
		}
		return boundedDecay(ageHours, 72) * 0.5
	case "habit":
		return boundedDecay(ageHours, 24*21)
	case "self_model":
		return boundedDecay(ageHours, 24*30)
	default:
		return boundedDecay(ageHours, 72)
	}
}

func boundedDecay(ageHours float64, halfLife float64) float64 {
	if halfLife <= 0 {
		return 0
	}
	return 1 / (1 + ageHours/halfLife)
}

func explainChain(anchor store.TemporalMemoryNode) string {
	switch strings.ToLower(strings.TrimSpace(anchor.NodeType)) {
	case "promise":
		return "围绕未完成事项的连续回忆"
	case "self_model":
		return "围绕近期自我认知的连续回忆"
	default:
		return "围绕近期经历的连续回忆"
	}
}

func inferChainType(anchor store.TemporalMemoryNode) string {
	switch strings.ToLower(strings.TrimSpace(anchor.NodeType)) {
	case "promise":
		return "open_loop"
	case "self_model":
		return "self_understanding"
	default:
		return "recent_life"
	}
}

func maxFloat(values ...float64) float64 {
	best := 0.0
	for _, value := range values {
		if value > best {
			best = value
		}
	}
	return best
}

func splitCueHints(text string) []string {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}
	fields := strings.FieldsFunc(text, func(r rune) bool {
		switch r {
		case '，', '。', '？', '！', '；', ',', '.', '?', '!', ';', '、', '\n', '\t', ' ':
			return true
		default:
			return false
		}
	})
	out := make([]string, 0, len(fields)+1)
	if utf8.RuneCountInString(text) <= 24 {
		out = append(out, text)
	}
	for _, field := range fields {
		field = strings.TrimSpace(field)
		length := utf8.RuneCountInString(field)
		if length < 2 || length > 18 {
			continue
		}
		out = append(out, field)
	}
	return out
}
