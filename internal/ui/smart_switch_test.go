package ui

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/deLiseLINO/codex-quota/internal/api"
	"github.com/deLiseLINO/codex-quota/internal/config"
)

func TestSmartSwitchHotkeyQueuesLiveRefresh(t *testing.T) {
	m := testModelForHotkeys(4)
	m.Loading = false
	m.LoadingMap = map[string]bool{}
	m.ErrorsMap = map[string]error{}
	m.BackgroundLoadingMap = map[string]bool{}
	m.BackgroundErrorMap = map[string]bool{}
	m.AutoRefreshPending = map[string]bool{}
	m.ActiveAccountIx = 3
	m.UsageData = map[string]api.UsageData{
		"managed:1": usableWeeklyQuota(55),
		"managed:2": usableWeeklyQuota(54),
		"managed:3": usableWeeklyQuota(53),
		"managed:4": usableWeeklyQuota(52),
	}
	markAppliedSources(&m, map[config.Source]string{
		config.SourceCodex:    "managed:1",
		config.SourceOpenCode: "managed:2",
	})

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	got := updated.(Model)

	if !got.PendingSmartSwitchManual {
		t.Fatalf("expected manual smart switch state to remain active")
	}
	if !got.PendingSmartSwitchKeys["managed:1"] || !got.PendingSmartSwitchKeys["managed:2"] {
		t.Fatalf("expected applied accounts to be watched, got %#v", got.PendingSmartSwitchKeys)
	}
	if !got.BackgroundLoadingMap["managed:1"] || !got.BackgroundLoadingMap["managed:2"] || !got.BackgroundLoadingMap["managed:3"] {
		t.Fatalf("expected smart switch burst to queue both applied rows and top candidate, got %#v", got.BackgroundLoadingMap)
	}
	if got.BackgroundLoadingMap["managed:4"] {
		t.Fatalf("did not expect unrelated selected row to be prioritized, got %#v", got.BackgroundLoadingMap)
	}
	if cmd == nil {
		t.Fatalf("expected smart switch to return a refresh command")
	}
}

func TestManualSmartSwitchPrefersSubscribedHighestWeeklyQuota(t *testing.T) {
	m := testModelForHotkeys(4)
	m.Loading = false
	m.ActiveAccountIx = 3
	m.PendingSmartSwitchManual = true
	m.PendingSmartSwitchKeys = map[string]bool{"managed:1": true}
	m.LoadingMap = map[string]bool{}
	m.ErrorsMap = map[string]error{}
	m.BackgroundLoadingMap = map[string]bool{}
	m.BackgroundErrorMap = map[string]bool{}
	m.PlanTypeByAccount = map[string]string{
		"managed:2": "team",
		"managed:3": "plus",
		"managed:4": "free",
	}
	m.UsageData = map[string]api.UsageData{
		"managed:2": usableWeeklyQuota(42),
		"managed:3": usableWeeklyQuota(88),
		"managed:4": usableWeeklyQuota(97),
	}
	markAppliedSources(&m, map[config.Source]string{config.SourceCodex: "managed:1", config.SourceOpenCode: "managed:1"})

	updated, cmd := m.Update(DataMsg{
		AccountKey: "managed:1",
		Data:       exhaustedFiveHourQuota(),
		FetchedAt:  time.Now(),
	})
	got := updated.(Model)

	if got.activeAccountKey() != "managed:3" {
		t.Fatalf("active account = %q, want managed:3", got.activeAccountKey())
	}
	if got.PendingSmartSwitchManual || len(got.PendingSmartSwitchKeys) != 0 {
		t.Fatalf("expected pending smart switch state to clear, got %#v manual=%v", got.PendingSmartSwitchKeys, got.PendingSmartSwitchManual)
	}
	if got.Data.Windows[0].LeftPercent != 88 {
		t.Fatalf("expected switched account data to become active, got %#v", got.Data)
	}
	if cmd == nil {
		t.Fatalf("expected follow-up commands for switch/apply flow")
	}
}

func TestManualSmartSwitchShowsNoticeWhenNoReplacementExists(t *testing.T) {
	m := testModelForHotkeys(1)
	m.Loading = false
	m.PendingSmartSwitchManual = true
	m.PendingSmartSwitchKeys = map[string]bool{"managed:1": true}
	m.LoadingMap = map[string]bool{}
	m.ErrorsMap = map[string]error{}
	m.BackgroundLoadingMap = map[string]bool{}
	m.BackgroundErrorMap = map[string]bool{}
	markAppliedSources(&m, map[config.Source]string{config.SourceCodex: "managed:1", config.SourceOpenCode: "managed:1"})

	updated, cmd := m.Update(DataMsg{
		AccountKey: "managed:1",
		Data:       exhaustedFiveHourQuota(),
		FetchedAt:  time.Now(),
	})
	got := updated.(Model)

	if !strings.Contains(got.Notice, "no replacement account available") {
		t.Fatalf("expected no-replacement notice, got %q", got.Notice)
	}
	if got.activeAccountKey() != "managed:1" {
		t.Fatalf("active account = %q, want managed:1", got.activeAccountKey())
	}
	if cmd == nil {
		t.Fatalf("expected notice timeout command")
	}
}

func TestSmartSwitchIntervalUsesPeakOnlySteppedThresholds(t *testing.T) {
	m := testModelForHotkeys(1)
	m.Settings = config.DefaultSettings()
	m.Settings.AutoSwitchExhausted = true
	m.LoadingMap = map[string]bool{}
	m.BackgroundLoadingMap = map[string]bool{}
	m.ErrorsMap = map[string]error{}
	m.BackgroundErrorMap = map[string]bool{}
	markAppliedSources(&m, map[config.Source]string{config.SourceCodex: "managed:1", config.SourceOpenCode: "managed:1"})
	peakNow := time.Date(2026, 4, 10, 3, 0, 0, 0, time.UTC)
	tests := []struct {
		name        string
		leftPercent float64
		want        time.Duration
		ok          bool
	}{
		{name: "above threshold", leftPercent: 30, want: 0, ok: false},
		{name: "warning band", leftPercent: 20, want: 150 * time.Second, ok: true},
		{name: "medium band", leftPercent: 12, want: time.Minute, ok: true},
		{name: "fast band", leftPercent: 5, want: 30 * time.Second, ok: true},
		{name: "urgent band", leftPercent: 1, want: 10 * time.Second, ok: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			m.UsageData = map[string]api.UsageData{
				"managed:1": {
					Windows: []api.QuotaWindow{
						{Label: "Weekly usage limit", WindowSec: 604800, LeftPercent: 70, ResetAt: time.Now().Add(24 * time.Hour)},
						{Label: "5 hour usage limit", WindowSec: 18000, LeftPercent: tc.leftPercent, ResetAt: time.Now().Add(time.Hour)},
					},
				},
			}

			interval, ok := m.smartSwitchInterval("managed:1", peakNow)
			if ok != tc.ok || interval != tc.want {
				t.Fatalf("smartSwitchInterval() = %v, %v, want %v, %v", interval, ok, tc.want, tc.ok)
			}
		})
	}

	m.UsageData["managed:1"] = usableWeeklyQuota(5)
	interval, ok := m.smartSwitchInterval("managed:1", peakNow)
	if !ok || interval != 30*time.Second {
		t.Fatalf("smartSwitchInterval() with weekly fallback = %v, %v, want 30s, true", interval, ok)
	}

	offPeakNow := time.Date(2026, 4, 10, 18, 0, 0, 0, time.UTC)
	interval, ok = m.smartSwitchInterval("managed:1", offPeakNow)
	if ok || interval != 0 {
		t.Fatalf("smartSwitchInterval() off-peak = %v, %v, want 0, false", interval, ok)
	}
}

func TestAutoSwitchEnabledBackgroundRefreshSwitchesActiveAccount(t *testing.T) {
	m := testModelForHotkeys(2)
	m.Loading = false
	m.ActiveAccountIx = 1
	m.Settings = config.DefaultSettings()
	m.Settings.AutoSwitchExhausted = true
	m.LoadingMap = map[string]bool{}
	m.ErrorsMap = map[string]error{}
	m.BackgroundLoadingMap = map[string]bool{}
	m.BackgroundErrorMap = map[string]bool{}
	m.PlanTypeByAccount = map[string]string{"managed:2": "team"}
	m.UsageData = map[string]api.UsageData{
		"managed:2": usableWeeklyQuota(64),
	}
	markAppliedSources(&m, map[config.Source]string{config.SourceCodex: "managed:1", config.SourceOpenCode: "managed:1"})

	updated, cmd := m.Update(DataMsg{
		AccountKey: "managed:1",
		Data:       exhaustedFiveHourQuota(),
		Background: true,
		FetchedAt:  time.Now(),
	})
	got := updated.(Model)

	if got.activeAccountKey() != "managed:2" {
		t.Fatalf("active account = %q, want managed:2", got.activeAccountKey())
	}
	if cmd == nil {
		t.Fatalf("expected auto-switch background flow to produce follow-up commands")
	}
}

func TestManualSmartSwitchUsesSplitAppliedRowsNotSelection(t *testing.T) {
	m := testModelForHotkeys(4)
	m.Loading = false
	m.ActiveAccountIx = 3
	m.LoadingMap = map[string]bool{}
	m.ErrorsMap = map[string]error{}
	m.BackgroundLoadingMap = map[string]bool{}
	m.BackgroundErrorMap = map[string]bool{}
	m.AutoRefreshPending = map[string]bool{}
	m.UsageData = map[string]api.UsageData{
		"managed:1": usableWeeklyQuota(55),
		"managed:2": usableWeeklyQuota(54),
		"managed:3": usableWeeklyQuota(53),
		"managed:4": usableWeeklyQuota(52),
	}
	markAppliedSources(&m, map[config.Source]string{
		config.SourceCodex:    "managed:1",
		config.SourceOpenCode: "managed:2",
	})

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	got := updated.(Model)

	if !got.BackgroundLoadingMap["managed:1"] || !got.BackgroundLoadingMap["managed:2"] {
		t.Fatalf("expected both split applied rows to refresh, got %#v", got.BackgroundLoadingMap)
	}
	if got.BackgroundLoadingMap["managed:4"] {
		t.Fatalf("did not expect selected row to affect smart switch burst, got %#v", got.BackgroundLoadingMap)
	}
}

func TestManualSmartSwitchWithSplitAppliedRowsSwitchesOnFirstExhaustedRefresh(t *testing.T) {
	m := testModelForHotkeys(4)
	m.Loading = false
	m.PendingSmartSwitchManual = true
	m.PendingSmartSwitchKeys = map[string]bool{"managed:1": true, "managed:2": true}
	m.LoadingMap = map[string]bool{}
	m.ErrorsMap = map[string]error{}
	m.BackgroundLoadingMap = map[string]bool{}
	m.BackgroundErrorMap = map[string]bool{}
	m.PlanTypeByAccount = map[string]string{"managed:3": "team"}
	m.UsageData = map[string]api.UsageData{
		"managed:1": usableWeeklyQuota(44),
		"managed:3": usableWeeklyQuota(91),
		"managed:4": usableWeeklyQuota(21),
	}
	markAppliedSources(&m, map[config.Source]string{
		config.SourceCodex:    "managed:1",
		config.SourceOpenCode: "managed:2",
	})

	updated, cmd := m.Update(DataMsg{
		AccountKey: "managed:2",
		Data:       exhaustedFiveHourQuota(),
		FetchedAt:  time.Now(),
	})
	got := updated.(Model)

	if got.activeAccountKey() != "managed:3" {
		t.Fatalf("active account = %q, want managed:3", got.activeAccountKey())
	}
	if cmd == nil {
		t.Fatalf("expected switch/apply flow after first exhausted applied refresh")
	}
}

func usableWeeklyQuota(left float64) api.UsageData {
	return api.UsageData{
		Windows: []api.QuotaWindow{{Label: "Weekly usage limit", WindowSec: 604800, LeftPercent: left, ResetAt: time.Now().Add(24 * time.Hour)}},
	}
}

func exhaustedFiveHourQuota() api.UsageData {
	return api.UsageData{
		Windows: []api.QuotaWindow{
			{Label: "Weekly usage limit", WindowSec: 604800, LeftPercent: 67, ResetAt: time.Now().Add(24 * time.Hour)},
			{Label: "5 hour usage limit", WindowSec: 18000, LeftPercent: 0, ResetAt: time.Now().Add(time.Hour)},
		},
	}
}

func markAppliedSources(m *Model, assignments map[config.Source]string) {
	if m == nil {
		return
	}
	m.ActiveSourcesByIdentity = make(map[string][]string)
	for source, accountKey := range assignments {
		account := m.findAccountByKey(accountKey)
		if account == nil {
			continue
		}
		for _, key := range config.ActiveIdentityKeys(account) {
			m.ActiveSourcesByIdentity[key] = append(m.ActiveSourcesByIdentity[key], string(source))
		}
	}
}
