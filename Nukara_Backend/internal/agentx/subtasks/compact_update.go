package subtasks

import (
	"encoding/json"
	"strings"
)

type compactPayload struct {
	Summary string   `json:"summary"`
	Facts   []string `json:"facts"`
}

func NormalizeCompactJSON(raw string) (string, error) {
	start := strings.Index(raw, "{")
	end := strings.LastIndex(raw, "}")
	if start >= 0 && end > start {
		raw = raw[start : end+1]
	}
	var payload compactPayload
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return "", err
	}
	normalized, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	return string(normalized), nil
}
