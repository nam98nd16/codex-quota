package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

const usageURL = "https://chatgpt.com/backend-api/wham/usage"

type UsageData struct {
	PlanType                       string
	Allowed                        bool
	LimitReached                   bool
	Windows                        []QuotaWindow
	AvailableRateLimitResetCredits *int64
}

type QuotaWindow struct {
	Label       string
	UsedPercent float64
	LeftPercent float64
	ResetAt     time.Time
	WindowSec   int64
}

type HTTPError struct {
	StatusCode int
	Body       string
}

type usageResponse struct {
	PlanType              string                        `json:"plan_type"`
	RateLimit             rateLimitStatus               `json:"rate_limit"`
	RateLimitResetCredits *RateLimitResetCreditsSummary `json:"rate_limit_reset_credits"`
}

type RateLimitResetCreditsSummary struct {
	AvailableCount int64 `json:"available_count"`
}

type rateLimitStatus struct {
	Allowed       bool            `json:"allowed"`
	LimitReached  bool            `json:"limit_reached"`
	PrimaryWindow *windowSnapshot `json:"primary_window"`
	Secondary     *windowSnapshot `json:"secondary_window"`
}

type windowSnapshot struct {
	LimitWindowSeconds int64   `json:"limit_window_seconds"`
	UsedPercent        float64 `json:"used_percent"`
	ResetAt            int64   `json:"reset_at"`
}

func (e *HTTPError) Error() string {
	body := strings.TrimSpace(e.Body)
	if body == "" {
		return fmt.Sprintf("request failed with status %d", e.StatusCode)
	}
	return fmt.Sprintf("request failed with status %d: %s", e.StatusCode, body)
}

func IsUnauthorized(err error) bool {
	httpErr, ok := err.(*HTTPError)
	if !ok {
		return false
	}
	return httpErr.StatusCode == http.StatusUnauthorized || httpErr.StatusCode == http.StatusForbidden
}

func CallAPI(accessToken, accountID string) (UsageData, error) {
	req, err := http.NewRequest(http.MethodGet, usageURLFromEnv(), nil)
	if err != nil {
		return UsageData{}, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "codex-quota")
	if accountID != "" {
		req.Header.Set("ChatGPT-Account-Id", accountID)
	}

	httpClient := &http.Client{Timeout: 30 * time.Second}
	resp, err := httpClient.Do(req)
	if err != nil {
		return UsageData{}, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		bodyText := strings.TrimSpace(string(body))
		if len(bodyText) > 500 {
			bodyText = bodyText[:500]
		}
		return UsageData{}, &HTTPError{StatusCode: resp.StatusCode, Body: bodyText}
	}

	var payload usageResponse
	if err := decodeJSON(resp.Body, &payload); err != nil {
		return UsageData{}, fmt.Errorf("failed to decode response: %w", err)
	}

	windows := make([]QuotaWindow, 0, 2)
	if payload.RateLimit.PrimaryWindow != nil {
		windows = append(windows, mapWindow(payload.RateLimit.PrimaryWindow, "primary"))
	}
	if payload.RateLimit.Secondary != nil {
		windows = append(windows, mapWindow(payload.RateLimit.Secondary, "secondary"))
	}
	if len(windows) == 0 {
		return UsageData{}, fmt.Errorf("response does not contain rate limit windows")
	}

	data := UsageData{
		PlanType:     payload.PlanType,
		Allowed:      payload.RateLimit.Allowed,
		LimitReached: payload.RateLimit.LimitReached,
		Windows:      windows,
	}
	if payload.RateLimitResetCredits != nil {
		available := payload.RateLimitResetCredits.AvailableCount
		data.AvailableRateLimitResetCredits = &available
	}

	return data, nil
}

func usageURLFromEnv() string {
	if value := strings.TrimSpace(os.Getenv("CQ_USAGE_URL")); value != "" {
		return value
	}
	return usageURL
}

func mapWindow(snapshot *windowSnapshot, fallback string) QuotaWindow {
	used := clampPercent(snapshot.UsedPercent)
	left := clampPercent(100 - used)

	result := QuotaWindow{
		Label:       formatWindowLabel(snapshot.LimitWindowSeconds, fallback),
		UsedPercent: used,
		LeftPercent: left,
		WindowSec:   snapshot.LimitWindowSeconds,
	}

	if snapshot.ResetAt > 0 {
		result.ResetAt = time.Unix(snapshot.ResetAt, 0)
	}

	return result
}

func formatWindowLabel(windowSec int64, fallback string) string {
	if windowSec == 18000 {
		return "5 hour usage limit"
	}
	if windowSec == 604800 {
		return "Weekly usage limit"
	}
	if windowSec > 0 && windowSec%3600 == 0 {
		return fmt.Sprintf("%d hour usage limit", windowSec/3600)
	}
	if windowSec > 0 && windowSec%60 == 0 {
		return fmt.Sprintf("%d minute usage limit", windowSec/60)
	}
	if windowSec > 0 {
		return fmt.Sprintf("%d second usage limit", windowSec)
	}
	return fallback + " usage limit"
}

func clampPercent(value float64) float64 {
	if value < 0 {
		return 0
	}
	if value > 100 {
		return 100
	}
	return value
}

func decodeJSON(reader io.Reader, target any) error {
	decoder := json.NewDecoder(reader)
	return decoder.Decode(target)
}
