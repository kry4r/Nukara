package agent

import (
	"bufio"
	"crypto/rand"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/url"
	"strings"
	"sync"
	"time"
)

// WebSocket frame opcodes and limits.
const (
	wsOpcodeText  byte = 1
	wsOpcodePing  byte = 9
	wsOpcodePong  byte = 10
	wsOpcodeClose byte = 8

	wsMaxFrameSize int64 = 4 * 1024 * 1024 // 4 MB
)

// nanobotWSClient is a WebSocket client for the nanobot extend-chat channel.
// It maintains a persistent connection and multiplexes conversations.
type nanobotWSClient struct {
	wsURL  string
	token  string
	conn   net.Conn
	reader *bufio.Reader
	writer *bufio.Writer
	mu     sync.Mutex // protects writes

	subs   map[string][]chan NanobotEvent // conversation_id → listeners
	subsMu sync.RWMutex

	closed   chan struct{}
	closeMu  sync.Mutex
	isClosed bool
}

func newNanobotWSClient(wsURL, token string) *nanobotWSClient {
	return &nanobotWSClient{
		wsURL:  wsURL,
		token:  token,
		subs:   make(map[string][]chan NanobotEvent),
		closed: make(chan struct{}),
	}
}

// Connect establishes the WebSocket connection and starts the read loop.
// If the initial dial fails, it starts reconnecting in the background.
func (c *nanobotWSClient) Connect() error {
	if err := c.dial(); err != nil {
		log.Printf("[nanobot-ws] initial connect failed, will retry: %v", err)
		go c.reconnectLoop()
		go c.pingLoop()
		return err
	}
	go c.readLoop()
	go c.pingLoop()
	return nil
}

// Subscribe registers a channel to receive events for a conversation.
func (c *nanobotWSClient) Subscribe(convID string) <-chan NanobotEvent {
	ch := make(chan NanobotEvent, 64)
	c.subsMu.Lock()
	c.subs[convID] = append(c.subs[convID], ch)
	c.subsMu.Unlock()
	return ch
}

// Unsubscribe removes a channel from a conversation's listeners.
func (c *nanobotWSClient) Unsubscribe(convID string, ch <-chan NanobotEvent) {
	c.subsMu.Lock()
	defer c.subsMu.Unlock()
	listeners := c.subs[convID]
	for i, l := range listeners {
		if l == ch {
			c.subs[convID] = append(listeners[:i], listeners[i+1:]...)
			close(l)
			break
		}
	}
	if len(c.subs[convID]) == 0 {
		delete(c.subs, convID)
	}
}

// Send sends a JSON message over the WebSocket.
func (c *nanobotWSClient) Send(msg any) error {
	raw, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	return c.writeClientFrame(wsOpcodeText, raw)
}

// Close closes the WebSocket connection.
func (c *nanobotWSClient) Close() {
	c.closeMu.Lock()
	defer c.closeMu.Unlock()
	if c.isClosed {
		return
	}
	c.isClosed = true
	close(c.closed)
	if c.conn != nil {
		_ = c.conn.Close()
	}
}

// dial performs the WebSocket handshake as a client.
func (c *nanobotWSClient) dial() error {
	u, err := url.Parse(c.wsURL)
	if err != nil {
		return fmt.Errorf("parse ws url: %w", err)
	}

	host := u.Host
	if !strings.Contains(host, ":") {
		if u.Scheme == "wss" {
			host += ":443"
		} else {
			host += ":80"
		}
	}

	conn, err := net.DialTimeout("tcp", host, 10*time.Second)
	if err != nil {
		return fmt.Errorf("dial %s: %w", host, err)
	}

	// Generate WebSocket key
	keyBytes := make([]byte, 16)
	_, _ = rand.Read(keyBytes)
	wsKey := base64.StdEncoding.EncodeToString(keyBytes)

	path := u.Path
	if u.RawQuery != "" {
		path += "?" + u.RawQuery
	}
	if c.token != "" {
		sep := "?"
		if strings.Contains(path, "?") {
			sep = "&"
		}
		path += sep + "token=" + url.QueryEscape(c.token)
	}

	// Send upgrade request
	req := fmt.Sprintf(
		"GET %s HTTP/1.1\r\nHost: %s\r\nUpgrade: websocket\r\nConnection: Upgrade\r\nSec-WebSocket-Key: %s\r\nSec-WebSocket-Version: 13\r\n\r\n",
		path, u.Host, wsKey,
	)
	if _, err := conn.Write([]byte(req)); err != nil {
		_ = conn.Close()
		return fmt.Errorf("write upgrade: %w", err)
	}

	// Read response
	reader := bufio.NewReader(conn)
	statusLine, err := reader.ReadString('\n')
	if err != nil {
		_ = conn.Close()
		return fmt.Errorf("read status: %w", err)
	}
	if !strings.Contains(statusLine, "101") {
		_ = conn.Close()
		return fmt.Errorf("upgrade rejected: %s", strings.TrimSpace(statusLine))
	}

	// Skip headers
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			_ = conn.Close()
			return fmt.Errorf("read headers: %w", err)
		}
		if strings.TrimSpace(line) == "" {
			break
		}
	}

	c.conn = conn
	c.reader = reader
	c.writer = bufio.NewWriter(conn)
	return nil
}

// readLoop continuously reads events from the WebSocket and dispatches them.
func (c *nanobotWSClient) readLoop() {
	defer c.reconnectLoop()

	for {
		select {
		case <-c.closed:
			return
		default:
		}

		_ = c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		opcode, payload, err := c.readServerFrame()
		if err != nil {
			if errors.Is(err, io.EOF) || errors.Is(err, net.ErrClosed) {
				return
			}
			if ne, ok := err.(net.Error); ok && ne.Timeout() {
				continue // read timeout, will ping
			}
			log.Printf("[nanobot-ws] read error: %v", err)
			return
		}

		switch opcode {
		case wsOpcodeText:
			var evt NanobotEvent
			if err := json.Unmarshal(payload, &evt); err != nil {
				log.Printf("[nanobot-ws] unmarshal error: %v", err)
				continue
			}
			c.dispatch(evt)
		case wsOpcodePing:
			_ = c.writeClientFrame(wsOpcodePong, payload)
		case wsOpcodePong:
			// ok
		case wsOpcodeClose:
			return
		}
	}
}

// dispatch sends an event to all subscribers of the conversation.
func (c *nanobotWSClient) dispatch(evt NanobotEvent) {
	c.subsMu.RLock()
	listeners := c.subs[evt.ConversationID]
	c.subsMu.RUnlock()

	for _, ch := range listeners {
		select {
		case ch <- evt:
		default:
			// drop if full
		}
	}
}

// reconnectLoop attempts to reconnect with exponential backoff.
func (c *nanobotWSClient) reconnectLoop() {
	backoff := time.Second
	maxBackoff := 30 * time.Second

	for {
		select {
		case <-c.closed:
			return
		default:
		}

		log.Printf("[nanobot-ws] reconnecting in %s...", backoff)
		time.Sleep(backoff)

		if err := c.dial(); err != nil {
			log.Printf("[nanobot-ws] reconnect failed: %v", err)
			backoff = min(backoff*2, maxBackoff)
			continue
		}

		log.Printf("[nanobot-ws] reconnected")
		backoff = time.Second
		go c.readLoop()
		return
	}
}

// pingLoop sends periodic pings to keep the connection alive.
func (c *nanobotWSClient) pingLoop() {
	ticker := time.NewTicker(25 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-c.closed:
			return
		case <-ticker.C:
			c.mu.Lock()
			connected := c.writer != nil
			c.mu.Unlock()
			if connected {
				_ = c.Send(map[string]string{"type": "ping"})
			}
		}
	}
}

// readServerFrame reads a WebSocket frame from the server (unmasked).
func (c *nanobotWSClient) readServerFrame() (byte, []byte, error) {
	header := make([]byte, 2)
	if _, err := io.ReadFull(c.reader, header); err != nil {
		return 0, nil, err
	}

	opcode := header[0] & 0x0F
	masked := (header[1] & 0x80) != 0
	payloadLen := int64(header[1] & 0x7F)

	if payloadLen == 126 {
		ext := make([]byte, 2)
		if _, err := io.ReadFull(c.reader, ext); err != nil {
			return 0, nil, err
		}
		payloadLen = int64(binary.BigEndian.Uint16(ext))
	} else if payloadLen == 127 {
		ext := make([]byte, 8)
		if _, err := io.ReadFull(c.reader, ext); err != nil {
			return 0, nil, err
		}
		payloadLen = int64(binary.BigEndian.Uint64(ext))
	}

	if payloadLen < 0 || payloadLen > wsMaxFrameSize {
		return 0, nil, errors.New("frame too large")
	}

	var mask []byte
	if masked {
		mask = make([]byte, 4)
		if _, err := io.ReadFull(c.reader, mask); err != nil {
			return 0, nil, err
		}
	}

	payload := make([]byte, payloadLen)
	if _, err := io.ReadFull(c.reader, payload); err != nil {
		return 0, nil, err
	}
	if masked {
		for i := range payload {
			payload[i] ^= mask[i%4]
		}
	}

	return opcode, payload, nil
}

// writeClientFrame writes a masked WebSocket frame (client → server must mask).
func (c *nanobotWSClient) writeClientFrame(opcode byte, payload []byte) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.writer == nil {
		return errors.New("not connected")
	}

	mask := make([]byte, 4)
	_, _ = rand.Read(mask)

	// Header
	frame := []byte{0x80 | opcode}

	payloadLen := len(payload)
	switch {
	case payloadLen < 126:
		frame = append(frame, 0x80|byte(payloadLen)) // mask bit set
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

	frame = append(frame, mask...)

	// Mask payload
	masked := make([]byte, payloadLen)
	copy(masked, payload)
	for i := range masked {
		masked[i] ^= mask[i%4]
	}
	frame = append(frame, masked...)

	if _, err := c.writer.Write(frame); err != nil {
		return err
	}
	return c.writer.Flush()
}

// nanobotWSPool maintains multiple WS connections with sticky routing.
type nanobotWSPool struct {
	clients []*nanobotWSClient
	size    int
}

func newNanobotWSPool(wsURL, token string, size int) *nanobotWSPool {
	if size < 1 {
		size = 1
	}
	clients := make([]*nanobotWSClient, size)
	for i := range clients {
		clients[i] = newNanobotWSClient(wsURL, token)
	}
	return &nanobotWSPool{clients: clients, size: size}
}

func (p *nanobotWSPool) pick(convID string) *nanobotWSClient {
	// FNV-1a hash for sticky routing
	h := uint32(2166136261)
	for i := 0; i < len(convID); i++ {
		h ^= uint32(convID[i])
		h *= 16777619
	}
	return p.clients[h%uint32(p.size)]
}

func (p *nanobotWSPool) Connect() error {
	var firstErr error
	for _, c := range p.clients {
		if err := c.Connect(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

func (p *nanobotWSPool) Close() {
	for _, c := range p.clients {
		c.Close()
	}
}

func (p *nanobotWSPool) Subscribe(convID string) <-chan NanobotEvent {
	return p.pick(convID).Subscribe(convID)
}

func (p *nanobotWSPool) Unsubscribe(convID string, ch <-chan NanobotEvent) {
	p.pick(convID).Unsubscribe(convID, ch)
}

func (p *nanobotWSPool) Send(convID string, msg any) error {
	return p.pick(convID).Send(msg)
}
