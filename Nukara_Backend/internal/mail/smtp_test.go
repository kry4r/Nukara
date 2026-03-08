package mail

import (
	"strings"
	"testing"
	"time"
)

func TestBuildVerificationMessageIncludesCodeAndTTL(t *testing.T) {
	msg := BuildVerificationMessage("123456", 15*time.Minute)
	if !strings.Contains(msg.Subject, "验证码") {
		t.Fatalf("subject = %q", msg.Subject)
	}
	if !strings.Contains(msg.HTML, "123456") {
		t.Fatalf("html body missing code: %q", msg.HTML)
	}
	if !strings.Contains(msg.HTML, "15 分钟") && !strings.Contains(msg.HTML, "15分钟") {
		t.Fatalf("html body missing ttl text: %q", msg.HTML)
	}
	if !strings.Contains(msg.Text, "123456") {
		t.Fatalf("text body missing code: %q", msg.Text)
	}
}

func TestBuildTestMessageUsesVerificationPreviewLayout(t *testing.T) {
	msg := BuildTestMessage()
	if !strings.Contains(msg.Subject, "SMTP") {
		t.Fatalf("subject = %q", msg.Subject)
	}
	if !strings.Contains(msg.HTML, "246810") {
		t.Fatalf("html body missing preview code: %q", msg.HTML)
	}
	if !strings.Contains(msg.Text, "15 分钟") {
		t.Fatalf("text body missing ttl text: %q", msg.Text)
	}
}
