package api

import (
	"bufio"
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"nukara/backend/internal/agent"
	"nukara/backend/internal/apns"
	"nukara/backend/internal/store"
)

func TestWSChatMessageFlow(t *testing.T) {
	server, token, botID, convID, closeFn := setupTestServer(t)
	defer closeFn()

	ws := mustDialWS(t, server.URL, token)
	defer ws.Close()

	ws.SendJSON(t, map[string]any{
		"type":            "message",
		"conversation_id": convID,
		"client_msg_id":   "client-msg-1",
		"content": map[string]any{
			"type": "text",
			"text": "你好",
		},
	})

	required := map[string]bool{
		"ack":               false,
		"multi_reply_start": false,
		"message":           false,
		"multi_reply_end":   false,
		"bot_status_update": false,
	}

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		event := ws.ReadJSON(t, 2*time.Second)
		if event == nil {
			continue
		}
		eventType, _ := event["type"].(string)
		if _, ok := required[eventType]; ok {
			required[eventType] = true
		}
		if eventType == "ack" {
			if got := event["client_msg_id"]; got != "client-msg-1" {
				t.Fatalf("unexpected client_msg_id: %v", got)
			}
		}
		// Verify status extraction: message text should be cleaned (no tags)
		if eventType == "message" {
			content, _ := event["content"].(map[string]any)
			if content != nil {
				text, _ := content["text"].(string)
				if strings.Contains(text, "[status:") || strings.Contains(text, "[emotion:") {
					t.Fatalf("message text still contains tags: %s", text)
				}
			}
		}
		// Verify bot_status_update has extracted emoji/status
		if eventType == "bot_status_update" {
			emoji, _ := event["emoji"].(string)
			statusText, _ := event["text"].(string)
			if emoji == "" {
				t.Fatal("bot_status_update missing emoji")
			}
			if statusText == "" {
				t.Fatal("bot_status_update missing status text")
			}
		}

		allDone := true
		for _, done := range required {
			if !done {
				allDone = false
				break
			}
		}
		if allDone {
			break
		}
	}

	for eventType, done := range required {
		if !done {
			t.Fatalf("missing websocket event: %s", eventType)
		}
	}

	_ = botID
}

func TestWSChatDisconnectStillSavesReply(t *testing.T) {
	server, token, userID, _, convID, st, closeFn := setupTestServerWithStore(t, fakeNanobotHandler())
	defer closeFn()

	ws := mustDialWS(t, server.URL, token)
	ws.SendJSON(t, map[string]any{
		"type":            "message",
		"conversation_id": convID,
		"client_msg_id":   "client-disconnect-1",
		"content": map[string]any{
			"type": "text",
			"text": "hi",
		},
	})

	// Simulate leaving chat page before aggregation window flushes.
	time.Sleep(100 * time.Millisecond)
	ws.Close()

	deadline := time.Now().Add(6 * time.Second)
	for time.Now().Before(deadline) {
		msgs, ok := st.ListMessages(userID, convID, 0)
		if !ok {
			t.Fatalf("conversation disappeared for user=%s conv=%s", userID, convID)
		}
		for _, msg := range msgs {
			if msg.SenderType == "bot" && strings.TrimSpace(msg.Content.Text) != "" {
				return
			}
		}
		time.Sleep(100 * time.Millisecond)
	}

	msgs, _ := st.ListMessages(userID, convID, 0)
	t.Fatalf("expected bot reply saved after websocket disconnect, got %d messages", len(msgs))
}

func TestUserStatusAPI(t *testing.T) {
	server, token, _, _, closeFn := setupTestServer(t)
	defer closeFn()

	// PUT status
	body, _ := json.Marshal(map[string]string{"emoji": "😴", "text": "困了"})
	req, _ := http.NewRequest(http.MethodPut, server.URL+"/api/v1/users/status", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("PUT status failed: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("PUT status: got %d", resp.StatusCode)
	}

	// GET status
	req, _ = http.NewRequest(http.MethodGet, server.URL+"/api/v1/users/status", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET status failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("GET status: got %d", resp.StatusCode)
	}

	var result map[string]any
	json.NewDecoder(resp.Body).Decode(&result)
	if result["emoji"] != "😴" || result["text"] != "困了" {
		t.Fatalf("GET status: got %v", result)
	}
}

func TestProactiveMessageBroadcastToWS(t *testing.T) {
	server, token, botID, convID, closeFn := setupTestServer(t)
	defer closeFn()

	ws := mustDialWS(t, server.URL, token)
	defer ws.Close()

	body := map[string]string{
		"bot_id":          botID,
		"conversation_id": convID,
		"trigger_type":    "manual",
	}
	rawBody, _ := json.Marshal(body)

	req, err := http.NewRequest(http.MethodPost, server.URL+"/api/v1/gateway/test/proactive", bytes.NewReader(rawBody))
	if err != nil {
		t.Fatalf("create request failed: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("call proactive endpoint failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		data, _ := io.ReadAll(resp.Body)
		t.Fatalf("unexpected status %d body=%s", resp.StatusCode, string(data))
	}

	var proactiveResp struct {
		ShouldSend bool `json:"should_send"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&proactiveResp); err != nil {
		t.Fatalf("decode proactive response failed: %v", err)
	}
	if !proactiveResp.ShouldSend {
		t.Fatalf("expected should_send=true")
	}

	found := false
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		event := ws.ReadJSON(t, time.Second)
		if event["type"] == "proactive_message" {
			found = true
			if event["conversation_id"] != convID {
				t.Fatalf("unexpected conversation id: %v", event["conversation_id"])
			}
			break
		}
	}

	if !found {
		t.Fatalf("expected proactive_message event")
	}
}

func setupTestServer(t *testing.T) (*httptest.Server, string, string, string, func()) {
	httpServer, token, _, botID, convID, _, closeFn := setupTestServerWithStore(t, fakeNanobotHandler())
	return httpServer, token, botID, convID, closeFn
}

func setupTestServerWithStore(t *testing.T, nanobotHandler http.Handler) (*httptest.Server, string, string, string, string, *store.Store, func()) {
	t.Helper()

	fakeNanobot := httptest.NewServer(nanobotHandler)

	st := store.NewStore()
	user, err := st.CreateUser("13900139000", "tester")
	if err != nil {
		t.Fatalf("create user failed: %v", err)
	}

	bot := st.CreateBot(user.ID, store.Bot{
		Name:          "苏子衿",
		Summary:       "温柔",
		SpeakingStyle: "温柔",
		Background:    "江南",
		Traits:        []string{"体贴"},
		Gender:        "female",
	})

	conv, found := st.FindConversationByBot(user.ID, bot.ID)
	if !found {
		t.Fatalf("conversation not found")
	}

	wsURL := "ws://" + strings.TrimPrefix(fakeNanobot.URL, "http://") + "/ws/chat"
	agentClient := agent.NewAgent(agent.Config{
		NanobotHTTPURL: fakeNanobot.URL,
		NanobotWSURL:   wsURL,
	})
	if err := agentClient.Connect(); err != nil {
		t.Fatalf("agent connect failed: %v", err)
	}

	apiServer := NewServer(st, agentClient, apns.NewClient("com.nukara.app"), "test-secret", "")
	token, err := apiServer.issueToken(user.ID)
	if err != nil {
		t.Fatalf("issue token failed: %v", err)
	}

	httpServer := httptest.NewServer(apiServer.HandlerFor("gateway"))
	return httpServer, token, user.ID, bot.ID, conv.ID, st, func() {
		httpServer.Close()
		agentClient.Close()
		fakeNanobot.Close()
	}
}

type rawWSClient struct {
	conn   net.Conn
	reader *bufio.Reader
}

func mustDialWS(t *testing.T, baseURL, token string) *rawWSClient {
	t.Helper()

	parsed, err := url.Parse(baseURL)
	if err != nil {
		t.Fatalf("parse base url failed: %v", err)
	}

	conn, err := net.DialTimeout("tcp", parsed.Host, 3*time.Second)
	if err != nil {
		t.Fatalf("dial websocket failed: %v", err)
	}

	key := base64.StdEncoding.EncodeToString([]byte("nukara-test-websocket"))
	request := fmt.Sprintf("GET /ws/chat HTTP/1.1\r\nHost: %s\r\nUpgrade: websocket\r\nConnection: Upgrade\r\nSec-WebSocket-Key: %s\r\nSec-WebSocket-Version: 13\r\nAuthorization: Bearer %s\r\n\r\n", parsed.Host, key, token)
	if _, err := conn.Write([]byte(request)); err != nil {
		_ = conn.Close()
		t.Fatalf("write handshake failed: %v", err)
	}

	reader := bufio.NewReader(conn)
	statusLine, err := reader.ReadString('\n')
	if err != nil {
		_ = conn.Close()
		t.Fatalf("read handshake status failed: %v", err)
	}
	if !strings.Contains(statusLine, "101") {
		body, _ := io.ReadAll(reader)
		_ = conn.Close()
		t.Fatalf("unexpected handshake status: %s body=%s", strings.TrimSpace(statusLine), string(body))
	}

	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			_ = conn.Close()
			t.Fatalf("read handshake header failed: %v", err)
		}
		if line == "\r\n" {
			break
		}
	}

	return &rawWSClient{conn: conn, reader: reader}
}

func (c *rawWSClient) Close() {
	_ = c.conn.Close()
}

func (c *rawWSClient) SendJSON(t *testing.T, payload any) {
	t.Helper()

	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload failed: %v", err)
	}

	frame := buildClientTextFrame(raw)
	if _, err := c.conn.Write(frame); err != nil {
		t.Fatalf("send frame failed: %v", err)
	}
}

func (c *rawWSClient) ReadJSON(t *testing.T, timeout time.Duration) map[string]any {
	t.Helper()

	_ = c.conn.SetReadDeadline(time.Now().Add(timeout))

	for {
		opcode, payload, err := readServerFrame(c.reader)
		if err != nil {
			if ne, ok := err.(net.Error); ok && ne.Timeout() {
				return nil
			}
			t.Fatalf("read websocket frame failed: %v", err)
		}
		if opcode != wsOpcodeText {
			continue
		}

		var out map[string]any
		if err := json.Unmarshal(payload, &out); err != nil {
			t.Fatalf("decode websocket json failed: %v payload=%s", err, string(payload))
		}
		return out
	}
}

func buildClientTextFrame(payload []byte) []byte {
	maskKey := [4]byte{0x11, 0x22, 0x33, 0x44}

	frame := make([]byte, 0, len(payload)+14)
	frame = append(frame, 0x80|wsOpcodeText)

	payloadLen := len(payload)
	switch {
	case payloadLen < 126:
		frame = append(frame, 0x80|byte(payloadLen))
	case payloadLen <= 65535:
		frame = append(frame, 0x80|126)
		ext := make([]byte, 2)
		binary.BigEndian.PutUint16(ext, uint16(payloadLen))
		frame = append(frame, ext...)
	default:
		frame = append(frame, 0x80|127)
		ext := make([]byte, 8)
		binary.BigEndian.PutUint64(ext, uint64(payloadLen))
		frame = append(frame, ext...)
	}

	frame = append(frame, maskKey[:]...)

	masked := make([]byte, len(payload))
	copy(masked, payload)
	for i := range masked {
		masked[i] ^= maskKey[i%4]
	}
	frame = append(frame, masked...)

	return frame
}

func readServerFrame(reader *bufio.Reader) (byte, []byte, error) {
	header := make([]byte, 2)
	if _, err := io.ReadFull(reader, header); err != nil {
		return 0, nil, err
	}

	opcode := header[0] & 0x0F
	masked := (header[1] & 0x80) != 0
	payloadLen := int64(header[1] & 0x7F)

	if payloadLen == 126 {
		ext := make([]byte, 2)
		if _, err := io.ReadFull(reader, ext); err != nil {
			return 0, nil, err
		}
		payloadLen = int64(binary.BigEndian.Uint16(ext))
	} else if payloadLen == 127 {
		ext := make([]byte, 8)
		if _, err := io.ReadFull(reader, ext); err != nil {
			return 0, nil, err
		}
		payloadLen = int64(binary.BigEndian.Uint64(ext))
	}

	var maskKey [4]byte
	if masked {
		if _, err := io.ReadFull(reader, maskKey[:]); err != nil {
			return 0, nil, err
		}
	}

	payload := make([]byte, payloadLen)
	if _, err := io.ReadFull(reader, payload); err != nil {
		return 0, nil, err
	}

	if masked {
		for i := range payload {
			payload[i] ^= maskKey[i%4]
		}
	}

	return opcode, payload, nil
}

// --- Fake nanobot server for tests ---

func fakeNanobotHandler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/ws/chat", fakeNanobotWS)
	mux.HandleFunc("/chat", fakeNanobotHTTP)
	return mux
}

func fakeNanobotHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"conversation_id": "test",
		"content":         map[string]string{"type": "text", "text": "fake proactive reply"},
	})
}

func fakeNanobotWS(w http.ResponseWriter, r *http.Request) {
	hijacker, ok := w.(http.Hijacker)
	if !ok {
		http.Error(w, "hijack not supported", 500)
		return
	}
	conn, buf, err := hijacker.Hijack()
	if err != nil {
		return
	}
	defer conn.Close()

	buf.WriteString("HTTP/1.1 101 Switching Protocols\r\nUpgrade: websocket\r\nConnection: Upgrade\r\nSec-WebSocket-Accept: dummy\r\n\r\n")
	buf.Flush()

	reader := bufio.NewReader(conn)
	for {
		opcode, payload, err := readServerFrame(reader)
		if err != nil {
			return
		}
		if opcode != wsOpcodeText {
			continue
		}

		var msg map[string]any
		if err := json.Unmarshal(payload, &msg); err != nil {
			continue
		}

		if msg["type"] != "message" {
			continue
		}

		convID, _ := msg["conversation_id"].(string)
		rgID := "rg-test-1"

		writeServerJSON(conn, map[string]any{
			"type": "multi_reply_start", "conversation_id": convID,
			"reply_group_id": rgID, "count": 1,
		})
		writeServerJSON(conn, map[string]any{
			"type": "message", "conversation_id": convID,
			"reply_group_id": rgID, "sequence": 1,
			"content": map[string]string{"type": "text", "text": "你好呀～ [emotion:happy] [status:😊,开心]"},
		})
		writeServerJSON(conn, map[string]any{
			"type": "multi_reply_end", "conversation_id": convID,
			"reply_group_id": rgID,
		})
	}
}

func writeServerJSON(conn net.Conn, msg any) {
	raw, _ := json.Marshal(msg)
	frame := buildServerTextFrame(raw)
	conn.Write(frame)
}

func TestCalcDelay(t *testing.T) {
	tests := []struct {
		name     string
		msgLen   int
		expected time.Duration
	}{
		{"short message", 3, 2 * time.Second},
		{"boundary 9 chars", 9, 2 * time.Second},
		{"boundary 10 chars", 10, 1500 * time.Millisecond},
		{"medium message", 30, 1500 * time.Millisecond},
		{"boundary 50 chars", 50, 1500 * time.Millisecond},
		{"long message", 80, 1 * time.Second},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := calcDelay(tt.msgLen)
			if got != tt.expected {
				t.Fatalf("calcDelay(%d) = %v, want %v", tt.msgLen, got, tt.expected)
			}
		})
	}
}

func TestFormatAggregatedPrompt(t *testing.T) {
	t.Run("single message", func(t *testing.T) {
		got := formatAggregatedPrompt([]string{"你好"})
		if got != "你好" {
			t.Fatalf("got %q, want %q", got, "你好")
		}
	})

	t.Run("multiple messages", func(t *testing.T) {
		got := formatAggregatedPrompt([]string{"今天好累", "加班到现在", "你在干嘛"})
		if !strings.Contains(got, "[用户连续发送了3条消息]") {
			t.Fatalf("missing header, got: %s", got)
		}
		if !strings.Contains(got, "1. 今天好累") {
			t.Fatalf("missing numbered item, got: %s", got)
		}
		if !strings.Contains(got, "3. 你在干嘛") {
			t.Fatalf("missing last item, got: %s", got)
		}
	})
}

func buildServerTextFrame(payload []byte) []byte {
	frame := []byte{0x80 | wsOpcodeText}
	n := len(payload)
	switch {
	case n < 126:
		frame = append(frame, byte(n))
	case n <= 65535:
		frame = append(frame, 126)
		ext := make([]byte, 2)
		binary.BigEndian.PutUint16(ext, uint16(n))
		frame = append(frame, ext...)
	default:
		frame = append(frame, 127)
		ext := make([]byte, 8)
		binary.BigEndian.PutUint64(ext, uint64(n))
		frame = append(frame, ext...)
	}
	frame = append(frame, payload...)
	return frame
}
