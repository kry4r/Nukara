package admin

import "testing"

func TestNormalizeEmailAuthSettingsPayloadDefaultsTTLAndTrims(t *testing.T) {
	payload := normalizeEmailAuthSettingsPayload(emailAuthSettingsPayload{
		SMTPHost:       " smtp.qq.com ",
		SMTPPort:       " 465 ",
		SMTPUsername:   " qq@example.com ",
		SMTPPassword:   " secret ",
		FromEmail:      " qq@example.com ",
		FromName:       " Nukara ",
		CodeTTLSeconds: 0,
	})
	if payload.SMTPHost != "smtp.qq.com" {
		t.Fatalf("smtp host = %q", payload.SMTPHost)
	}
	if payload.CodeTTLSeconds != 900 {
		t.Fatalf("ttl = %d, want 900", payload.CodeTTLSeconds)
	}
}

func TestMaskEmailAuthSettingsPayloadHidesPasswordButKeepsConfiguredState(t *testing.T) {
	payload := maskEmailAuthSettingsPayload(emailAuthSettingsPayload{
		SMTPPassword: "secret",
	})
	if payload.SMTPPassword != "" {
		t.Fatalf("smtp password should be masked, got %q", payload.SMTPPassword)
	}
	if !payload.SMTPPasswordConfigured {
		t.Fatal("expected smtp password configured flag to be true")
	}
}
