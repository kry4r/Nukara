package memorygraph

import (
	"strings"
	"unicode"

	"nukara/backend/internal/store"
)

type SimilarityMatch struct {
	NodeID      string
	Similarity  float64
	ShouldMerge bool
}

func findBestSimilarityNode(candidate store.TemporalMemoryNode, existing []store.TemporalMemoryNode) (SimilarityMatch, store.TemporalMemoryNode, bool) {
	best := SimilarityMatch{}
	var matched store.TemporalMemoryNode
	found := false
	for _, node := range existing {
		if strings.TrimSpace(node.ID) == "" || strings.TrimSpace(node.ID) == strings.TrimSpace(candidate.ID) {
			continue
		}
		score := nodeSimilarity(candidate, node)
		if !found || score > best.Similarity {
			best = SimilarityMatch{NodeID: node.ID, Similarity: score, ShouldMerge: score >= 0.72}
			matched = node
			found = true
		}
	}
	if !found {
		return SimilarityMatch{}, store.TemporalMemoryNode{}, false
	}
	return best, matched, true
}

func nodeSimilarity(a, b store.TemporalMemoryNode) float64 {
	if strings.TrimSpace(a.NodeType) != strings.TrimSpace(b.NodeType) {
		return 0
	}
	if strings.TrimSpace(a.SemanticCategory) != "" && strings.TrimSpace(b.SemanticCategory) != "" && strings.TrimSpace(a.SemanticCategory) != strings.TrimSpace(b.SemanticCategory) {
		return 0
	}
	if strings.TrimSpace(a.MergeKey) != "" && strings.TrimSpace(a.MergeKey) == strings.TrimSpace(b.MergeKey) {
		return 1
	}
	textA := similarityText(a)
	textB := similarityText(b)
	if textA == "" || textB == "" {
		return 0
	}
	if textA == textB {
		return 1
	}
	return diceCoefficient(textA, textB)
}

func similarityText(node store.TemporalMemoryNode) string {
	return normalizeSimilarityText(strings.TrimSpace(node.Title + " " + node.Summary))
}

func normalizeSimilarityText(value string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(strings.TrimSpace(value)) {
		if unicode.IsSpace(r) || unicode.IsPunct(r) || unicode.IsSymbol(r) {
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

func diceCoefficient(left, right string) float64 {
	leftSet := runeFrequency(left)
	rightSet := runeFrequency(right)
	if len(leftSet) == 0 || len(rightSet) == 0 {
		return 0
	}
	shared := 0
	leftCount := 0
	rightCount := 0
	for _, count := range leftSet {
		leftCount += count
	}
	for _, count := range rightSet {
		rightCount += count
	}
	for r, leftCountForRune := range leftSet {
		rightCountForRune := rightSet[r]
		if rightCountForRune == 0 {
			continue
		}
		if leftCountForRune < rightCountForRune {
			shared += leftCountForRune
		} else {
			shared += rightCountForRune
		}
	}
	return (2 * float64(shared)) / float64(leftCount+rightCount)
}

func runeFrequency(value string) map[rune]int {
	out := map[rune]int{}
	for _, r := range value {
		out[r]++
	}
	return out
}

func mergeNode(existing, candidate store.TemporalMemoryNode) store.TemporalMemoryNode {
	merged := existing
	if len([]rune(candidate.Summary)) > len([]rune(merged.Summary)) {
		merged.Summary = candidate.Summary
	}
	if len([]rune(candidate.Title)) > len([]rune(merged.Title)) {
		merged.Title = candidate.Title
	}
	if candidate.OccurredAt.After(merged.OccurredAt) {
		merged.OccurredAt = candidate.OccurredAt
	}
	if candidate.ObservedAt.After(merged.ObservedAt) {
		merged.ObservedAt = candidate.ObservedAt
	}
	if candidate.ValidFrom.Before(merged.ValidFrom) || merged.ValidFrom.IsZero() {
		merged.ValidFrom = candidate.ValidFrom
	}
	if candidate.Salience > merged.Salience {
		merged.Salience = candidate.Salience
	}
	if candidate.Confidence > merged.Confidence {
		merged.Confidence = candidate.Confidence
	}
	if candidate.Stability > merged.Stability {
		merged.Stability = candidate.Stability
	}
	if strings.TrimSpace(merged.SourceKind) == "" {
		merged.SourceKind = strings.TrimSpace(candidate.SourceKind)
	}
	if strings.TrimSpace(merged.SemanticCategory) == "" {
		merged.SemanticCategory = strings.TrimSpace(candidate.SemanticCategory)
	}
	if strings.TrimSpace(merged.StabilityLabel) == "" {
		merged.StabilityLabel = strings.TrimSpace(candidate.StabilityLabel)
	}
	if strings.TrimSpace(merged.MergeKey) == "" {
		merged.MergeKey = strings.TrimSpace(candidate.MergeKey)
	}
	merged.EvidenceCount += maxInt(candidate.EvidenceCount, 1)
	merged.Entities = mergeEntities(merged.Entities, candidate.Entities)
	return merged
}

func mergeEntities(left, right []store.Entity) []store.Entity {
	out := append([]store.Entity(nil), left...)
	seen := map[string]struct{}{}
	for _, item := range out {
		key := strings.TrimSpace(item.ID) + ":" + strings.TrimSpace(item.Name)
		seen[key] = struct{}{}
	}
	for _, item := range right {
		key := strings.TrimSpace(item.ID) + ":" + strings.TrimSpace(item.Name)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, item)
	}
	return out
}

func maxInt(left, right int) int {
	if left > right {
		return left
	}
	return right
}
