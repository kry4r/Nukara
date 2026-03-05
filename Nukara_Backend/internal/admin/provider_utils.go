package admin

import (
	"encoding/json"
	"strings"
)

func sanitizeProviderID(raw string) string {
	raw = strings.TrimSpace(strings.ToLower(raw))
	if raw == "" {
		return ""
	}
	var builder strings.Builder
	builder.Grow(len(raw))
	lastUnderscore := false
	for _, r := range raw {
		switch {
		case r >= 'a' && r <= 'z':
			builder.WriteRune(r)
			lastUnderscore = false
		case r >= '0' && r <= '9':
			builder.WriteRune(r)
			lastUnderscore = false
		default:
			if !lastUnderscore {
				builder.WriteByte('_')
				lastUnderscore = true
			}
		}
	}
	return strings.Trim(builder.String(), "_")
}

func firstModel(models []string) string {
	for _, model := range models {
		value := strings.TrimSpace(model)
		if value != "" {
			return value
		}
	}
	return ""
}

func normalizeModels(models []string) []string {
	if len(models) == 0 {
		return []string{}
	}
	seen := map[string]struct{}{}
	out := make([]string, 0, len(models))
	for _, model := range models {
		value := strings.TrimSpace(model)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}

func unmarshalModels(raw []byte) []string {
	if len(raw) == 0 {
		return []string{}
	}
	var models []string
	if err := json.Unmarshal(raw, &models); err != nil {
		return []string{}
	}
	return normalizeModels(models)
}
