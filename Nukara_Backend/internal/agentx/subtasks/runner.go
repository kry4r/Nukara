package subtasks

import (
	"context"
	"fmt"
	"strings"
	"time"

	"nukara/backend/internal/agentx/memorygraph"
	"nukara/backend/internal/agentx/persona"
	"nukara/backend/internal/store"
)

type Input struct {
	UserID         string
	BotID          string
	ConversationID string
	TurnID         string
	UserText       string
	BotText        string
}

type Result struct {
	PersonaUpdated bool
	PatchSummary   string
	Patch          persona.Patch
}

type MemoryExtractor func(ctx context.Context, in Input) (string, error)
type CompactUpdater func(ctx context.Context, in Input) (string, error)
type PersonaIterator func(ctx context.Context, in Input) (string, error)
type SelfCognitionUpdater func(ctx context.Context, in Input, bot store.Bot, changes []store.PersonaChangeEvent) (store.Bot, error)
type MemoryGraphService interface {
	IngestTurn(ctx context.Context, in memorygraph.IngestTurnInput) (memorygraph.IngestTurnResult, error)
}

type personaApplyInput = store.PersonaPatchInput

type RunnerDeps struct {
	Store interface {
		UpsertMemoryItem(item store.MemoryItem) (store.MemoryItem, error)
		ListMemoryItems(userID, botID string, limit int) []store.MemoryItem
		UpsertCompact(conversationID, compactJSON, untilTurnID string) error
		GetBot(userID, botID string) (store.Bot, bool)
		ApplyBotPersonaPatch(userID, botID string, input store.PersonaPatchInput) (store.Bot, bool)
		UpsertBotRuntimeState(state store.BotRuntimeState) (store.BotRuntimeState, error)
		CreatePersonaChangeEvent(event store.PersonaChangeEvent) (store.PersonaChangeEvent, error)
		IncrementTurnCount(userID, botID string) int
	}
	MemoryTool interface {
		Save(ctx context.Context, item store.MemoryItem) (store.MemoryItem, error)
	}
	MemoryGraph          MemoryGraphService
	MemoryExtractor      MemoryExtractor
	CompactUpdater       CompactUpdater
	PersonaIterator      PersonaIterator
	SelfCognitionUpdater SelfCognitionUpdater
}

type Runner struct {
	store      RunnerDeps
	memoryTool interface {
		Save(ctx context.Context, item store.MemoryItem) (store.MemoryItem, error)
	}
	memoryExtractor      MemoryExtractor
	compactUpdater       CompactUpdater
	personaIterator      PersonaIterator
	selfCognitionUpdater SelfCognitionUpdater
	memoryGraph          MemoryGraphService
}

func NewRunner(deps RunnerDeps) *Runner {
	return &Runner{
		store:                deps,
		memoryTool:           deps.MemoryTool,
		memoryExtractor:      deps.MemoryExtractor,
		compactUpdater:       deps.CompactUpdater,
		personaIterator:      deps.PersonaIterator,
		selfCognitionUpdater: deps.SelfCognitionUpdater,
		memoryGraph:          deps.MemoryGraph,
	}
}

func (r *Runner) Run(ctx context.Context, in Input) (Result, error) {
	result := Result{}
	if r == nil {
		return result, nil
	}

	savedItems := []store.MemoryItem(nil)
	if r.memoryExtractor != nil {
		if raw, err := r.memoryExtractor(ctx, in); err == nil {
			if items, applyErr := r.applyMemory(raw, in); applyErr == nil {
				savedItems = items
				r.updateRuntimeState(items, in)
			}
		}
	}

	compactJSON := ""
	if r.compactUpdater != nil {
		if compact, err := r.compactUpdater(ctx, in); err == nil && strings.TrimSpace(compact) != "" {
			compactJSON = compact
			_ = r.store.Store.UpsertCompact(in.ConversationID, compact, in.TurnID)
		}
	}

	if r.memoryGraph != nil && (len(savedItems) > 0 || strings.TrimSpace(compactJSON) != "") {
		_, _ = r.memoryGraph.IngestTurn(ctx, memorygraph.IngestTurnInput{
			UserID:         in.UserID,
			BotID:          in.BotID,
			ConversationID: in.ConversationID,
			TurnID:         in.TurnID,
			Items:          append([]store.MemoryItem(nil), savedItems...),
			CompactJSON:    compactJSON,
			Now:            time.Now().UTC(),
		})
	}

	if len(savedItems) > 0 {
		r.applyPersonaDecisions(ctx, in, savedItems, &result)
	}

	return result, nil
}

func (r *Runner) applyMemory(raw string, in Input) ([]store.MemoryItem, error) {
	items, err := ParseMemoryItems(raw)
	if err != nil {
		return nil, err
	}
	savedItems := make([]store.MemoryItem, 0, len(items))
	for _, item := range items {
		if strings.TrimSpace(item.Content) == "" {
			continue
		}
		if len([]rune(item.Content)) > 160 {
			item.Content = string([]rune(item.Content)[:160])
		}
		item.UserID = in.UserID
		item.BotID = in.BotID
		if item.OccurredAt.IsZero() {
			item.OccurredAt = time.Now().UTC()
		}
		if strings.TrimSpace(item.Status) == "" {
			item.Status = "active"
		}
		if r.memoryTool != nil {
			saved, saveErr := r.memoryTool.Save(context.Background(), item)
			if saveErr != nil {
				return nil, saveErr
			}
			savedItems = append(savedItems, saved)
			continue
		}
		saved, saveErr := r.store.Store.UpsertMemoryItem(item)
		if saveErr != nil {
			return nil, saveErr
		}
		savedItems = append(savedItems, saved)
	}
	return savedItems, nil
}

func (r *Runner) updateRuntimeState(items []store.MemoryItem, in Input) {
	if len(items) == 0 {
		return
	}
	for i := len(items) - 1; i >= 0; i-- {
		item := items[i]
		if !shouldUseForRuntimeState(item) {
			continue
		}
		_, _ = r.store.Store.UpsertBotRuntimeState(store.BotRuntimeState{
			UserID:          in.UserID,
			BotID:           in.BotID,
			ActivityText:    strings.TrimSpace(item.Content),
			BasisTags:       []string{item.Kind},
			SourceMemoryIDs: []string{item.ID},
			UpdatedAt:       item.OccurredAt,
		})
		return
	}
}

func shouldUseForRuntimeState(item store.MemoryItem) bool {
	kind := strings.TrimSpace(item.Kind)
	if kind != "self_fact" && kind != "state_basis" && kind != "event" && kind != "habit" {
		return false
	}
	return strings.TrimSpace(item.Content) != ""
}

func (r *Runner) applyPersonaDecisions(ctx context.Context, in Input, items []store.MemoryItem, result *Result) {
	bot, found := r.store.Store.GetBot(in.UserID, in.BotID)
	if !found {
		return
	}
	acceptedChanges := make([]store.PersonaChangeEvent, 0, len(items))
	mergedPatch := persona.Patch{}
	for _, item := range items {
		decision, ok := decidePersonaChange(item, bot)
		if !ok {
			continue
		}
		event := store.PersonaChangeEvent{
			UserID:        in.UserID,
			BotID:         in.BotID,
			Field:         decision.Field,
			ChangeType:    "append",
			ProposedValue: strings.TrimSpace(item.Content),
			SourceTurnID:  in.TurnID,
			Risk:          strings.TrimSpace(decision.Risk),
			Status:        strings.TrimSpace(decision.Status),
		}
		if decision.ShouldApply {
			updatedBot, ok := r.store.Store.ApplyBotPersonaPatch(in.UserID, in.BotID, decision.Patch)
			if !ok {
				event.Status = "failed"
				_, _ = r.store.Store.CreatePersonaChangeEvent(event)
				continue
			}
			bot = updatedBot
			accepted, err := r.store.Store.CreatePersonaChangeEvent(event)
			if err != nil {
				continue
			}
			acceptedChanges = append(acceptedChanges, accepted)
			mergedPatch = mergePersonaPatch(mergedPatch, decision.Patch)
			result.PersonaUpdated = true
			continue
		}
		_, _ = r.store.Store.CreatePersonaChangeEvent(event)
	}
	if r.selfCognitionUpdater != nil && len(acceptedChanges) > 0 {
		if updatedBot, err := r.selfCognitionUpdater(ctx, in, bot, acceptedChanges); err == nil {
			bot = updatedBot
			_ = bot
		}
	}
	if result.PersonaUpdated {
		result.Patch = mergedPatch
		result.PatchSummary = summarizePatch(mergedPatch)
	}
}

type personaChangeCandidate struct {
	field string
	adds  []string
}

func (r *Runner) createPendingPersonaChanges(patch persona.Patch, in Input) ([]store.PersonaChangeEvent, error) {
	return r.createPersonaChanges(patch, in, string(persona.RiskHigh), "pending")
}

func (r *Runner) createAcceptedPersonaChanges(patch persona.Patch, in Input, risk persona.RiskLevel) ([]store.PersonaChangeEvent, error) {
	return r.createPersonaChanges(patch, in, string(risk), "accepted")
}

func (r *Runner) createPersonaChanges(patch persona.Patch, in Input, risk, status string) ([]store.PersonaChangeEvent, error) {
	created := make([]store.PersonaChangeEvent, 0, 5)
	for _, candidate := range personaChangeCandidates(patch) {
		if len(candidate.adds) == 0 {
			continue
		}
		event, err := r.store.Store.CreatePersonaChangeEvent(store.PersonaChangeEvent{
			UserID:        in.UserID,
			BotID:         in.BotID,
			Field:         candidate.field,
			ChangeType:    "append",
			ProposedValue: strings.Join(candidate.adds, "；"),
			SourceTurnID:  in.TurnID,
			Risk:          strings.TrimSpace(risk),
			Status:        strings.TrimSpace(status),
		})
		if err != nil {
			return nil, err
		}
		created = append(created, event)
	}
	return created, nil
}

func personaChangeCandidates(patch persona.Patch) []personaChangeCandidate {
	return []personaChangeCandidate{
		{field: "identity", adds: patch.IdentityAdds},
		{field: "personality", adds: patch.PersonalityAdds},
		{field: "expression_style", adds: patch.ExpressionStyleAdds},
		{field: "life_context", adds: patch.LifeContextAdds},
		{field: "taboos_and_preferences", adds: patch.TaboosAndPreferencesAdds},
	}
}

func summarizePatch(p persona.Patch) string {
	chunks := make([]string, 0, 5)
	if len(p.IdentityAdds) > 0 {
		chunks = append(chunks, "身份设定+"+fmt.Sprintf("%d", len(p.IdentityAdds)))
	}
	if len(p.PersonalityAdds) > 0 {
		chunks = append(chunks, "性格特征+"+fmt.Sprintf("%d", len(p.PersonalityAdds)))
	}
	if len(p.ExpressionStyleAdds) > 0 {
		chunks = append(chunks, "表达风格微调")
	}
	if len(p.LifeContextAdds) > 0 {
		chunks = append(chunks, "生活环境补充")
	}
	if len(p.TaboosAndPreferencesAdds) > 0 {
		chunks = append(chunks, "偏好边界更新")
	}
	if len(chunks) == 0 {
		return "人设微调"
	}
	return strings.Join(chunks, "，")
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func split(v string) []string {
	if strings.TrimSpace(v) == "" {
		return nil
	}
	parts := strings.Split(v, "|")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

func appendTextAdds(base string, adds []string) string {
	parts := make([]string, 0, len(adds)+1)
	parts = append(parts, splitTextValues(base)...)
	parts = append(parts, adds...)
	seen := map[string]struct{}{}
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if _, ok := seen[part]; ok {
			continue
		}
		seen[part] = struct{}{}
		out = append(out, part)
	}
	return strings.Join(out, "；")
}

func splitTextValues(v string) []string {
	if strings.TrimSpace(v) == "" {
		return nil
	}
	return strings.FieldsFunc(v, func(r rune) bool {
		switch r {
		case '|', '；', ';', '\n':
			return true
		default:
			return false
		}
	})
}
