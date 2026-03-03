package admin

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestSaveNanobotConfigPreservesUnrelatedSections(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.json")
	statePath := filepath.Join(dir, "provider-state.json")

	original := `{
  "agents": {
    "defaults": {
      "model": "xminimaxm25",
      "temperature": 0.7
    }
  },
  "channels": {
    "extend_chat": {
      "enabled": true,
      "port": 8081
    }
  },
  "providers": {
    "custom": {
      "api_key": "a",
      "api_base": "https://example.com/v1"
    },
    "runtime": {
      "api_key": "b",
      "api_base": "https://example.com/v2"
    }
  },
  "tools": {
    "disabled_tools": [
      "exec"
    ]
  }
}`
	if err := os.WriteFile(configPath, []byte(original), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	s := &Server{
		nanobotConfigPath: configPath,
		nanobotStatePath:  statePath,
	}
	cfg, err := s.loadNanobotConfig()
	if err != nil {
		t.Fatalf("loadNanobotConfig: %v", err)
	}

	cfg.Agents.Defaults.Model = "MiniMax-M2.5"
	cfg.Providers["runtime"] = nanobotProvider{
		APIKey:  "new-key",
		APIBase: "https://new.example.com/v1",
	}
	if err := s.saveNanobotConfig(cfg); err != nil {
		t.Fatalf("saveNanobotConfig: %v", err)
	}

	raw, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("unmarshal config: %v", err)
	}

	channels, ok := doc["channels"].(map[string]any)
	if !ok {
		t.Fatalf("channels section was lost: %v", doc["channels"])
	}
	extend, ok := channels["extend_chat"].(map[string]any)
	if !ok {
		t.Fatalf("extend_chat section was lost: %v", channels["extend_chat"])
	}
	enabled, ok := extend["enabled"].(bool)
	if !ok || !enabled {
		t.Fatalf("extend_chat.enabled changed unexpectedly: %v", extend["enabled"])
	}

	if _, ok := doc["tools"].(map[string]any); !ok {
		t.Fatalf("tools section was lost: %v", doc["tools"])
	}

	agents := doc["agents"].(map[string]any)
	defaults := agents["defaults"].(map[string]any)
	if defaults["model"] != "MiniMax-M2.5" {
		t.Fatalf("model not updated, got=%v", defaults["model"])
	}

	providers := doc["providers"].(map[string]any)
	runtime := providers["runtime"].(map[string]any)
	if runtime["api_key"] != "new-key" || runtime["api_base"] != "https://new.example.com/v1" {
		t.Fatalf("runtime provider not updated correctly: %+v", runtime)
	}
}
