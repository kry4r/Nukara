package persona

import (
	"encoding/json"
	"errors"
	"strings"
)

type Patch struct {
	IdentityAdds             []string `json:"identity_adds"`
	PersonalityAdds          []string `json:"personality_adds"`
	ExpressionStyleAdds      []string `json:"expression_style_adds"`
	LifeContextAdds          []string `json:"life_context_adds"`
	TaboosAndPreferencesAdds []string `json:"taboos_and_preferences_adds"`
}

func ParsePatch(raw string) (Patch, error) {
	start := strings.Index(raw, "{")
	end := strings.LastIndex(raw, "}")
	if start < 0 || end <= start {
		return Patch{}, errors.New("patch json not found")
	}
	raw = raw[start : end+1]

	var patch Patch
	if err := json.Unmarshal([]byte(raw), &patch); err != nil {
		return Patch{}, err
	}
	return patch, nil
}

func ValidatePatch(input Patch) (Patch, error) {
	out := Patch{}
	out.IdentityAdds = normalizeAdds(input.IdentityAdds, 4, 40)
	out.PersonalityAdds = normalizeAdds(input.PersonalityAdds, 4, 16)
	out.ExpressionStyleAdds = normalizeAdds(input.ExpressionStyleAdds, 4, 24)
	out.LifeContextAdds = normalizeAdds(input.LifeContextAdds, 4, 40)
	out.TaboosAndPreferencesAdds = normalizeAdds(input.TaboosAndPreferencesAdds, 4, 40)
	if len(out.IdentityAdds) == 0 && len(out.PersonalityAdds) == 0 && len(out.ExpressionStyleAdds) == 0 && len(out.LifeContextAdds) == 0 && len(out.TaboosAndPreferencesAdds) == 0 {
		return Patch{}, errors.New("empty patch")
	}
	return out, nil
}

func normalizeAdds(values []string, maxItems, maxRunes int) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, maxItems)
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		runes := []rune(value)
		if len(runes) > maxRunes {
			value = string(runes[:maxRunes])
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
		if len(out) >= maxItems {
			break
		}
	}
	return out
}
