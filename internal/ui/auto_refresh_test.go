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
	model.LastQuotaFetchAt = map[string]time.Time{"managed:1": time.Now().Add(-10 * time.Minute)}

	if !model.enqueueDueAutoRefreshes(time.Now()) {
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
