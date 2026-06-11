package ui

import (
	"errors"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/deLiseLINO/codex-quota/internal/api"
	"github.com/deLiseLINO/codex-quota/internal/config"
)

func TestWarmupCmdSkipsAlreadyWarmedWindow(t *testing.T) {
	withWarmupHooks(t)

	resetAt := time.Now().Add(time.Hour).UTC().Truncate(time.Second)
	account := testWarmupAccount("account-1", "free")
	stateKey := config.WarmupStateKey(account)
	loadWarmupState = func() (config.WarmupState, error) {
		return config.WarmupState{Entries: map[string]config.WarmupEntry{
			stateKey: {ResetAt: resetAt, WarmedAt: time.Now()},
		}}, nil
	}
	callQuotaAPI = func(accessToken, accountID string) (api.UsageData, error) {
		return testWarmupUsage("free", resetAt), nil
	}
	callWarmCodex = func(accessToken, accountID string) error {
		t.Fatalf("warm API should not be called for already-warmed window")
		return nil
	}

	msg := WarmupCmd([]*config.Account{account}, warmupSelected)()
	finished, ok := msg.(WarmupFinishedMsg)
	if !ok {
		t.Fatalf("message = %T, want WarmupFinishedMsg", msg)
	}
	if len(finished.Results) != 1 || !finished.Results[0].Skipped || finished.Results[0].SkipReason != "already warmed" {
		t.Fatalf("unexpected result: %#v", finished.Results)
	}
}

func TestWarmupCmdAllFreeFiltersAndContinuesAfterFailure(t *testing.T) {
	withWarmupHooks(t)

	resetAt := time.Now().Add(time.Hour).UTC().Truncate(time.Second)
	accounts := []*config.Account{
		testWarmupAccount("free-ok", "free"),
		testWarmupAccount("plus", "plus"),
		testWarmupAccount("free-fail", "free"),
	}
	warmCalls := []string{}
	saved := false
	callQuotaAPI = func(accessToken, accountID string) (api.UsageData, error) {
		for _, account := range accounts {
			if account.AccountID == accountID {
				return testWarmupUsage(account.Label, resetAt), nil
			}
		}
		return api.UsageData{}, errors.New("unknown account")
	}
	callWarmCodex = func(accessToken, accountID string) error {
		warmCalls = append(warmCalls, accountID)
		if accountID == "free-fail" {
			return errors.New("warm failed")
		}
		return nil
	}
	saveWarmupState = func(state config.WarmupState) error {
		saved = true
		if len(state.Entries) != 1 {
			t.Fatalf("saved entries = %d, want 1", len(state.Entries))
		}
		return nil
	}

	msg := WarmupCmd(accounts, warmupFree)()
	finished, ok := msg.(WarmupFinishedMsg)
	if !ok {
		t.Fatalf("message = %T, want WarmupFinishedMsg", msg)
	}
	if len(warmCalls) != 2 || warmCalls[0] != "free-ok" || warmCalls[1] != "free-fail" {
		t.Fatalf("warm calls = %v, want free accounts only", warmCalls)
	}
	if !saved {
		t.Fatalf("expected successful warm to save state")
	}
	if got := warmupSummary(finished.Results, nil); got != "warmup complete: 1 warmed, 1 skipped, 1 failed" {
		t.Fatalf("summary = %q", got)
	}
}

func TestActionMenuWarmFreeRequiresConfirmation(t *testing.T) {
	m := testModelForHotkeys(2)
	m.ActionMenuVisible = true
	m.ActionMenuCursor = 6

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	got := updated.(Model)

	if cmd != nil {
		t.Fatalf("expected no command before batch confirmation")
	}
	if !got.WarmupConfirm || got.WarmupMode != warmupFree {
		t.Fatalf("expected warmup free confirmation, got confirm=%v mode=%q", got.WarmupConfirm, got.WarmupMode)
	}
}

func withWarmupHooks(t *testing.T) {
	t.Helper()
	withFetchHooks(t)
	originalWarm := callWarmCodex
	originalLoad := loadWarmupState
	originalSave := saveWarmupState
	originalDelay := warmupRequestDelay
	warmupRequestDelay = 0
	loadWarmupState = func() (config.WarmupState, error) {
		return config.WarmupState{Entries: map[string]config.WarmupEntry{}}, nil
	}
	saveWarmupState = func(config.WarmupState) error { return nil }
	callWarmCodex = func(accessToken, accountID string) error { return nil }
	t.Cleanup(func() {
		callWarmCodex = originalWarm
		loadWarmupState = originalLoad
		saveWarmupState = originalSave
		warmupRequestDelay = originalDelay
	})
}

func testWarmupAccount(accountID, plan string) *config.Account {
	return &config.Account{
		Key:         accountID,
		Label:       plan,
		Email:       accountID + "@example.com",
		AccountID:   accountID,
		AccessToken: "access-token-" + accountID,
	}
}

func testWarmupUsage(plan string, resetAt time.Time) api.UsageData {
	return api.UsageData{
		PlanType: plan,
		Allowed:  true,
		Windows: []api.QuotaWindow{{
			Label:       "Weekly usage limit",
			WindowSec:   604800,
			LeftPercent: 90,
			ResetAt:     resetAt,
		}},
	}
}
