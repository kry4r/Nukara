package subtasks

import (
	"strings"
	"unicode"

	"nukara/backend/internal/agentx/persona"
	"nukara/backend/internal/store"
)

type personaDecision struct {
	Field       string
	Patch       store.PersonaPatchInput
	ShouldApply bool
	Status      string
	Reason      string
	Risk        string
}

func decidePersonaChange(item store.MemoryItem, bot store.Bot) (personaDecision, bool) {
	field := mapSemanticCategoryToPersonaField(item)
	if field == "" {
		return personaDecision{}, false
	}
	decision := personaDecision{
		Field:  field,
		Status: "skipped",
		Reason: "not_eligible",
		Risk:   riskForPersonaField(field),
	}
	if strings.TrimSpace(item.Stability) != "stable" {
		decision.Reason = "temporary_fact"
		return decision, true
	}
	if strings.TrimSpace(item.Kind) == "user_fact" && field != "taboos_and_preferences" {
		decision.Reason = "user_fact_does_not_mutate_bot_persona"
		return decision, true
	}
	patch := patchFromMemoryItem(field, item.Content)
	if isEmptyPersonaPatch(patch) {
		return personaDecision{}, false
	}
	if personaFieldAlreadyCovers(bot, field, item.Content) {
		decision.Reason = "already_covered"
		return decision, true
	}
	decision.Patch = patch
	decision.ShouldApply = true
	decision.Status = "accepted"
	decision.Reason = "stable_core_fact"
	return decision, true
}

func mapSemanticCategoryToPersonaField(item store.MemoryItem) string {
	switch strings.TrimSpace(item.SemanticCategory) {
	case "identity":
		return "identity"
	case "personality":
		return "personality"
	case "expression_style":
		return "expression_style"
	case "life_context":
		return "life_context"
	case "taboos_and_preferences":
		return "taboos_and_preferences"
	default:
		return ""
	}
}

func patchFromMemoryItem(field, content string) store.PersonaPatchInput {
	content = strings.TrimSpace(content)
	patch := store.PersonaPatchInput{}
	switch field {
	case "identity":
		patch.IdentityAdds = []string{content}
	case "personality":
		patch.PersonalityAdds = []string{content}
	case "expression_style":
		patch.ExpressionStyleAdds = []string{content}
	case "life_context":
		patch.LifeContextAdds = []string{content}
	case "taboos_and_preferences":
		patch.TaboosAndPreferencesAdds = []string{content}
	}
	return patch
}

func isEmptyPersonaPatch(patch store.PersonaPatchInput) bool {
	return len(patch.IdentityAdds) == 0 &&
		len(patch.PersonalityAdds) == 0 &&
		len(patch.ExpressionStyleAdds) == 0 &&
		len(patch.LifeContextAdds) == 0 &&
		len(patch.TaboosAndPreferencesAdds) == 0
}

func personaFieldAlreadyCovers(bot store.Bot, field, content string) bool {
	needle := normalizePersonaComparison(content)
	if needle == "" {
		return false
	}
	haystacks := []string{}
	switch field {
	case "identity":
		haystacks = append(haystacks, bot.Identity)
	case "personality":
		haystacks = append(haystacks, bot.Personality...)
	case "expression_style":
		haystacks = append(haystacks, bot.ExpressionStyle)
	case "life_context":
		haystacks = append(haystacks, bot.LifeContext)
	case "taboos_and_preferences":
		haystacks = append(haystacks, bot.TaboosAndPreferences)
	}
	for _, haystack := range haystacks {
		normalized := normalizePersonaComparison(haystack)
		if normalized == "" {
			continue
		}
		if normalized == needle || strings.Contains(normalized, needle) || strings.Contains(needle, normalized) {
			return true
		}
	}
	return false
}

func normalizePersonaComparison(value string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(strings.TrimSpace(value)) {
		if unicode.IsSpace(r) || unicode.IsPunct(r) || unicode.IsSymbol(r) {
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

func riskForPersonaField(field string) string {
	switch field {
	case "identity", "personality":
		return string(persona.RiskHigh)
	default:
		return string(persona.RiskLow)
	}
}

func mergePersonaPatch(dst persona.Patch, src store.PersonaPatchInput) persona.Patch {
	dst.IdentityAdds = append(dst.IdentityAdds, src.IdentityAdds...)
	dst.PersonalityAdds = append(dst.PersonalityAdds, src.PersonalityAdds...)
	dst.ExpressionStyleAdds = append(dst.ExpressionStyleAdds, src.ExpressionStyleAdds...)
	dst.LifeContextAdds = append(dst.LifeContextAdds, src.LifeContextAdds...)
	dst.TaboosAndPreferencesAdds = append(dst.TaboosAndPreferencesAdds, src.TaboosAndPreferencesAdds...)
	return dst
}
