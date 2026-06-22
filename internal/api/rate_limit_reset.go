package api

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

const rateLimitResetConsumeURL = "https://chatgpt.com/backend-api/wham/rate-limit-reset-credits/consume"

type RateLimitResetOutcome string

const (
	RateLimitResetOutcomeReset           RateLimitResetOutcome = "reset"
	RateLimitResetOutcomeNothingToReset  RateLimitResetOutcome = "nothing_to_reset"
	RateLimitResetOutcomeNoCredit        RateLimitResetOutcome = "no_credit"
	RateLimitResetOutcomeAlreadyRedeemed RateLimitResetOutcome = "already_redeemed"
)

type RateLimitResetResult struct {
	Outcome      RateLimitResetOutcome
	WindowsReset int64
}

type consumeRateLimitResetRequest struct {
	RedeemRequestID string `json:"redeem_request_id"`
}

type consumeRateLimitResetResponse struct {
	Code         RateLimitResetOutcome `json:"code"`
	WindowsReset int64                 `json:"windows_reset"`
}

func ConsumeRateLimitResetCredit(accessToken, accountID, redeemRequestID string) (RateLimitResetResult, error) {
	accessToken = strings.TrimSpace(accessToken)
	accountID = strings.TrimSpace(accountID)
	redeemRequestID = strings.TrimSpace(redeemRequestID)
	if accessToken == "" {
		return RateLimitResetResult{}, fmt.Errorf("access token is empty")
	}
	if accountID == "" {
		return RateLimitResetResult{}, fmt.Errorf("account id is empty")
	}
	if redeemRequestID == "" {
		return RateLimitResetResult{}, fmt.Errorf("redeem request id is empty")
	}

	body, err := json.Marshal(consumeRateLimitResetRequest{RedeemRequestID: redeemRequestID})
	if err != nil {
		return RateLimitResetResult{}, fmt.Errorf("failed to encode request: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, rateLimitResetConsumeURLFromEnv(), bytes.NewReader(body))
	if err != nil {
		return RateLimitResetResult{}, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "codex-quota")
	req.Header.Set("ChatGPT-Account-Id", accountID)

	httpClient := &http.Client{Timeout: 30 * time.Second}
	resp, err := httpClient.Do(req)
	if err != nil {
		return RateLimitResetResult{}, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		body, _ := io.ReadAll(resp.Body)
		bodyText := strings.TrimSpace(string(body))
		if len(bodyText) > 500 {
			bodyText = bodyText[:500]
		}
		return RateLimitResetResult{}, &HTTPError{StatusCode: resp.StatusCode, Body: bodyText}
	}

	var payload consumeRateLimitResetResponse
	if err := decodeJSON(resp.Body, &payload); err != nil {
		return RateLimitResetResult{}, fmt.Errorf("failed to decode response: %w", err)
	}
	if !isKnownRateLimitResetOutcome(payload.Code) {
		return RateLimitResetResult{}, fmt.Errorf("unknown rate limit reset outcome: %q", payload.Code)
	}

	return RateLimitResetResult{Outcome: payload.Code, WindowsReset: payload.WindowsReset}, nil
}

func rateLimitResetConsumeURLFromEnv() string {
	if value := strings.TrimSpace(os.Getenv("CQ_RATE_LIMIT_RESET_CONSUME_URL")); value != "" {
		return value
	}
	return rateLimitResetConsumeURL
}

func isKnownRateLimitResetOutcome(outcome RateLimitResetOutcome) bool {
	switch outcome {
	case RateLimitResetOutcomeReset,
		RateLimitResetOutcomeNothingToReset,
		RateLimitResetOutcomeNoCredit,
		RateLimitResetOutcomeAlreadyRedeemed:
		return true
	default:
		return false
	}
}
