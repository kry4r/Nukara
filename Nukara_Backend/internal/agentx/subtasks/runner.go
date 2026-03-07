package subtasks

import (
	"context"
	"fmt"
	"strings"
	"time"

	"nukara/backend/internal/agentx/memory"
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

type personaApplyInput = store.PersonaPatchInput

type RunnerDeps struct {
	Store interface {
		UpsertMemoryItem(item store.MemoryItem) (store.MemoryItem, error)
		ListMemoryItems(userID, botID string, limit int) []store.MemoryItem
		UpsertCompact(conversationID, compactJSON, untilTurnID string) error
		GetBot(userID, botID string) (store.Bot, bool)
		ApplyBotPersonaPatch(userID, botID string, input store.PersonaPatchInput) (store.Bot, bool)
		IncrementTurnCount(userID, botID string) int
	}
	MemoryTool interface {
		Save(ctx context.Context, item store.MemoryItem) (store.MemoryItem, error)
	}
	MemoryExtractor MemoryExtractor
	CompactUpdater  CompactUpdater
	PersonaIterator PersonaIterator
}

type Runner struct {
	store      RunnerDeps
	memoryTool interface {
		Save(ctx context.Context, item store.MemoryItem) (store.MemoryItem, error)
	}
	memoryExtractor MemoryExtractor
	compactUpdater  CompactUpdater
	personaIterator PersonaIterator
}

func NewRunner(deps RunnerDeps) *Runner {
	memoryTool := deps.MemoryTool
	if memoryTool == nil {
		memoryTool = memory.NewStore(deps.Store)
	}
	return &Runner{
		store:           deps,
		memoryTool:      memoryTool,
		memoryExtractor: deps.MemoryExtractor,
		compactUpdater:  deps.CompactUpdater,
		personaIterator: deps.PersonaIterator,
	}
}

func (r *Runner) Run(ctx context.Context, in Input) (Result, error) {
	result := Result{}
	if r == nil {
		return result, nil
	}

	if r.memoryExtractor != nil {
		if raw, err := r.memoryExtractor(ctx, in); err == nil {
			_ = r.applyMemory(raw, in)
		}
	}

	if r.compactUpdater != nil {
		if compact, err := r.compactUpdater(ctx, in); err == nil && strings.TrimSpace(compact) != "" {
			_ = r.store.Store.UpsertCompact(in.ConversationID, compact, in.TurnID)
		}
	}

	if r.personaIterator != nil {
		turnCount := r.store.Store.IncrementTurnCount(in.UserID, in.BotID)
		if turnCount%3 == 0 {
			raw, err := r.personaIterator(ctx, in)
			if err != nil {
				return result, nil
			}
			patch, err := persona.ParsePatch(raw)
			if err != nil {
				return result, nil
			}
			patch, err = persona.ValidatePatch(patch)
			if err != nil {
				return result, nil
			}

			bot, found := r.store.Store.GetBot(in.UserID, in.BotID)
			if !found {
				return result, nil
			}
			prompt := persona.CompilePrompt(store.Bot{
				Name:                 bot.Name,
				Identity:             appendTextAdds(bot.Identity, patch.IdentityAdds),
				Personality:          append(append([]string(nil), bot.Personality...), patch.PersonalityAdds...),
				ExpressionStyle:      appendTextAdds(bot.ExpressionStyle, patch.ExpressionStyleAdds),
				LifeContext:          appendTextAdds(bot.LifeContext, patch.LifeContextAdds),
				TaboosAndPreferences: appendTextAdds(bot.TaboosAndPreferences, patch.TaboosAndPreferencesAdds),
			}, 420)

			_, ok := r.store.Store.ApplyBotPersonaPatch(in.UserID, in.BotID, store.PersonaPatchInput{
				IdentityAdds:             patch.IdentityAdds,
				PersonalityAdds:          patch.PersonalityAdds,
				ExpressionStyleAdds:      patch.ExpressionStyleAdds,
				LifeContextAdds:          patch.LifeContextAdds,
				TaboosAndPreferencesAdds: patch.TaboosAndPreferencesAdds,
				PersonaPrompt:            prompt,
			})
			if ok {
				result.PersonaUpdated = true
				result.Patch = patch
				result.PatchSummary = summarizePatch(patch)
			}
		}
	}

	return result, nil
}

func (r *Runner) applyMemory(raw string, in Input) error {
	items, err := ParseMemoryItems(raw)
	if err != nil {
		return err
	}
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
			if _, err := r.memoryTool.Save(context.Background(), item); err != nil {
				return err
			}
			continue
		}
		if _, err := r.store.Store.UpsertMemoryItem(item); err != nil {
			return err
		}
	}
	return nil
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
