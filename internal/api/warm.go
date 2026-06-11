package api

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

const codexResponsesURL = "https://chatgpt.com/backend-api/codex/responses"

type warmRequest struct {
	Model             string         `json:"model"`
	Instructions      string         `json:"instructions"`
	Input             []warmInput    `json:"input"`
	Tools             []any          `json:"tools"`
	ToolChoice        string         `json:"tool_choice"`
	ParallelToolCalls bool           `json:"parallel_tool_calls"`
	Reasoning         map[string]any `json:"reasoning"`
	Store             bool           `json:"store"`
	Stream            bool           `json:"stream"`
	Include           []string       `json:"include"`
}

type warmInput struct {
	Type    string        `json:"type"`
	Role    string        `json:"role"`
	Content []warmContent `json:"content"`
}

type warmContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

func WarmCodex(accessToken, accountID string) error {
	accessToken = strings.TrimSpace(accessToken)
	accountID = strings.TrimSpace(accountID)
	if accessToken == "" {
		return fmt.Errorf("access token is empty")
	}
	if accountID == "" {
		return fmt.Errorf("account id is empty")
	}

	body, err := json.Marshal(newWarmRequest())
	if err != nil {
		return fmt.Errorf("failed to encode request: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, codexResponsesURLFromEnv(), bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("ChatGPT-Account-Id", accountID)
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "codex-cli")
	req.Header.Set("session-id", randomHexID())
	req.Header.Set("thread-id", randomHexID())

	client := &http.Client{Timeout: 45 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		bodyText := strings.TrimSpace(string(body))
		if len(bodyText) > 500 {
			bodyText = bodyText[:500]
		}
		return &HTTPError{StatusCode: resp.StatusCode, Body: bodyText}
	}

	_, _ = io.Copy(io.Discard, resp.Body)
	return nil
}

func newWarmRequest() warmRequest {
	reply := randomWarmReply()
	return warmRequest{
		Model:        "gpt-5.4-mini",
		Instructions: "Reply exactly: " + reply,
		Input: []warmInput{{
			Type: "message",
			Role: "user",
			Content: []warmContent{{
				Type: "input_text",
				Text: "Reply with exactly: " + reply,
			}},
		}},
		Tools:             []any{},
		ToolChoice:        "auto",
		ParallelToolCalls: false,
		Reasoning:         nil,
		Store:             false,
		Stream:            true,
		Include:           []string{},
	}
}

func randomWarmReply() string {
	return "ok-" + randomHexString(3)
}

func codexResponsesURLFromEnv() string {
	if value := strings.TrimSpace(os.Getenv("CQ_CODEX_RESPONSES_URL")); value != "" {
		return value
	}
	return codexResponsesURL
}

func randomHexID() string {
	return randomHexString(16)
}

func randomHexString(byteCount int) string {
	if byteCount <= 0 {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}

	b := make([]byte, byteCount)
	if _, err := rand.Read(b); err != nil {
		fallback := fmt.Sprintf("%x", time.Now().UnixNano())
		needed := byteCount * 2
		if len(fallback) >= needed {
			return fallback[:needed]
		}
		return fallback
	}
	return hex.EncodeToString(b)
}
