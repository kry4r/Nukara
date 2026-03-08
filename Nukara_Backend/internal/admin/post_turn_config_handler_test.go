package admin

import "testing"

func TestNormalizePostTurnConfigPayloadTrimsValues(t *testing.T) {
	payload := normalizePostTurnConfigPayload(postTurnConfigPayload{
		ProviderID: " minimax_m2_5 ",
		Model:      " MiniMax-M2.5-Small ",
	})
	if payload.ProviderID != "minimax_m2_5" {
		t.Fatalf("provider id = %q", payload.ProviderID)
	}
	if payload.Model != "MiniMax-M2.5-Small" {
		t.Fatalf("model = %q", payload.Model)
	}
}

func TestNormalizePostTurnConfigPayloadAllowsEmptyModel(t *testing.T) {
	payload := normalizePostTurnConfigPayload(postTurnConfigPayload{
		ProviderID: "provider_a",
		Model:      "   ",
	})
	if payload.ProviderID != "provider_a" {
		t.Fatalf("provider id = %q", payload.ProviderID)
	}
	if payload.Model != "" {
		t.Fatalf("model = %q, want empty", payload.Model)
	}
}
