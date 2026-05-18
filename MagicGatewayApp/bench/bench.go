package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

// Usage: go run bench.go <direct|proxy> <api-key>
//
// Sends a single chat request and prints timing breakdown as JSON.
// Designed to be called by the benchmark shell script for A/B comparison.
//
//   direct: calls DeepSeek API directly
//   proxy:  calls through MagicGateway

const (
	deepSeekURL = "https://api.deepseek.com/anthropic/v1/messages"
)

var requestBody = []byte(`{
	"model": "deepseek-chat",
	"max_tokens": 512,
	"stream": true,
	"messages": [
		{"role": "user", "content": "Explain the difference between TCP and UDP in one paragraph."}
	]
}`)

func main() {
	if len(os.Args) < 3 {
		fmt.Fprintf(os.Stderr, "Usage: bench <direct|proxy> <api-key> [gateway-url]\n")
		os.Exit(1)
	}

	mode := os.Args[1]
	apiKey := os.Args[2]

	var targetURL string
	if mode == "direct" {
		targetURL = deepSeekURL
	} else {
		if len(os.Args) >= 4 {
			targetURL = strings.TrimRight(os.Args[3], "/") + "/v1/messages"
		} else {
			targetURL = "http://localhost:8080/v1/messages"
		}
	}

	result := run(targetURL, apiKey)
	out, _ := json.Marshal(result)
	fmt.Println(string(out))
}

type TimingResult struct {
	Mode             string  `json:"mode"`
	StatusCode       int     `json:"status_code"`
	TTFTMs           float64 `json:"ttft_ms"`
	TotalTimeMs      float64 `json:"total_time_ms"`
	TotalBytes       int     `json:"total_bytes"`
	InputTokens      int     `json:"input_tokens"`
	OutputTokens     int     `json:"output_tokens"`
	TokensPerSecond  float64 `json:"tokens_per_second"`
	Error            string  `json:"error,omitempty"`
}

func run(url, apiKey string) TimingResult {
	r := TimingResult{Mode: "unknown"}

	if strings.Contains(url, "api.deepseek.com") {
		r.Mode = "direct"
	} else {
		r.Mode = "proxy"
	}

	req, _ := http.NewRequest("POST", url, bytes.NewReader(requestBody))
	req.Header.Set("Content-Type", "application/json")
	if r.Mode == "proxy" {
		req.Header.Set("x-api-key", apiKey)
	} else {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}
	req.Header.Set("anthropic-version", "2023-06-01")

	t0 := time.Now()
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		r.Error = err.Error()
		r.TotalTimeMs = float64(time.Since(t0).Microseconds()) / 1000.0
		return r
	}
	defer resp.Body.Close()

	r.StatusCode = resp.StatusCode

	// Stream read with TTFT measurement
	firstChunk := true
	var firstTokenTime time.Time
	buf := make([]byte, 4096)
	var allBytes bytes.Buffer

	for {
		n, readErr := resp.Body.Read(buf)
		if n > 0 {
			if firstChunk {
				firstTokenTime = time.Now()
				firstChunk = false
			}
			allBytes.Write(buf[:n])
		}
		if readErr != nil {
			break
		}
	}

	r.TotalTimeMs = float64(time.Since(t0).Microseconds()) / 1000.0
	if !firstTokenTime.IsZero() {
		r.TTFTMs = float64(firstTokenTime.Sub(t0).Microseconds()) / 1000.0
	}
	r.TotalBytes = allBytes.Len()

	// Parse usage from SSE or JSON response
	parseUsage(allBytes.Bytes(), &r)

	if r.OutputTokens > 0 && r.TTFTMs > 0 {
		genTime := r.TotalTimeMs - r.TTFTMs
		if genTime > 0 {
			r.TokensPerSecond = float64(r.OutputTokens) / (genTime / 1000.0)
		}
	}

	return r
}

func parseUsage(data []byte, r *TimingResult) {
	// Try JSON first (non-streaming)
	var jsonResp struct {
		Usage struct {
			InputTokens  int `json:"input_tokens"`
			OutputTokens int `json:"output_tokens"`
		} `json:"usage"`
	}
	if json.Unmarshal(data, &jsonResp) == nil && jsonResp.Usage.InputTokens > 0 {
		r.InputTokens = jsonResp.Usage.InputTokens
		r.OutputTokens = jsonResp.Usage.OutputTokens
		return
	}

	// Parse SSE events
	events := bytes.Split(data, []byte("\n\n"))
	for _, event := range events {
		for _, line := range bytes.Split(event, []byte("\n")) {
			if !bytes.HasPrefix(line, []byte("data: ")) {
				continue
			}
			jsonData := bytes.TrimPrefix(line, []byte("data: "))
			var evt map[string]interface{}
			if json.Unmarshal(jsonData, &evt) != nil {
				continue
			}

			usage, ok := evt["usage"].(map[string]interface{})
			if !ok {
				continue
			}
			if it, ok := usage["input_tokens"].(float64); ok {
				r.InputTokens = int(it)
			}
			if ot, ok := usage["output_tokens"].(float64); ok {
				r.OutputTokens = int(ot)
			}
		}
	}
}
