package subtasks

import (
	"context"
	"fmt"
	"strings"
	"time"

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
		UpsertCompact(conversationID, compactJSON, untilTurnID string) error
		GetBot(userID, botID string) (store.Bot, bool)
		ApplyBotPersonaPatch(userID, botID string, input store.PersonaPatchInput) (store.Bot, bool)
	}
	MemoryExtractor MemoryExtractor
	CompactUpdater  CompactUpdater
	PersonaIterator PersonaIterator
}

type Runner struct {
	store           RunnerDeps
	memoryExtractor MemoryExtractor
	compactUpdater  CompactUpdater
	personaIterator PersonaIterator
}

func NewRunner(deps RunnerDeps) *Runner {
	return &Runner{
		store:           deps,
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
			Name:          bot.Name,
			Relationship:  firstNonEmpty(patch.Relationship, bot.Relationship),
			Role:          firstNonEmpty(patch.Role, bot.Role),
			SelfCognition: append(append([]string(nil), bot.SelfCognition...), patch.SelfCognitionAdds...),
			SpeakingStyle: strings.Join(append(split(bot.SpeakingStyle), patch.SpeakingStyleAdds...), "|"),
			Traits:        append(append([]string(nil), bot.Traits...), patch.TraitAdds...),
			Gender:        firstNonEmpty(patch.Gender, bot.Gender),
		}, 420)

		var genderPtr *string
		if patch.Gender != "" {
			gender := patch.Gender
			genderPtr = &gender
		}
		_, ok := r.store.Store.ApplyBotPersonaPatch(in.UserID, in.BotID, store.PersonaPatchInput{
			Relationship:      patch.Relationship,
			Role:              patch.Role,
			SelfCognitionAdds: patch.SelfCognitionAdds,
			SpeakingStyleAdds: patch.SpeakingStyleAdds,
			TraitAdds:         patch.TraitAdds,
			Gender:            genderPtr,
			PersonaPrompt:     prompt,
		})
		if ok {
			result.PersonaUpdated = true
			result.Patch = patch
			result.PatchSummary = summarizePatch(patch)
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
		if _, err := r.store.Store.UpsertMemoryItem(item); err != nil {
			return err
		}
	}
	return nil
}

func summarizePatch(p persona.Patch) string {
	chunks := make([]string, 0, 4)
	if p.Relationship != "" {
		chunks = append(chunks, "关系更新")
	}
	if p.Role != "" {
		chunks = append(chunks, "角色设定更新")
	}
	if len(p.SelfCognitionAdds) > 0 {
		chunks = append(chunks, "自我认知+"+fmt.Sprintf("%d", len(p.SelfCognitionAdds)))
	}
	if len(p.SpeakingStyleAdds) > 0 || len(p.TraitAdds) > 0 {
		chunks = append(chunks, "风格微调")
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
