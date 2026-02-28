package ui

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"

	"github.com/deLiseLINO/codex-quota/internal/api"
	"github.com/deLiseLINO/codex-quota/internal/config"
)

func TestRenderWindowsViewSingleWindowHasGroupHeader(t *testing.T) {
	model := testModelWithWindows([]api.QuotaWindow{
		{
			Label:       "Weekly usage limit",
			WindowSec:   604800,
			LeftPercent: 30.0,
			ResetAt:     time.Now().Add(2 * time.Hour),
		},
	})

	out := ansi.Strip(model.renderWindowsView())
	if !strings.Contains(out, "Weekly\n") {
		t.Fatalf("expected standalone group header for single window output:\n%s", out)
	}
	if !strings.Contains(out, "Weekly usage limit") {
		t.Fatalf("expected window label in output, got:\n%s", out)
	}
}

func TestRenderWindowsViewMultipleWindowsKeepsGroupHeaders(t *testing.T) {
	model := testModelWithWindows([]api.QuotaWindow{
		{
			Label:       "5 hour usage limit",
			WindowSec:   18000,
			LeftPercent: 10.0,
			ResetAt:     time.Now().Add(1 * time.Hour),
		},
		{
			Label:       "Weekly usage limit",
			WindowSec:   604800,
			LeftPercent: 60.0,
			ResetAt:     time.Now().Add(24 * time.Hour),
		},
	})

	out := ansi.Strip(model.renderWindowsView())
	if !strings.Contains(out, "5 hour\n") {
		t.Fatalf("expected 5 hour group header, got:\n%s", out)
	}
	if !strings.Contains(out, "Weekly\n") {
		t.Fatalf("expected Weekly group header, got:\n%s", out)
	}
	if !strings.Contains(out, "5 hour usage limit") || !strings.Contains(out, "Weekly usage limit") {
		t.Fatalf("expected both window labels in output, got:\n%s", out)
	}
}

func TestViewCentersContentInLargeViewport(t *testing.T) {
	model := testModelWithWindows([]api.QuotaWindow{
		{
			Label:       "Weekly usage limit",
			WindowSec:   604800,
			LeftPercent: 30.0,
			ResetAt:     time.Now().Add(2 * time.Hour),
		},
	})
	model.Width = 180
	model.Height = 40

	out := ansi.Strip(model.View())
	lines := strings.Split(out, "\n")

	titleLine := ""
	for _, line := range lines {
		if strings.Contains(line, "Codex Quota Monitor") {
			titleLine = line
			break
		}
	}
	if titleLine == "" {
		t.Fatalf("title line not found in rendered output")
	}
	if !strings.HasPrefix(titleLine, "  ") {
		t.Fatalf("expected centered line with left padding, got: %q", titleLine)
	}
}

func TestViewTabModeLoadingRendersWeeklyOnlyForUnknownOrFreePlan(t *testing.T) {
	model := testModelWithWindows([]api.QuotaWindow{
		{
			Label:       "Weekly usage limit",
			WindowSec:   604800,
			LeftPercent: 30.0,
			ResetAt:     time.Now().Add(2 * time.Hour),
		},
	})
	model.CompactMode = false
	model.Loading = true

	out := ansi.Strip(model.View())
	if !strings.Contains(out, "Weekly usage limit") {
		t.Fatalf("expected weekly loading skeleton row in tab mode output:\n%s", out)
	}
	if strings.Contains(out, "5 hour usage limit") {
		t.Fatalf("did not expect 5 hour loading skeleton row for unknown/free plan:\n%s", out)
	}
	if !strings.Contains(out, "Loading...") {
		t.Fatalf("expected loading status in tab mode skeleton output:\n%s", out)
	}
}

func TestViewTabModeLoadingRendersWeeklyOnlyForKnownFreePlan(t *testing.T) {
	model := testModelWithWindows([]api.QuotaWindow{
		{
			Label:       "Weekly usage limit",
			WindowSec:   604800,
			LeftPercent: 30.0,
			ResetAt:     time.Now().Add(2 * time.Hour),
		},
	})
	model.CompactMode = false
	model.Loading = true
	model.PlanTypeByAccount = map[string]string{
		"account-1": "free",
	}

	out := ansi.Strip(model.View())
	if strings.Contains(out, "5 hour usage limit") {
		t.Fatalf("did not expect 5 hour loading skeleton for known free plan:\n%s", out)
	}
	if !strings.Contains(out, "Weekly usage limit") {
		t.Fatalf("expected weekly loading skeleton row in tab mode output:\n%s", out)
	}
}

func TestViewTabModeLoadingRenders5HourForKnownSubscription(t *testing.T) {
	model := testModelWithWindows([]api.QuotaWindow{
		{
			Label:       "Weekly usage limit",
			WindowSec:   604800,
			LeftPercent: 30.0,
			ResetAt:     time.Now().Add(2 * time.Hour),
		},
	})
	model.CompactMode = false
	model.Loading = true
	model.PlanTypeByAccount = map[string]string{
		"account-1": "pro",
	}

	out := ansi.Strip(model.View())
	if !strings.Contains(out, "5 hour usage limit") {
		t.Fatalf("expected 5 hour loading skeleton for known subscription:\n%s", out)
	}
	if !strings.Contains(out, "Weekly usage limit") {
		t.Fatalf("expected weekly loading skeleton row in tab mode output:\n%s", out)
	}
}

func TestRenderCompactView_MixedRowsStayAligned(t *testing.T) {
	model := testModelWithWindows([]api.QuotaWindow{
		{
			Label:       "Weekly usage limit",
			WindowSec:   604800,
			LeftPercent: 30.0,
			ResetAt:     time.Now().Add(2 * time.Hour),
		},
	})
	model.CompactMode = true
	model.Width = 150

	model.Accounts = []*config.Account{
		{Key: "a1", Label: "first@example.com", Email: "first@example.com", AccountID: "id-1", Source: config.SourceManaged, Writable: true},
		{Key: "a2", Label: "second@example.com", Email: "second@example.com", AccountID: "id-2", Source: config.SourceManaged, Writable: true},
		{Key: "a3", Label: "third@example.com", Email: "third@example.com", AccountID: "id-3", Source: config.SourceManaged, Writable: true},
	}
	model.ActiveAccountIx = 0
	model.UsageData = map[string]api.UsageData{
		"a1": {Windows: []api.QuotaWindow{{Label: "Weekly usage limit", WindowSec: 604800, LeftPercent: 30.0, ResetAt: time.Now().Add(2 * time.Hour)}}},
	}
	model.LoadingMap = map[string]bool{"a2": true}
	model.ErrorsMap = map[string]error{}

	out := ansi.Strip(model.View())
	lines := strings.Split(out, "\n")
	if len(lines) == 0 {
		t.Fatalf("expected non-empty rendered view")
	}

	if !strings.Contains(out, "Loading...") {
		t.Fatalf("expected loading status in compact view")
	}
	if !strings.Contains(out, "Queued...") {
		t.Fatalf("expected queued status in compact view")
	}
	if !strings.Contains(out, "30.0%") {
		t.Fatalf("expected percentage metadata in compact view")
	}
}

func TestRenderCompactView_NarrowWidthRendersWithoutBreakage(t *testing.T) {
	model := testModelWithWindows([]api.QuotaWindow{
		{
			Label:       "Weekly usage limit",
			WindowSec:   604800,
			LeftPercent: 30.0,
			ResetAt:     time.Now().Add(2 * time.Hour),
		},
	})
	model.CompactMode = true
	model.Width = 72

	out := ansi.Strip(model.renderCompactView())
	if !strings.Contains(out, "Loading...") {
		t.Fatalf("expected loading state in narrow mode output:\n%s", out)
	}
	if !strings.Contains(out, "user@example.com") {
		t.Fatalf("expected account label in narrow mode output:\n%s", out)
	}
}

func TestRenderCompactView_LoadingAndQueuedShareRowGeometry(t *testing.T) {
	model := testModelWithWindows([]api.QuotaWindow{
		{
			Label:       "Weekly usage limit",
			WindowSec:   604800,
			LeftPercent: 30.0,
			ResetAt:     time.Now().Add(2 * time.Hour),
		},
	})
	model.CompactMode = true
	model.Width = 140
	model.Accounts = []*config.Account{
		{Key: "a1", Label: "first@example.com", Email: "first@example.com", AccountID: "id-1", Source: config.SourceManaged, Writable: true},
		{Key: "a2", Label: "second@example.com", Email: "second@example.com", AccountID: "id-2", Source: config.SourceManaged, Writable: true},
		{Key: "a3", Label: "third@example.com", Email: "third@example.com", AccountID: "id-3", Source: config.SourceManaged, Writable: true},
	}
	model.LoadingMap = map[string]bool{"a2": true}
	model.UsageData = map[string]api.UsageData{
		"a1": {Windows: []api.QuotaWindow{{Label: "Weekly usage limit", WindowSec: 604800, LeftPercent: 30.0, ResetAt: time.Now().Add(2 * time.Hour)}}},
	}
	model.ErrorsMap = map[string]error{}

	out := ansi.Strip(model.renderCompactView())
	rawLines := strings.Split(out, "\n")
	lines := make([]string, 0, 3)
	for _, line := range rawLines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		lines = append(lines, line)
	}
	if len(lines) != 3 {
		t.Fatalf("expected 3 non-empty lines, got %d", len(lines))
	}

	loadingWidth := ansi.StringWidth(strings.TrimRight(lines[1], " "))
	queuedWidth := ansi.StringWidth(strings.TrimRight(lines[2], " "))
	diff := loadingWidth - queuedWidth
	if diff < 0 {
		diff = -diff
	}
	if diff > 1 {
		t.Fatalf("expected loading and queued rows to have near-equal width, got %d vs %d", loadingWidth, queuedWidth)
	}
}

func TestRenderCompactView_GroupsExhaustedAccountsAtBottom(t *testing.T) {
	model := testModelWithWindows([]api.QuotaWindow{
		{Label: "Weekly usage limit", WindowSec: 604800, LeftPercent: 40.0, ResetAt: time.Now().Add(2 * time.Hour)},
	})
	model.CompactMode = true
	model.Width = 150
	model.Accounts = []*config.Account{
		{Key: "a1", Label: "normal-1@example.com", Email: "normal-1@example.com", AccountID: "id-1", Source: config.SourceManaged, Writable: true},
		{Key: "a2", Label: "exhausted@example.com", Email: "exhausted@example.com", AccountID: "id-2", Source: config.SourceManaged, Writable: true},
		{Key: "a3", Label: "normal-2@example.com", Email: "normal-2@example.com", AccountID: "id-3", Source: config.SourceManaged, Writable: true},
	}
	model.UsageData = map[string]api.UsageData{
		"a1": {Windows: []api.QuotaWindow{{Label: "Weekly usage limit", WindowSec: 604800, LeftPercent: 40.0, ResetAt: time.Now().Add(2 * time.Hour)}}},
		"a2": {Windows: []api.QuotaWindow{{Label: "Weekly usage limit", WindowSec: 604800, LeftPercent: 0.0, ResetAt: time.Now().Add(2 * time.Hour)}}},
		"a3": {Windows: []api.QuotaWindow{{Label: "Weekly usage limit", WindowSec: 604800, LeftPercent: 80.0, ResetAt: time.Now().Add(2 * time.Hour)}}},
	}

	out := ansi.Strip(model.renderCompactView())
	headerIx := strings.Index(out, "Exhausted accounts")
	if headerIx < 0 {
		t.Fatalf("expected exhausted section header, got:\n%s", out)
	}

	normalIx := strings.Index(out, "normal-1@example.com")
	exhaustedIx := strings.Index(out, "exhausted@example.com")
	if normalIx < 0 || exhaustedIx < 0 {
		t.Fatalf("expected both normal and exhausted labels in output, got:\n%s", out)
	}
	if normalIx > headerIx {
		t.Fatalf("expected normal accounts before exhausted header, got:\n%s", out)
	}
	if exhaustedIx < headerIx {
		t.Fatalf("expected exhausted account below exhausted header, got:\n%s", out)
	}
}

func TestRenderCompactView_TreatsLimitReachedAsExhausted(t *testing.T) {
	model := testModelWithWindows([]api.QuotaWindow{
		{Label: "Weekly usage limit", WindowSec: 604800, LeftPercent: 40.0, ResetAt: time.Now().Add(2 * time.Hour)},
	})
	model.CompactMode = true
	model.Width = 150
	model.Accounts = []*config.Account{
		{Key: "a1", Label: "normal@example.com", Email: "normal@example.com", AccountID: "id-1", Source: config.SourceManaged, Writable: true},
		{Key: "a2", Label: "limit-reached@example.com", Email: "limit-reached@example.com", AccountID: "id-2", Source: config.SourceManaged, Writable: true},
	}
	model.UsageData = map[string]api.UsageData{
		"a1": {Windows: []api.QuotaWindow{{Label: "Weekly usage limit", WindowSec: 604800, LeftPercent: 40.0, ResetAt: time.Now().Add(2 * time.Hour)}}},
		"a2": {LimitReached: true, Windows: []api.QuotaWindow{{Label: "Weekly usage limit", WindowSec: 604800, LeftPercent: 25.0, ResetAt: time.Now().Add(2 * time.Hour)}}},
	}

	out := ansi.Strip(model.renderCompactView())
	headerIx := strings.Index(out, "Exhausted accounts")
	limitReachedIx := strings.Index(out, "limit-reached@example.com")
	if headerIx < 0 || limitReachedIx < 0 {
		t.Fatalf("expected exhausted header and account in output, got:\n%s", out)
	}
	if limitReachedIx < headerIx {
		t.Fatalf("expected limit-reached account in exhausted section, got:\n%s", out)
	}
}

func TestRenderCompactView_LoadingAccountStaysInMainSection(t *testing.T) {
	model := testModelWithWindows([]api.QuotaWindow{
		{Label: "Weekly usage limit", WindowSec: 604800, LeftPercent: 40.0, ResetAt: time.Now().Add(2 * time.Hour)},
	})
	model.CompactMode = true
	model.Width = 150
	model.Accounts = []*config.Account{
		{Key: "a1", Label: "loading@example.com", Email: "loading@example.com", AccountID: "id-1", Source: config.SourceManaged, Writable: true},
		{Key: "a2", Label: "exhausted@example.com", Email: "exhausted@example.com", AccountID: "id-2", Source: config.SourceManaged, Writable: true},
	}
	model.LoadingMap = map[string]bool{"a1": true}
	model.UsageData = map[string]api.UsageData{
		"a1": {LimitReached: true, Windows: []api.QuotaWindow{{Label: "Weekly usage limit", WindowSec: 604800, LeftPercent: 0.0, ResetAt: time.Now().Add(2 * time.Hour)}}},
		"a2": {Windows: []api.QuotaWindow{{Label: "Weekly usage limit", WindowSec: 604800, LeftPercent: 0.0, ResetAt: time.Now().Add(2 * time.Hour)}}},
	}

	out := ansi.Strip(model.renderCompactView())
	headerIx := strings.Index(out, "Exhausted accounts")
	loadingIx := strings.Index(out, "loading@example.com")
	if headerIx < 0 || loadingIx < 0 {
		t.Fatalf("expected header and loading account in output, got:\n%s", out)
	}
	if loadingIx > headerIx {
		t.Fatalf("expected loading account to stay in main section, got:\n%s", out)
	}
}

func TestRenderCompactView_ActiveAccountHighlightWorksInExhaustedSection(t *testing.T) {
	model := testModelWithWindows([]api.QuotaWindow{
		{Label: "Weekly usage limit", WindowSec: 604800, LeftPercent: 40.0, ResetAt: time.Now().Add(2 * time.Hour)},
	})
	model.CompactMode = true
	model.Width = 150
	model.Accounts = []*config.Account{
		{Key: "a1", Label: "normal@example.com", Email: "normal@example.com", AccountID: "id-1", Source: config.SourceManaged, Writable: true},
		{Key: "a2", Label: "exhausted@example.com", Email: "exhausted@example.com", AccountID: "id-2", Source: config.SourceManaged, Writable: true},
	}
	model.ActiveAccountIx = 1
	model.UsageData = map[string]api.UsageData{
		"a1": {Windows: []api.QuotaWindow{{Label: "Weekly usage limit", WindowSec: 604800, LeftPercent: 40.0, ResetAt: time.Now().Add(2 * time.Hour)}}},
		"a2": {Windows: []api.QuotaWindow{{Label: "Weekly usage limit", WindowSec: 604800, LeftPercent: 0.0, ResetAt: time.Now().Add(2 * time.Hour)}}},
	}

	out := ansi.Strip(model.renderCompactView())
	if !strings.Contains(out, "> exhausted@example.com") {
		t.Fatalf("expected active marker in exhausted section, got:\n%s", out)
	}
}

func testModelWithWindows(windows []api.QuotaWindow) Model {
	accounts := []*config.Account{
		{
			Key:       "account-1",
			Label:     "user@example.com",
			Email:     "user@example.com",
			AccountID: "98609d8a-85fb-4ff8-aee2-9344e68fbe3f",
			Source:    config.SourceManaged,
			Writable:  true,
		},
	}

	model := InitialModel(accounts, map[string][]string{}, map[string][]string{}, false)
	model.Loading = false
	model.Data = api.UsageData{Windows: windows}
	return model
}
