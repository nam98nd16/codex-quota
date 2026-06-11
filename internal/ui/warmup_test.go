package ui

import (
	"errors"
	"strings"
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

func TestWarmupShortcutOpensChooser(t *testing.T) {
	m := testModelForHotkeys(1)

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'w'}})
	got := updated.(Model)

	if cmd != nil {
		t.Fatalf("expected no command when opening chooser")
	}
	if !got.WarmupSelect {
		t.Fatalf("expected warmup chooser to open")
	}
	if !strings.Contains(got.renderFooter(), "s Selected") || !strings.Contains(got.renderFooter(), "f Free") || !strings.Contains(got.renderFooter(), "a All") {
		t.Fatalf("expected footer to show warmup choices, got %q", got.renderFooter())
	}
}

func TestWarmupChooserSelectedStartsWarmup(t *testing.T) {
	m := testModelForHotkeys(1)
	m.WarmupSelect = true

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	got := updated.(Model)

	if cmd == nil {
		t.Fatalf("expected selected warmup command")
	}
	if got.WarmupSelect || !got.WarmupRunning || got.WarmupMode != warmupSelected {
		t.Fatalf("expected selected warmup running, got select=%v running=%v mode=%q", got.WarmupSelect, got.WarmupRunning, got.WarmupMode)
	}
}

func TestWarmupChooserBatchModesRequireConfirmation(t *testing.T) {
	tests := []struct {
		key  rune
		mode warmupMode
	}{
		{key: 'f', mode: warmupFree},
		{key: 'a', mode: warmupAll},
	}

	for _, tc := range tests {
		t.Run(string(tc.key), func(t *testing.T) {
			m := testModelForHotkeys(2)
			m.WarmupSelect = true

			updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{tc.key}})
			got := updated.(Model)

			if cmd != nil {
				t.Fatalf("expected no command before batch confirmation")
			}
			if got.WarmupSelect || !got.WarmupConfirm || got.WarmupMode != tc.mode {
				t.Fatalf("expected batch confirmation for %q, got select=%v confirm=%v mode=%q", tc.key, got.WarmupSelect, got.WarmupConfirm, got.WarmupMode)
			}
		})
	}
}

func TestActionMenuShowsWarmupShortcutSequences(t *testing.T) {
	m := testModelForHotkeys(1)
	m.ActionMenuVisible = true
	out := m.renderActionMenuModal()

	for _, tc := range []struct {
		label    string
		shortcut string
	}{
		{label: "Warm selected quota", shortcut: "w s"},
		{label: "Warm all free", shortcut: "w f"},
		{label: "Warm all", shortcut: "w a"},
	} {
		if !actionMenuLineContains(out, tc.label, tc.shortcut) {
			t.Fatalf("expected action menu line to contain %q and %q:\n%s", tc.label, tc.shortcut, out)
		}
	}
}

func actionMenuLineContains(out, label, shortcut string) bool {
	for _, line := range strings.Split(out, "\n") {
		if strings.Contains(line, label) && strings.Contains(line, shortcut) {
			return true
		}
	}
	return false
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
