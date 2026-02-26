package apns

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"log"
	"math/big"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

type Client struct {
	Topic string

	keyID   string
	teamID  string
	key     *ecdsa.PrivateKey
	baseURL string

	httpClient *http.Client

	tokenMu   sync.Mutex
	cachedJWT string
	tokenExp  time.Time

	stub bool
}

func NewClient(topic string) *Client {
	client := &Client{Topic: topic}

	keyID := strings.TrimSpace(os.Getenv("NUKARA_APNS_KEY_ID"))
	teamID := strings.TrimSpace(os.Getenv("NUKARA_APNS_TEAM_ID"))
	keyPath := strings.TrimSpace(os.Getenv("NUKARA_APNS_P8_PATH"))
	keyBase64 := strings.TrimSpace(os.Getenv("NUKARA_APNS_P8_BASE64"))

	if strings.TrimSpace(topic) == "" || keyID == "" || teamID == "" || (keyPath == "" && keyBase64 == "") {
		client.stub = true
		return client
	}

	privateKey, err := loadPrivateKey(keyPath, keyBase64)
	if err != nil {
		log.Printf("apns production key load failed, fallback to stub: %v", err)
		client.stub = true
		return client
	}

	baseURL := "https://api.push.apple.com"
	if strings.EqualFold(strings.TrimSpace(os.Getenv("NUKARA_APNS_SANDBOX")), "true") {
		baseURL = "https://api.sandbox.push.apple.com"
	}

	client.keyID = keyID
	client.teamID = teamID
	client.key = privateKey
	client.baseURL = baseURL
	client.httpClient = &http.Client{
		Timeout: 12 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig:   &tls.Config{MinVersion: tls.VersionTLS12},
			ForceAttemptHTTP2: true,
		},
	}
	return client
}

func (c *Client) Send(deviceToken, title, body, conversationID string) error {
	deviceToken = strings.TrimSpace(deviceToken)
	if deviceToken == "" {
		return errors.New("apns device token is empty")
	}

	if c.stub {
		log.Printf("[APNs stub] token=%s title=%s body=%s conversation=%s", deviceToken, title, body, conversationID)
		return nil
	}

	token, err := c.providerJWT()
	if err != nil {
		return err
	}

	notification := map[string]any{
		"aps": map[string]any{
			"alert": map[string]any{
				"title": title,
				"body":  body,
			},
			"sound": "default",
		},
		"conversation_id": conversationID,
	}
	raw, _ := json.Marshal(notification)

	url := fmt.Sprintf("%s/3/device/%s", c.baseURL, deviceToken)
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(raw))
	if err != nil {
		return err
	}
	req.Header.Set("authorization", "bearer "+token)
	req.Header.Set("apns-topic", c.Topic)
	req.Header.Set("apns-push-type", "alert")
	req.Header.Set("apns-priority", "10")
	req.Header.Set("content-type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}

	var payload struct {
		Reason string `json:"reason"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&payload)
	if payload.Reason == "ExpiredProviderToken" {
		c.tokenMu.Lock()
		c.cachedJWT = ""
		c.tokenExp = time.Time{}
		c.tokenMu.Unlock()
	}
	if payload.Reason == "" {
		payload.Reason = "unknown"
	}
	return fmt.Errorf("apns status=%d reason=%s", resp.StatusCode, payload.Reason)
}

func (c *Client) providerJWT() (string, error) {
	c.tokenMu.Lock()
	defer c.tokenMu.Unlock()

	now := time.Now().UTC()
	if c.cachedJWT != "" && now.Before(c.tokenExp) {
		return c.cachedJWT, nil
	}

	header := map[string]any{"alg": "ES256", "kid": c.keyID}
	claims := map[string]any{"iss": c.teamID, "iat": now.Unix()}

	headerRaw, _ := json.Marshal(header)
	claimsRaw, _ := json.Marshal(claims)
	message := base64.RawURLEncoding.EncodeToString(headerRaw) + "." + base64.RawURLEncoding.EncodeToString(claimsRaw)

	hash := sha256.Sum256([]byte(message))
	r, s, err := ecdsa.Sign(rand.Reader, c.key, hash[:])
	if err != nil {
		return "", err
	}

	signature, err := marshalJOSESignature(r, s)
	if err != nil {
		return "", err
	}

	jwt := message + "." + base64.RawURLEncoding.EncodeToString(signature)
	c.cachedJWT = jwt
	c.tokenExp = now.Add(50 * time.Minute)
	return jwt, nil
}

func marshalJOSESignature(r, s *big.Int) ([]byte, error) {
	rb := r.Bytes()
	sb := s.Bytes()
	if len(rb) > 32 || len(sb) > 32 {
		return nil, errors.New("invalid ecdsa signature size")
	}
	raw := make([]byte, 64)
	copy(raw[32-len(rb):32], rb)
	copy(raw[64-len(sb):], sb)
	return raw, nil
}

func loadPrivateKey(path, encoded string) (*ecdsa.PrivateKey, error) {
	var pemBytes []byte
	var err error
	if strings.TrimSpace(path) != "" {
		pemBytes, err = os.ReadFile(path)
		if err != nil {
			return nil, err
		}
	} else {
		pemBytes, err = base64.StdEncoding.DecodeString(encoded)
		if err != nil {
			return nil, err
		}
	}

	block, _ := pem.Decode(pemBytes)
	if block == nil {
		return nil, errors.New("invalid p8 key")
	}

	key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, err
	}
	ecdsaKey, ok := key.(*ecdsa.PrivateKey)
	if !ok {
		return nil, errors.New("p8 key is not ecdsa")
	}
	return ecdsaKey, nil
}
