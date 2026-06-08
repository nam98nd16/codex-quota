package ui

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/deLiseLINO/codex-quota/internal/api"
	"github.com/deLiseLINO/codex-quota/internal/auth"
	"github.com/deLiseLINO/codex-quota/internal/config"
)

func TestFetchDataCmdSoftRefreshUsesRefreshedToken(t *testing.T) {
	withFetchHooks(t)
	refreshCalls := 0
	apiTokens := []string{}
	refreshAccountToken = func(account *config.Account) error {
		refreshCalls++
		account.AccessToken = "new-token"
		return nil
	}
	callQuotaAPI = func(accessToken, accountID string) (api.UsageData, error) {
		apiTokens = append(apiTokens, accessToken)
		return testUsageData(), nil
	}

	msg := FetchDataCmd(testRefreshAccount("old-token", "refresh-token"), false)()
	dataMsg, ok := msg.(DataMsg)
	if !ok {
		t.Fatalf("message = %T, want DataMsg", msg)
	}
	if refreshCalls != 1 {
		t.Fatalf("refreshCalls = %d, want 1", refreshCalls)
	}
	if len(apiTokens) != 1 || apiTokens[0] != "new-token" {
		t.Fatalf("api tokens = %v, want refreshed token", apiTokens)
	}
	if dataMsg.Account.AccessToken != "new-token" {
		t.Fatalf("returned account token = %q, want refreshed token", dataMsg.Account.AccessToken)
	}
}

func TestFetchDataCmdSoftRefreshFailureKeepsValidToken(t *testing.T) {
	withFetchHooks(t)
	refreshAccountToken = func(account *config.Account) error {
		return errors.New("temporary refresh failure")
	}
	apiTokens := []string{}
	callQuotaAPI = func(accessToken, accountID string) (api.UsageData, error) {
		apiTokens = append(apiTokens, accessToken)
		return testUsageData(), nil
	}

	msg := FetchDataCmd(testRefreshAccount("old-token", "refresh-token"), false)()
	if _, ok := msg.(DataMsg); !ok {
		t.Fatalf("message = %T, want DataMsg", msg)
	}
	if len(apiTokens) != 1 || apiTokens[0] != "old-token" {
		t.Fatalf("api tokens = %v, want old token after soft refresh failure", apiTokens)
	}
}

func TestFetchDataCmdRequiredRefreshFailureBlocksFetch(t *testing.T) {
	withFetchHooks(t)
	isTokenExpired = func(account *config.Account) bool { return true }
	shouldRefreshTokenSoon = func(account *config.Account) bool { return true }
	refreshAccountToken = func(account *config.Account) error {
		return errors.New("refresh denied")
	}
	callQuotaAPI = func(accessToken, accountID string) (api.UsageData, error) {
		t.Fatalf("quota API should not be called after required refresh failure")
		return api.UsageData{}, nil
	}

	msg := FetchDataCmd(testRefreshAccount("old-token", "refresh-token"), false)()
	errMsg, ok := msg.(ErrMsg)
	if !ok {
		t.Fatalf("message = %T, want ErrMsg", msg)
	}
	if errMsg.Err == nil || !strings.Contains(errMsg.Err.Error(), "token refresh failed") {
		t.Fatalf("expected refresh error, got %#v", errMsg.Err)
	}
}

func TestFetchDataCmdSkipsSoftRefreshWithoutRefreshToken(t *testing.T) {
	withFetchHooks(t)
	refreshAccountToken = func(account *config.Account) error {
		t.Fatalf("soft refresh should be skipped without refresh token")
		return nil
	}
	apiTokens := []string{}
	callQuotaAPI = func(accessToken, accountID string) (api.UsageData, error) {
		apiTokens = append(apiTokens, accessToken)
		return testUsageData(), nil
	}

	msg := FetchDataCmd(testRefreshAccount("old-token", ""), false)()
	if _, ok := msg.(DataMsg); !ok {
		t.Fatalf("message = %T, want DataMsg", msg)
	}
	if len(apiTokens) != 1 || apiTokens[0] != "old-token" {
		t.Fatalf("api tokens = %v, want old token", apiTokens)
	}
}

func TestFetchDataCmdUnauthorizedRefreshesAndRetries(t *testing.T) {
	withFetchHooks(t)
	shouldRefreshTokenSoon = func(account *config.Account) bool { return false }
	refreshCalls := 0
	refreshAccountToken = func(account *config.Account) error {
		refreshCalls++
		account.AccessToken = "new-token"
		return nil
	}
	apiTokens := []string{}
	callQuotaAPI = func(accessToken, accountID string) (api.UsageData, error) {
		apiTokens = append(apiTokens, accessToken)
		if len(apiTokens) == 1 {
			return api.UsageData{}, &api.HTTPError{StatusCode: 401, Body: "expired"}
		}
		return testUsageData(), nil
	}

	msg := FetchDataCmd(testRefreshAccount("old-token", "refresh-token"), false)()
	if _, ok := msg.(DataMsg); !ok {
		t.Fatalf("message = %T, want DataMsg", msg)
	}
	if refreshCalls != 1 {
		t.Fatalf("refreshCalls = %d, want 1", refreshCalls)
	}
	if len(apiTokens) != 2 || apiTokens[0] != "old-token" || apiTokens[1] != "new-token" {
		t.Fatalf("api tokens = %v, want old then new", apiTokens)
	}
}

func withFetchHooks(t *testing.T) {
	t.Helper()
	originalExpired := isTokenExpired
	originalSoon := shouldRefreshTokenSoon
	originalRefresh := refreshAccountToken
	originalAPI := callQuotaAPI
	isTokenExpired = func(account *config.Account) bool { return false }
	shouldRefreshTokenSoon = func(account *config.Account) bool { return true }
	refreshAccountToken = auth.RefreshToken
	callQuotaAPI = api.CallAPI
	t.Cleanup(func() {
		isTokenExpired = originalExpired
		shouldRefreshTokenSoon = originalSoon
		refreshAccountToken = originalRefresh
		callQuotaAPI = originalAPI
	})
}

func testRefreshAccount(accessToken, refreshToken string) *config.Account {
	return &config.Account{
		Key:          "account-1",
		AccountID:    "account-id",
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresAt:    time.Now().Add(23 * time.Hour),
	}
}

func testUsageData() api.UsageData {
	return api.UsageData{
		Allowed: true,
		Windows: []api.QuotaWindow{{
			Label:       "Weekly usage limit",
			WindowSec:   604800,
			LeftPercent: 90,
			ResetAt:     time.Now().Add(time.Hour),
		}},
	}
}
