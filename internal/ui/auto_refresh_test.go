package ui

import (
	"errors"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"

	"github.com/deLiseLINO/codex-quota/internal/api"
	"github.com/deLiseLINO/codex-quota/internal/config"
)

func TestRenderCompactView_UsesShortGradientFor5HourWindow(t *testing.T) {
	forceTrueColor(t)

	model := testModelWithWindows(nil)
	model.CompactMode = true
	model.Width = 140
	model.Loading = false
	model.LoadingMap = map[string]bool{}
	model.ErrorsMap = map[string]error{}
	model.UsageData = map[string]api.UsageData{
		"account-1": {
			Windows: []api.QuotaWindow{
				{Label: "Weekly usage limit", WindowSec: 604800, LeftPercent: 40, ResetAt: time.Now().Add(3 * time.Hour)},
				{Label: "5 hour usage limit", WindowSec: 18000, LeftPercent: 40, ResetAt: time.Now().Add(time.Hour)},
			},
		},
	}

	out := model.renderCompactView()
	if !strings.Contains(out, "38;2;65;133;243") {
		t.Fatalf("expected compact 5 hour row to use short gradient, got:\n%s", out)
	}
}

func TestRenderCompactView_KeepsQuotaVisibleOnBackgroundError(t *testing.T) {
	model := testModelWithWindows(nil)
	model.CompactMode = true
	model.Width = 140
	model.Loading = false
	model.LoadingMap = map[string]bool{}
	model.ErrorsMap = map[string]error{"account-1": errors.New("boom")}
	model.BackgroundErrorMap = map[string]bool{"account-1": true}
	model.UsageData = map[string]api.UsageData{
		"account-1": {
			Windows: []api.QuotaWindow{{Label: "Weekly usage limit", WindowSec: 604800, LeftPercent: 40, ResetAt: time.Now().Add(time.Hour)}},
		},
	}

	out := ansi.Strip(model.renderCompactView())
	if strings.Contains(out, "Error:") {
		t.Fatalf("expected cached quota row during background error, got:\n%s", out)
	}
	if !strings.Contains(out, "40%") {
		t.Fatalf("expected compact output to keep quota percent during background error, got:\n%s", out)
	}
}

func TestAutoRefreshIntervalUsesPeakAndOffPeakSettings(t *testing.T) {
	model := testModelForHotkeys(1)
	model.Settings = config.DefaultSettings()

	peakInterval, ok := model.autoRefreshInterval(time.Date(2026, 4, 10, 3, 0, 0, 0, time.UTC))
	if !ok || peakInterval != 5*time.Minute {
		t.Fatalf("expected peak auto refresh interval 5m, got %v (ok=%v)", peakInterval, ok)
	}

	offPeakInterval, ok := model.autoRefreshInterval(time.Date(2026, 4, 10, 18, 0, 0, 0, time.UTC))
	if !ok || offPeakInterval != 30*time.Minute {
		t.Fatalf("expected off-peak auto refresh interval 30m, got %v (ok=%v)", offPeakInterval, ok)
	}
}

func TestAutoRefreshDueAccountsUseBackgroundLoading(t *testing.T) {
	model := testModelForHotkeys(1)
	model.Settings = config.DefaultSettings()
	model.Loading = false
	model.LoadingMap = map[string]bool{}
	model.BackgroundLoadingMap = map[string]bool{}
	model.AutoRefreshPending = map[string]bool{}
	model.UsageData = map[string]api.UsageData{
		"managed:1": {Windows: []api.QuotaWindow{{Label: "Weekly usage limit", WindowSec: 604800, LeftPercent: 40, ResetAt: time.Now().Add(time.Hour)}}},
	}
	now := time.Date(2026, 4, 10, 3, 0, 0, 0, time.UTC)
	model.LastQuotaFetchAt = map[string]time.Time{"managed:1": now.Add(-10 * time.Minute)}

	if !model.enqueueDueAutoRefreshes(now) {
		t.Fatalf("expected due account to be enqueued for background refresh")
	}
	cmd := model.fetchNextCmd()
	if cmd == nil {
		t.Fatalf("expected background refresh fetch command")
	}
	if !model.BackgroundLoadingMap["managed:1"] {
		t.Fatalf("expected account to be marked as background loading")
	}
	if model.LoadingMap["managed:1"] {
		t.Fatalf("did not expect foreground loading marker for background refresh")
	}
}

func TestBackgroundErrorDoesNotRaiseModalWhenQuotaDataExists(t *testing.T) {
	model := testModelForHotkeys(1)
	model.Settings = config.DefaultSettings()
	model.Loading = false
	model.UsageData = map[string]api.UsageData{
		"managed:1": {Windows: []api.QuotaWindow{{Label: "Weekly usage limit", WindowSec: 604800, LeftPercent: 40, ResetAt: time.Now().Add(time.Hour)}}},
	}
	model.Data = model.UsageData["managed:1"]

	updated, _ := model.Update(ErrMsg{AccountKey: "managed:1", Err: errors.New("background refresh failed"), Background: true, FetchedAt: time.Now()})
	got := updated.(Model)

	if got.Err != nil {
		t.Fatalf("did not expect blocking error modal for background refresh, got %v", got.Err)
	}
	if got.ErrorsMap["managed:1"] == nil || !got.BackgroundErrorMap["managed:1"] {
		t.Fatalf("expected background error state to be retained on the account")
	}
	if !strings.Contains(ansi.Strip(got.renderWindowsView()), "Weekly usage limit") {
		t.Fatalf("expected current quota view to remain visible after background error")
	}
}

func TestActionMenuSettingsOpensSettingsOverlay(t *testing.T) {
	model := testModelForHotkeys(1)
	model.ActionMenuVisible = true
	model.ActionMenuCursor = 7

	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	got := updated.(Model)

	if !got.SettingsVisible {
		t.Fatalf("expected settings overlay to open from action menu")
	}
	if got.ActionMenuVisible {
		t.Fatalf("expected action menu to close when settings opens")
	}
	if got.SettingsDraft.AutoRefreshPeakStart != "08:30" || got.SettingsDraft.AutoRefreshPeakEnd != "22:30" {
		t.Fatalf("expected settings draft to load normalized defaults, got %q-%q", got.SettingsDraft.AutoRefreshPeakStart, got.SettingsDraft.AutoRefreshPeakEnd)
	}
}

func TestAutoRefreshDueAtUsesPeakAppliedLowQuotaCadenceWithoutAutoSwitch(t *testing.T) {
	model := testModelForHotkeys(1)
	model.Settings = config.DefaultSettings()
	model.Settings.AutoSwitchExhausted = false
	model.LoadingMap = map[string]bool{}
	model.BackgroundLoadingMap = map[string]bool{}
	model.ErrorsMap = map[string]error{}
	model.BackgroundErrorMap = map[string]bool{}
	lastFetchAt := time.Date(2026, 4, 10, 2, 59, 0, 0, time.UTC)
	now := time.Date(2026, 4, 10, 3, 0, 0, 0, time.UTC)
	model.LastQuotaFetchAt = map[string]time.Time{"managed:1": lastFetchAt}
	model.UsageData = map[string]api.UsageData{
		"managed:1": {
			Windows: []api.QuotaWindow{
				{Label: "Weekly usage limit", WindowSec: 604800, LeftPercent: 70, ResetAt: time.Now().Add(24 * time.Hour)},
				{Label: "5 hour usage limit", WindowSec: 18000, LeftPercent: 5, ResetAt: time.Now().Add(time.Hour)},
			},
		},
	}
	markAppliedSources(&model, map[config.Source]string{config.SourceCodex: "managed:1", config.SourceOpenCode: "managed:1"})

	dueAt, ok := model.autoRefreshDueAt("managed:1", now)
	if !ok {
		t.Fatalf("expected auto-refresh due time")
	}
	want := lastFetchAt.Add(30 * time.Second).Add(autoRefreshJitter("managed:1", 30*time.Second))
	if !dueAt.Equal(want) {
		t.Fatalf("autoRefreshDueAt() = %v, want %v", dueAt, want)
	}
}

func TestAutoRefreshDueAtBacksOffWhenAppliedLowQuotaIsUnchanged(t *testing.T) {
	model := testModelForHotkeys(1)
	model.Settings = config.DefaultSettings()
	model.Settings.AutoSwitchExhausted = false
	model.LoadingMap = map[string]bool{}
	model.BackgroundLoadingMap = map[string]bool{}
	model.ErrorsMap = map[string]error{}
	model.BackgroundErrorMap = map[string]bool{}
	markAppliedSources(&model, map[config.Source]string{config.SourceCodex: "managed:1", config.SourceOpenCode: "managed:1"})

	baseAt := time.Date(2026, 4, 10, 3, 0, 0, 0, time.UTC)
	intervals := []time.Duration{30 * time.Second, time.Minute, 2 * time.Minute, 4 * time.Minute, 5 * time.Minute}
	for index, wantInterval := range intervals {
		fetchedAt := baseAt.Add(time.Duration(index) * time.Minute)
		updated, _ := model.Update(DataMsg{AccountKey: "managed:1", Data: lowFiveHourQuota(5), FetchedAt: fetchedAt})
		model = updated.(Model)

		dueAt, ok := model.autoRefreshDueAt("managed:1", fetchedAt)
		if !ok {
			t.Fatalf("expected due time after unchanged refresh %d", index)
		}
		want := fetchedAt.Add(wantInterval).Add(autoRefreshJitter("managed:1", wantInterval))
		if !dueAt.Equal(want) {
			t.Fatalf("unchanged refresh %d dueAt = %v, want %v", index, dueAt, want)
		}
	}
}

func TestAutoRefreshDueAtResetsBackoffWhenAppliedLowQuotaChanges(t *testing.T) {
	model := testModelForHotkeys(1)
	model.Settings = config.DefaultSettings()
	model.Settings.AutoSwitchExhausted = false
	model.LoadingMap = map[string]bool{}
	model.BackgroundLoadingMap = map[string]bool{}
	model.ErrorsMap = map[string]error{}
	model.BackgroundErrorMap = map[string]bool{}
	markAppliedSources(&model, map[config.Source]string{config.SourceCodex: "managed:1", config.SourceOpenCode: "managed:1"})

	firstAt := time.Date(2026, 4, 10, 3, 0, 0, 0, time.UTC)
	updated, _ := model.Update(DataMsg{AccountKey: "managed:1", Data: lowFiveHourQuota(1), FetchedAt: firstAt})
	model = updated.(Model)
	secondAt := firstAt.Add(10 * time.Second)
	updated, _ = model.Update(DataMsg{AccountKey: "managed:1", Data: lowFiveHourQuota(1), FetchedAt: secondAt})
	model = updated.(Model)

	dueAt, ok := model.autoRefreshDueAt("managed:1", secondAt)
	if !ok {
		t.Fatalf("expected backed-off due time")
	}
	wantInterval := 20 * time.Second
	want := secondAt.Add(wantInterval).Add(autoRefreshJitter("managed:1", wantInterval))
	if !dueAt.Equal(want) {
		t.Fatalf("backed-off dueAt = %v, want %v", dueAt, want)
	}

	changedAt := firstAt.Add(30 * time.Second)
	updated, _ = model.Update(DataMsg{AccountKey: "managed:1", Data: lowFiveHourQuota(2), FetchedAt: changedAt})
	model = updated.(Model)
	dueAt, ok = model.autoRefreshDueAt("managed:1", changedAt)
	if !ok {
		t.Fatalf("expected due time after changed quota")
	}
	wantInterval = 10 * time.Second
	want = changedAt.Add(wantInterval).Add(autoRefreshJitter("managed:1", wantInterval))
	if !dueAt.Equal(want) {
		t.Fatalf("changed quota dueAt = %v, want %v", dueAt, want)
	}
}

func TestAutoRefreshDueAtDoesNotUseLowQuotaCadenceForNonAppliedAccount(t *testing.T) {
	model := testModelForHotkeys(2)
	model.Settings = config.DefaultSettings()
	model.Settings.AutoSwitchExhausted = false
	model.LoadingMap = map[string]bool{}
	model.BackgroundLoadingMap = map[string]bool{}
	model.ErrorsMap = map[string]error{}
	model.BackgroundErrorMap = map[string]bool{}
	fetchedAt := time.Date(2026, 4, 10, 3, 0, 0, 0, time.UTC)
	model.LastQuotaFetchAt = map[string]time.Time{"managed:2": fetchedAt}
	model.UsageData = map[string]api.UsageData{"managed:2": lowFiveHourQuota(1)}
	markAppliedSources(&model, map[config.Source]string{config.SourceCodex: "managed:1", config.SourceOpenCode: "managed:1"})

	dueAt, ok := model.autoRefreshDueAt("managed:2", fetchedAt)
	if !ok {
		t.Fatalf("expected normal due time for non-applied account")
	}
	wantInterval := 5 * time.Minute
	want := fetchedAt.Add(wantInterval).Add(autoRefreshJitter("managed:2", wantInterval))
	if !dueAt.Equal(want) {
		t.Fatalf("non-applied dueAt = %v, want %v", dueAt, want)
	}
}

func TestAutoRefreshDueAtKeepsOffPeakCadenceWhenQuotaIsLow(t *testing.T) {
	model := testModelForHotkeys(1)
	model.Settings = config.DefaultSettings()
	model.Settings.AutoSwitchExhausted = true
	model.LoadingMap = map[string]bool{}
	model.BackgroundLoadingMap = map[string]bool{}
	model.ErrorsMap = map[string]error{}
	model.BackgroundErrorMap = map[string]bool{}
	lastFetchAt := time.Date(2026, 4, 10, 17, 0, 0, 0, time.UTC)
	now := time.Date(2026, 4, 10, 18, 0, 0, 0, time.UTC)
	model.LastQuotaFetchAt = map[string]time.Time{"managed:1": lastFetchAt}
	model.UsageData = map[string]api.UsageData{
		"managed:1": {
			Windows: []api.QuotaWindow{
				{Label: "Weekly usage limit", WindowSec: 604800, LeftPercent: 70, ResetAt: time.Now().Add(24 * time.Hour)},
				{Label: "5 hour usage limit", WindowSec: 18000, LeftPercent: 1, ResetAt: time.Now().Add(time.Hour)},
			},
		},
	}
	markAppliedSources(&model, map[config.Source]string{config.SourceCodex: "managed:1", config.SourceOpenCode: "managed:1"})

	dueAt, ok := model.autoRefreshDueAt("managed:1", now)
	if !ok {
		t.Fatalf("expected auto-refresh due time")
	}
	wantInterval := 30 * time.Minute
	want := lastFetchAt.Add(wantInterval).Add(autoRefreshJitter("managed:1", wantInterval))
	if !dueAt.Equal(want) {
		t.Fatalf("autoRefreshDueAt() off-peak = %v, want %v", dueAt, want)
	}
}

func TestAutoRefreshJitterCapsFiveMinuteInterval(t *testing.T) {
	jitter := autoRefreshJitter("managed:1", 5*time.Minute)
	if jitter < 0 || jitter > 10*time.Second {
		t.Fatalf("autoRefreshJitter(5m) = %v, want <= 10s", jitter)
	}
}
