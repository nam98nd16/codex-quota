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
	m := testModelForHotkeys(1)
	m.Loading = false
	m.LoadingMap = map[string]bool{}
	m.ErrorsMap = map[string]error{}
	m.BackgroundLoadingMap = map[string]bool{}
	m.BackgroundErrorMap = map[string]bool{}
	m.AutoRefreshPending = map[string]bool{}
	m.UsageData = map[string]api.UsageData{
		"managed:1": usableWeeklyQuota(55),
	}

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	got := updated.(Model)

	if got.PendingSmartSwitchKey != "managed:1" {
		t.Fatalf("PendingSmartSwitchKey = %q, want managed:1", got.PendingSmartSwitchKey)
	}
	if !got.LoadingMap["managed:1"] {
		t.Fatalf("expected smart switch to queue a live refresh, got loading map %#v", got.LoadingMap)
	}
	if _, ok := got.UsageData["managed:1"]; ok {
		t.Fatalf("expected active account cache to be cleared before live refresh")
	}
	if cmd == nil {
		t.Fatalf("expected smart switch to return a refresh command")
	}
}

func TestManualSmartSwitchPrefersSubscribedHighestWeeklyQuota(t *testing.T) {
	m := testModelForHotkeys(4)
	m.Loading = false
	m.ActiveAccountIx = 0
	m.PendingSmartSwitchKey = "managed:1"
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

	updated, cmd := m.Update(DataMsg{
		AccountKey: "managed:1",
		Data:       exhaustedFiveHourQuota(),
		FetchedAt:  time.Now(),
	})
	got := updated.(Model)

	if got.activeAccountKey() != "managed:3" {
		t.Fatalf("active account = %q, want managed:3", got.activeAccountKey())
	}
	if got.PendingSmartSwitchKey != "" {
		t.Fatalf("expected pending smart switch state to clear, got %q", got.PendingSmartSwitchKey)
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
	m.PendingSmartSwitchKey = "managed:1"
	m.LoadingMap = map[string]bool{}
	m.ErrorsMap = map[string]error{}
	m.BackgroundLoadingMap = map[string]bool{}
	m.BackgroundErrorMap = map[string]bool{}

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

func TestSmartSwitchIntervalUsesFastWatchThreshold(t *testing.T) {
	m := testModelForHotkeys(1)
	m.Settings = config.DefaultSettings()
	m.Settings.AutoSwitchExhausted = true
	m.LoadingMap = map[string]bool{}
	m.BackgroundLoadingMap = map[string]bool{}
	m.ErrorsMap = map[string]error{}
	m.BackgroundErrorMap = map[string]bool{}
	m.UsageData = map[string]api.UsageData{
		"managed:1": {
			Windows: []api.QuotaWindow{
				{Label: "Weekly usage limit", WindowSec: 604800, LeftPercent: 70, ResetAt: time.Now().Add(24 * time.Hour)},
				{Label: "5 hour usage limit", WindowSec: 18000, LeftPercent: 9, ResetAt: time.Now().Add(time.Hour)},
			},
		},
	}

	interval, ok := m.smartSwitchInterval("managed:1")
	if !ok || interval != 10*time.Second {
		t.Fatalf("smartSwitchInterval() = %v, %v, want 10s, true", interval, ok)
	}

	m.UsageData["managed:1"] = usableWeeklyQuota(9)
	interval, ok = m.smartSwitchInterval("managed:1")
	if !ok || interval != 10*time.Second {
		t.Fatalf("smartSwitchInterval() with weekly fallback = %v, %v, want 10s, true", interval, ok)
	}
}

func TestAutoSwitchEnabledBackgroundRefreshSwitchesActiveAccount(t *testing.T) {
	m := testModelForHotkeys(2)
	m.Loading = false
	m.ActiveAccountIx = 0
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
