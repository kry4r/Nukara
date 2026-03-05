package persona

import (
	"encoding/json"
	"errors"
	"strings"
)

type Patch struct {
	Relationship      string   `json:"relationship"`
	Role              string   `json:"role"`
	SelfCognitionAdds []string `json:"self_cognition_adds"`
	SpeakingStyleAdds []string `json:"speaking_style_adds"`
	TraitAdds         []string `json:"trait_adds"`
	Gender            string   `json:"gender"`
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
	out := Patch{
		Relationship: strings.TrimSpace(input.Relationship),
		Role:         strings.TrimSpace(input.Role),
	}
	out.SelfCognitionAdds = normalizeAdds(input.SelfCognitionAdds, 4, 40)
	out.SpeakingStyleAdds = normalizeAdds(input.SpeakingStyleAdds, 4, 24)
	out.TraitAdds = normalizeAdds(input.TraitAdds, 4, 16)

	switch strings.ToLower(strings.TrimSpace(input.Gender)) {
	case "", "female", "male", "unknown":
		out.Gender = strings.ToLower(strings.TrimSpace(input.Gender))
	default:
		return Patch{}, errors.New("invalid gender")
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
