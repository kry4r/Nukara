package admin

import "testing"

func TestNormalizeSelfCognitionSummaryConfigPayloadTrimsValues(t *testing.T) {
	payload := normalizeSelfCognitionSummaryConfigPayload(selfCognitionSummaryConfigPayload{
		ProviderID: " summary_provider ",
		Model:      " summary-model ",
	})
	if payload.ProviderID != "summary_provider" {
		t.Fatalf("provider id = %q", payload.ProviderID)
	}
	if payload.Model != "summary-model" {
		t.Fatalf("model = %q", payload.Model)
	}
}

func TestNormalizeSelfCognitionSummaryConfigPayloadAllowsEmptyModel(t *testing.T) {
	payload := normalizeSelfCognitionSummaryConfigPayload(selfCognitionSummaryConfigPayload{
		ProviderID: "summary_provider",
		Model:      "   ",
	})
	if payload.ProviderID != "summary_provider" {
		t.Fatalf("provider id = %q", payload.ProviderID)
	}
	if payload.Model != "" {
		t.Fatalf("model = %q, want empty", payload.Model)
	}
}
