package proxy

import (
	"bytes"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"magicgateway/auth"
	"magicgateway/store"
)

type Proxy struct {
	Store       *store.Store
	DeepSeekURL string
	DeepSeekKey string
	client      *http.Client
}

func New(st *store.Store, baseURL, apiKey string) *Proxy {
	return &Proxy{
		Store:       st,
		DeepSeekURL: strings.TrimRight(baseURL, "/"),
		DeepSeekKey: apiKey,
		client: &http.Client{
			Timeout: 5 * time.Minute,
		},
	}
}

func (p *Proxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeProxyError(w, http.StatusMethodNotAllowed, "only POST allowed")
		return
	}

	// 1. Extract and validate virtual API key
	apiKey := extractAPIKey(r)
	if apiKey == "" {
		writeProxyError(w, http.StatusUnauthorized, "missing api key")
		return
	}

	keyHash := auth.HashAPIKey(apiKey)
	keyRecord, err := p.Store.GetKeyByHash(keyHash)
	if err != nil || keyRecord == nil || !keyRecord.IsActive {
		writeProxyError(w, http.StatusUnauthorized, "invalid or revoked api key")
		return
	}

	user, err := p.Store.GetUserByID(keyRecord.UserID)
	if err != nil || user == nil {
		writeProxyError(w, http.StatusInternalServerError, "user not found")
		return
	}

	// 2. Read the original request body
	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		writeProxyError(w, http.StatusBadRequest, "failed to read request body")
		return
	}
	r.Body.Close()

	// 3. Build request to DeepSeek
	targetURL := p.DeepSeekURL + "/v1/messages"
	req, err := http.NewRequest("POST", targetURL, bytes.NewReader(bodyBytes))
	if err != nil {
		writeProxyError(w, http.StatusInternalServerError, "failed to create upstream request")
		return
	}

	req.Header.Set("Authorization", "Bearer "+p.DeepSeekKey)
	req.Header.Set("Content-Type", "application/json")

	// Forward relevant Anthropic headers
	for _, h := range []string{"anthropic-version", "anthropic-beta"} {
		if v := r.Header.Get(h); v != "" {
			req.Header.Set(h, v)
		}
	}

	// 4. Execute request
	startTime := time.Now()
	resp, err := p.client.Do(req)
	if err != nil {
		log.Printf("[proxy] upstream error: %v", err)
		writeProxyError(w, http.StatusBadGateway, "upstream request failed")
		return
	}
	defer resp.Body.Close()

	ttfbMs := time.Since(startTime).Milliseconds()

	// Capture request-id from response header
	requestID := resp.Header.Get("request-id")

	// 5. Detect streaming vs non-streaming
	isStreaming := isStreamRequest(bodyBytes)

	// 6. Handle response
	var model string
	var inputTokens, outputTokens int

	if isStreaming {
		model, inputTokens, outputTokens = p.handleStream(w, resp)
	} else {
		model, inputTokens, outputTokens = p.handleNonStream(w, resp)
	}

	durationMs := time.Since(startTime).Milliseconds()

	// 7. Log usage (only for successful upstream responses)
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		if err := p.Store.InsertUsage(
			keyRecord.ID, user.ID, user.Username,
			model, requestID, inputTokens, outputTokens, durationMs,
		); err != nil {
			log.Printf("[proxy] failed to log usage: %v", err)
		}
	}

	log.Printf("[proxy] user=%s model=%s ttfb=%dms total=%dms in=%d out=%d",
		user.Username, model, ttfbMs, durationMs, inputTokens, outputTokens)
}

// ---- Streaming handler ----

func (p *Proxy) handleStream(w http.ResponseWriter, resp *http.Response) (model string, inputTokens, outputTokens int) {
	// Copy headers from upstream
	for k, vs := range resp.Header {
		for _, v := range vs {
			w.Header().Add(k, v)
		}
	}
	w.WriteHeader(resp.StatusCode)

	flusher, ok := w.(http.Flusher)
	if !ok {
		io.Copy(w, resp.Body)
		return
	}

	var buf bytes.Buffer
	buf.Grow(65536)

	chunk := make([]byte, 4096)
	for {
		n, err := resp.Body.Read(chunk)
		if n > 0 {
			buf.Write(chunk[:n])
			w.Write(chunk[:n])
			flusher.Flush()
		}
		if err != nil {
			break
		}
	}

	model, inputTokens, outputTokens = extractUsage(buf.Bytes())
	return
}

// ---- Non-streaming handler ----

func (p *Proxy) handleNonStream(w http.ResponseWriter, resp *http.Response) (model string, inputTokens, outputTokens int) {
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		writeProxyError(w, http.StatusBadGateway, "failed to read upstream response")
		return
	}

	// Extract usage from JSON response
	model, inputTokens, outputTokens = extractUsageJSON(bodyBytes)

	// Forward headers and body
	for k, vs := range resp.Header {
		for _, v := range vs {
			w.Header().Add(k, v)
		}
	}
	w.WriteHeader(resp.StatusCode)
	w.Write(bodyBytes)
	return
}

// ---- Helpers ----

func extractAPIKey(r *http.Request) string {
	if key := r.Header.Get("x-api-key"); key != "" {
		return key
	}
	auth := r.Header.Get("Authorization")
	if strings.HasPrefix(auth, "Bearer ") {
		return auth[7:]
	}
	return ""
}

func isStreamRequest(body []byte) bool {
	var req struct {
		Stream bool `json:"stream"`
	}
	json.Unmarshal(body, &req)
	return req.Stream
}

func extractUsage(data []byte) (model string, inputTokens, outputTokens int) {
	events := bytes.Split(data, []byte("\n\n"))
	for _, event := range events {
		eventData := parseSSEEvent(event)
		if eventData == nil {
			continue
		}

		if m, ok := eventData["model"].(string); ok && m != "" {
			model = m
		}

		eventType, _ := eventData["type"].(string)

		// input_tokens (including cache tokens) are in message_start
		if eventType == "message_start" {
			if msg, ok := eventData["message"].(map[string]interface{}); ok {
				if usage, ok := msg["usage"].(map[string]interface{}); ok {
					inputTokens += extractTokenCount(usage, "input_tokens")
					inputTokens += extractTokenCount(usage, "cache_creation_input_tokens")
					inputTokens += extractTokenCount(usage, "cache_read_input_tokens")
				}
			}
		}

		// output_tokens accumulate in message_delta
		if eventType == "message_delta" {
			if usage, ok := eventData["usage"].(map[string]interface{}); ok {
				outputTokens = extractTokenCount(usage, "output_tokens")
			}
		}
	}
	return
}

func extractTokenCount(usage map[string]interface{}, key string) int {
	if v, ok := usage[key].(float64); ok {
		return int(v)
	}
	return 0
}

func extractUsageJSON(data []byte) (model string, inputTokens, outputTokens int) {
	var resp map[string]interface{}
	if err := json.Unmarshal(data, &resp); err != nil {
		return
	}

	if m, ok := resp["model"].(string); ok {
		model = m
	}
	if usage, ok := resp["usage"].(map[string]interface{}); ok {
		inputTokens += extractTokenCount(usage, "input_tokens")
		inputTokens += extractTokenCount(usage, "cache_creation_input_tokens")
		inputTokens += extractTokenCount(usage, "cache_read_input_tokens")
		outputTokens = extractTokenCount(usage, "output_tokens")
	}
	return
}

func parseSSEEvent(event []byte) map[string]interface{} {
	for _, line := range bytes.Split(event, []byte("\n")) {
		if bytes.HasPrefix(line, []byte("data: ")) {
			jsonData := bytes.TrimPrefix(line, []byte("data: "))
			var parsed map[string]interface{}
			if err := json.Unmarshal(jsonData, &parsed); err == nil {
				return parsed
			}
		}
	}
	return nil
}

func writeProxyError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	resp := map[string]interface{}{
		"type": "error",
		"error": map[string]string{
			"type":    "api_error",
			"message": message,
		},
	}
	json.NewEncoder(w).Encode(resp)
}
