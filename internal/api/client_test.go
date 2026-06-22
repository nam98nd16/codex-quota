package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCallAPIParsesRateLimitResetCredits(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer token" {
			t.Fatalf("authorization header = %q", r.Header.Get("Authorization"))
		}
		if r.Header.Get("ChatGPT-Account-Id") != "account-1" {
			t.Fatalf("account header = %q", r.Header.Get("ChatGPT-Account-Id"))
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"plan_type": "plus",
			"rate_limit": map[string]any{
				"allowed":       true,
				"limit_reached": false,
				"primary_window": map[string]any{
					"limit_window_seconds": 18000,
					"used_percent":         42,
					"reset_at":             1893456000,
				},
			},
			"rate_limit_reset_credits": map[string]any{"available_count": 2},
		})
	}))
	defer server.Close()
	t.Setenv("CQ_USAGE_URL", server.URL)

	data, err := CallAPI("token", "account-1")
	if err != nil {
		t.Fatalf("CallAPI returned error: %v", err)
	}
	if data.AvailableRateLimitResetCredits == nil || *data.AvailableRateLimitResetCredits != 2 {
		t.Fatalf("available resets = %#v, want 2", data.AvailableRateLimitResetCredits)
	}
}

func TestConsumeRateLimitResetCreditSendsRedeemRequest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("method = %s, want POST", r.Method)
		}
		if r.Header.Get("Authorization") != "Bearer token" {
			t.Fatalf("authorization header = %q", r.Header.Get("Authorization"))
		}
		if r.Header.Get("ChatGPT-Account-Id") != "account-1" {
			t.Fatalf("account header = %q", r.Header.Get("ChatGPT-Account-Id"))
		}

		var body map[string]string
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("failed to decode request body: %v", err)
		}
		if body["redeem_request_id"] != "redeem-1" {
			t.Fatalf("redeem_request_id = %q, want redeem-1", body["redeem_request_id"])
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"code": "reset", "windows_reset": 2})
	}))
	defer server.Close()
	t.Setenv("CQ_RATE_LIMIT_RESET_CONSUME_URL", server.URL)

	result, err := ConsumeRateLimitResetCredit("token", "account-1", "redeem-1")
	if err != nil {
		t.Fatalf("ConsumeRateLimitResetCredit returned error: %v", err)
	}
	if result.Outcome != RateLimitResetOutcomeReset || result.WindowsReset != 2 {
		t.Fatalf("result = %#v, want reset with two windows", result)
	}
}
