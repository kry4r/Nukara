package bootstrap

import "testing"

func TestShouldStartScheduler(t *testing.T) {
	t.Setenv("NUKARA_GATEWAY_ENABLE_SCHEDULER", "")
	if shouldStartScheduler("gateway") {
		t.Fatalf("gateway should not start scheduler by default")
	}
	if !shouldStartScheduler("proactive") {
		t.Fatalf("proactive role should start scheduler")
	}
	if shouldStartScheduler("account") {
		t.Fatalf("account role should not start scheduler")
	}

	t.Setenv("NUKARA_GATEWAY_ENABLE_SCHEDULER", "true")
	if !shouldStartScheduler("gateway") {
		t.Fatalf("gateway should start scheduler when explicitly enabled")
	}
}
