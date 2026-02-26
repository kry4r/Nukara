package api

import (
	"bufio"
	"crypto/sha1"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

const (
	wsOpcodeContinuation = 0x0
	wsOpcodeText         = 0x1
	wsOpcodeBinary       = 0x2
	wsOpcodeClose        = 0x8
	wsOpcodePing         = 0x9
	wsOpcodePong         = 0xA

	wsMaxFrameSize = 4 << 20
)

var websocketGUID = "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"

type wsConn struct {
	conn      net.Conn
	reader    *bufio.Reader
	writer    *bufio.Writer
	writeMu   sync.Mutex
	closeOnce sync.Once
}

func upgradeWebSocket(w http.ResponseWriter, r *http.Request) (*wsConn, error) {
	if r.Method != http.MethodGet {
		return nil, errors.New("websocket upgrade requires GET")
	}

	if !headerContainsToken(r.Header.Get("Connection"), "upgrade") || !strings.EqualFold(strings.TrimSpace(r.Header.Get("Upgrade")), "websocket") {
		return nil, errors.New("missing websocket upgrade headers")
	}

	key := strings.TrimSpace(r.Header.Get("Sec-WebSocket-Key"))
	if key == "" {
		return nil, errors.New("missing websocket key")
	}

	hijacker, ok := w.(http.Hijacker)
	if !ok {
		return nil, errors.New("http hijacking is not supported")
	}

	netConn, rw, err := hijacker.Hijack()
	if err != nil {
		return nil, err
	}

	accept := computeWebSocketAccept(key)
	response := strings.Builder{}
	response.WriteString("HTTP/1.1 101 Switching Protocols\r\n")
	response.WriteString("Upgrade: websocket\r\n")
	response.WriteString("Connection: Upgrade\r\n")
	response.WriteString("Sec-WebSocket-Accept: ")
	response.WriteString(accept)
	response.WriteString("\r\n\r\n")

	if _, err := rw.WriteString(response.String()); err != nil {
		_ = netConn.Close()
		return nil, err
	}
	if err := rw.Flush(); err != nil {
		_ = netConn.Close()
		return nil, err
	}

	return &wsConn{
		conn:   netConn,
		reader: rw.Reader,
		writer: bufio.NewWriter(netConn),
	}, nil
}

func (c *wsConn) SetReadDeadline(t time.Time) error {
	return c.conn.SetReadDeadline(t)
}

func (c *wsConn) ReadText() (string, error) {
	for {
		opcode, payload, err := c.readFrame()
		if err != nil {
			return "", err
		}

		switch opcode {
		case wsOpcodeText:
			return string(payload), nil
		case wsOpcodeBinary, wsOpcodeContinuation:
			continue
		case wsOpcodePing:
			if err := c.writeFrame(wsOpcodePong, payload); err != nil {
				return "", err
			}
		case wsOpcodePong:
			continue
		case wsOpcodeClose:
			_ = c.writeFrame(wsOpcodeClose, nil)
			return "", io.EOF
		default:
			return "", fmt.Errorf("unsupported websocket opcode: %d", opcode)
		}
	}
}

func (c *wsConn) WriteJSON(payload any) error {
	raw, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	return c.writeFrame(wsOpcodeText, raw)
}

func (c *wsConn) Close() error {
	var err error
	c.closeOnce.Do(func() {
		err = c.conn.Close()
	})
	return err
}

func (c *wsConn) readFrame() (byte, []byte, error) {
	header := make([]byte, 2)
	if _, err := io.ReadFull(c.reader, header); err != nil {
		return 0, nil, err
	}

	fin := (header[0] & 0x80) != 0
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

	if !masked {
		return 0, nil, errors.New("client websocket frame is not masked")
	}
	if payloadLen < 0 || payloadLen > wsMaxFrameSize {
		return 0, nil, errors.New("websocket frame too large")
	}

	mask := make([]byte, 4)
	if _, err := io.ReadFull(c.reader, mask); err != nil {
		return 0, nil, err
	}

	payload := make([]byte, payloadLen)
	if _, err := io.ReadFull(c.reader, payload); err != nil {
		return 0, nil, err
	}
	for i := range payload {
		payload[i] ^= mask[i%4]
	}

	if !fin && (opcode == wsOpcodeText || opcode == wsOpcodeBinary || opcode == wsOpcodeContinuation) {
		return 0, nil, errors.New("fragmented websocket frames are not supported")
	}

	return opcode, payload, nil
}

func (c *wsConn) writeFrame(opcode byte, payload []byte) error {
	if len(payload) > wsMaxFrameSize {
		return errors.New("payload too large")
	}

	frame := make([]byte, 0, len(payload)+14)
	frame = append(frame, 0x80|opcode)

	payloadLen := len(payload)
	switch {
	case payloadLen < 126:
		frame = append(frame, byte(payloadLen))
	case payloadLen <= 65535:
		frame = append(frame, 126)
		ext := make([]byte, 2)
		binary.BigEndian.PutUint16(ext, uint16(payloadLen))
		frame = append(frame, ext...)
	default:
		frame = append(frame, 127)
		ext := make([]byte, 8)
		binary.BigEndian.PutUint64(ext, uint64(payloadLen))
		frame = append(frame, ext...)
	}

	frame = append(frame, payload...)

	c.writeMu.Lock()
	defer c.writeMu.Unlock()

	if _, err := c.writer.Write(frame); err != nil {
		return err
	}
	return c.writer.Flush()
}

func computeWebSocketAccept(key string) string {
	sum := sha1.Sum([]byte(key + websocketGUID))
	return base64.StdEncoding.EncodeToString(sum[:])
}

func headerContainsToken(value, token string) bool {
	token = strings.ToLower(strings.TrimSpace(token))
	for _, piece := range strings.Split(value, ",") {
		if strings.ToLower(strings.TrimSpace(piece)) == token {
			return true
		}
	}
	return false
}
