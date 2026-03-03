package agent

import "testing"

func TestExtractEmotion(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantTxt string
		wantEmo string
	}{
		{"with tag", "你好呀 [emotion:happy]", "你好呀", "happy"},
		{"no tag", "你好呀", "你好呀", "gentle"},
		{"empty", "", "", "neutral"},
		{"tag only", "[emotion:sad]", "[emotion:sad]", "gentle"},
		{"trailing space", "嗨 [emotion:love]  ", "嗨", "love"},
		{"mid-text tag ignored", "[emotion:happy] 你好", "[emotion:happy] 你好", "gentle"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			txt, emo := ExtractEmotion(tt.input)
			if txt != tt.wantTxt {
				t.Errorf("text = %q, want %q", txt, tt.wantTxt)
			}
			if emo != tt.wantEmo {
				t.Errorf("emotion = %q, want %q", emo, tt.wantEmo)
			}
		})
	}
}

func TestExtractStatus(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		emotion   string
		wantTxt   string
		wantEmoji string
		wantStat  string
	}{
		{"with tag", "你好呀 [status:😊,开心]", "happy", "你好呀", "😊", "开心"},
		{"no tag fallback", "你好呀", "happy", "你好呀", "😊", "聊天中"},
		{"no tag default emotion", "你好呀", "unknown", "你好呀", "☕️", "聊天中"},
		{"tag only", "[status:💭,想你]", "gentle", "[status:💭,想你]", "💭", "想你"},
		{"trailing space", "嗨 [status:🌙,晚安]  ", "gentle", "嗨", "🌙", "晚安"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			txt, emoji, stat := ExtractStatus(tt.input, tt.emotion)
			if txt != tt.wantTxt {
				t.Errorf("text = %q, want %q", txt, tt.wantTxt)
			}
			if emoji != tt.wantEmoji {
				t.Errorf("emoji = %q, want %q", emoji, tt.wantEmoji)
			}
			if stat != tt.wantStat {
				t.Errorf("status = %q, want %q", stat, tt.wantStat)
			}
		})
	}
}

func TestEmotionDefaultEmoji(t *testing.T) {
	cases := map[string]string{
		"happy": "😊", "love": "💕", "sad": "🌧",
		"excited": "🤩", "angry": "😤", "anxious": "😟",
		"unknown": "☕️", "": "☕️",
	}
	for emo, want := range cases {
		if got := EmotionDefaultEmoji(emo); got != want {
			t.Errorf("EmotionDefaultEmoji(%q) = %q, want %q", emo, got, want)
		}
	}
}

func TestExtractEmotionThenStatus(t *testing.T) {
	raw := "今天天气真好呀 [status:🌤,好天气] [emotion:happy]"
	txt, emo := ExtractEmotion(raw)
	if emo != "happy" {
		t.Fatalf("emotion = %q, want happy", emo)
	}
	cleaned, emoji, stat := ExtractStatus(txt, emo)
	if cleaned != "今天天气真好呀" {
		t.Errorf("cleaned = %q, want 今天天气真好呀", cleaned)
	}
	if emoji != "🌤" {
		t.Errorf("emoji = %q, want 🌤", emoji)
	}
	if stat != "好天气" {
		t.Errorf("status = %q, want 好天气", stat)
	}
}

func TestSanitizeLLMReply(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "strip think tags",
			input: "<think>先分析一下用户问题</think>\n你好呀",
			want:  "你好呀",
		},
		{
			name:  "strip tool and system tags",
			input: "<minimax:tool_call>{\"x\":1}</minimax:tool_call>\n[system:debug]\n回复内容",
			want:  "回复内容",
		},
		{
			name:  "strip markdown reasoning header",
			input: "**Thinking:**\n先想一想\n正式回复",
			want:  "先想一想\n正式回复",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := SanitizeLLMReply(tt.input); got != tt.want {
				t.Fatalf("SanitizeLLMReply() = %q, want %q", got, tt.want)
			}
		})
	}
}
