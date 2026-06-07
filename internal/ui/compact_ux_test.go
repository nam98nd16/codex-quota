package ui

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"

	"github.com/deLiseLINO/codex-quota/internal/api"
	"github.com/deLiseLINO/codex-quota/internal/config"
)

func TestCompactSearchFilterAndSortControls(t *testing.T) {
	m := compactUXTestModel()

	m.CompactSearchQuery = "beta"
	if got := m.compactVisualOrderIndices(); len(got) != 1 || got[0] != 1 {
		t.Fatalf("search order = %v, want only beta", got)
	}

	m.CompactSearchQuery = ""
	m.CompactFilter = compactFilterErrors
	if got := m.compactVisualOrderIndices(); len(got) != 1 || got[0] != 2 {
		t.Fatalf("error filter order = %v, want only gamma", got)
	}

	m.CompactFilter = compactFilterAll
	m.CompactSort = compactSortQuota
	if got := m.compactVisualOrderIndices(); len(got) < 3 || got[0] != 1 || got[1] != 0 {
		t.Fatalf("quota sort order = %v, want beta then alpha before errored rows", got)
	}
}

func TestCompactDefaultSortPrioritizesSubscriptionsThenQuotaResetName(t *testing.T) {
	now := time.Now()
	accounts := []*config.Account{
		{Key: "free-low", Label: "zeta@example.com", Email: "zeta@example.com", AccountID: "free-low", Source: config.SourceManaged, Writable: true},
		{Key: "sub-late", Label: "bravo@example.com", Email: "bravo@example.com", AccountID: "sub-late", Source: config.SourceManaged, Writable: true},
		{Key: "sub-high", Label: "alpha@example.com", Email: "alpha@example.com", AccountID: "sub-high", Source: config.SourceManaged, Writable: true},
		{Key: "sub-early", Label: "charlie@example.com", Email: "charlie@example.com", AccountID: "sub-early", Source: config.SourceManaged, Writable: true},
		{Key: "sub-name-a", Label: "aaron@example.com", Email: "aaron@example.com", AccountID: "sub-name-a", Source: config.SourceManaged, Writable: true},
		{Key: "sub-name-b", Label: "zoe@example.com", Email: "zoe@example.com", AccountID: "sub-name-b", Source: config.SourceManaged, Writable: true},
	}
	m := InitialModel(accounts, map[string][]string{}, map[string][]string{}, true)
	m.Loading = false
	m.LoadingMap = map[string]bool{}
	m.ErrorsMap = map[string]error{}
	m.PlanTypeByAccount = map[string]string{
		"sub-late":   "team",
		"sub-high":   "team",
		"sub-early":  "team",
		"sub-name-a": "team",
		"sub-name-b": "team",
	}
	m.UsageData = map[string]api.UsageData{
		"free-low":   {Windows: []api.QuotaWindow{{Label: "Weekly usage limit", WindowSec: 604800, LeftPercent: 1, ResetAt: now.Add(time.Hour)}}},
		"sub-late":   {Windows: []api.QuotaWindow{{Label: "Weekly usage limit", WindowSec: 604800, LeftPercent: 10, ResetAt: now.Add(3 * time.Hour)}}},
		"sub-high":   {Windows: []api.QuotaWindow{{Label: "Weekly usage limit", WindowSec: 604800, LeftPercent: 20, ResetAt: now.Add(time.Hour)}}},
		"sub-early":  {Windows: []api.QuotaWindow{{Label: "Weekly usage limit", WindowSec: 604800, LeftPercent: 10, ResetAt: now.Add(2 * time.Hour)}}},
		"sub-name-a": {Windows: []api.QuotaWindow{{Label: "Weekly usage limit", WindowSec: 604800, LeftPercent: 30, ResetAt: now.Add(4 * time.Hour)}}},
		"sub-name-b": {Windows: []api.QuotaWindow{{Label: "Weekly usage limit", WindowSec: 604800, LeftPercent: 30, ResetAt: now.Add(4 * time.Hour)}}},
	}

	got := m.compactVisualOrderIndices()
	want := []int{3, 1, 2, 4, 5, 0}
	if len(got) != len(want) {
		t.Fatalf("default subscription sort length = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("default subscription sort = %v, want %v", got, want)
		}
	}
}

func TestCompactPinnedAndCollapsedSections(t *testing.T) {
	m := compactUXTestModel()
	m.ActiveSourcesByIdentity = map[string][]string{
		"email-account:beta@example.com|id-2": {"codex", "opencode"},
	}
	m.CompactPinApplied = true

	out := ansi.Strip(m.renderCompactView())
	if !strings.Contains(out, "Applied accounts") {
		t.Fatalf("expected applied section:\n%s", out)
	}
	if got := m.compactVisualOrderIndices(); len(got) == 0 || got[0] != 1 {
		t.Fatalf("pinned order = %v, want beta first", got)
	}

	m.CompactExhaustedCollapsed = true
	out = ansi.Strip(m.renderCompactView())
	if !strings.Contains(out, "Exhausted accounts (1 hidden)") {
		t.Fatalf("expected collapsed exhausted header:\n%s", out)
	}
	if strings.Contains(out, "delta@example.com") {
		t.Fatalf("expected exhausted account hidden while collapsed:\n%s", out)
	}
}

func TestCompactStatusDensityAndDetailModal(t *testing.T) {
	m := compactUXTestModel()
	m.Width = 220
	m.PlanTypeByAccount = map[string]string{"a1": "team"}
	m.CompactSearchQuery = "alpha"
	m.CompactSort = compactSortReset
	m.CompactFilter = compactFilterAvailable

	detailed := ansi.Strip(m.renderCompactRecordsStatus())
	for _, want := range []string{"Search \"alpha\"", "Filter available", "Sort reset", "Subscriptions 1"} {
		if !strings.Contains(detailed, want) {
			t.Fatalf("expected detailed status to contain %q:\n%s", want, detailed)
		}
	}

	m.CompactStatusMinimal = true
	minimal := ansi.Strip(m.renderCompactRecordsStatus())
	if strings.Contains(minimal, "Filter available") || strings.Contains(minimal, "Sort reset") {
		t.Fatalf("expected minimal status to hide controls:\n%s", minimal)
	}

	m.CompactDetailVisible = true
	detail := ansi.Strip(m.renderCompactDetailModal())
	for _, want := range []string{"Quota details", "alpha@example.com", "Weekly", "80% left"} {
		if !strings.Contains(detail, want) {
			t.Fatalf("expected detail modal to contain %q:\n%s", want, detail)
		}
	}
}

func TestCompactMouseHitTestingSelectsRows(t *testing.T) {
	m := compactUXTestModel()
	m.Width = 120
	m.Height = 30
	m.CompactPinApplied = false
	m.CompactSort = compactSortOriginal

	x, y := compactScreenPointContaining(t, m, "beta@example")
	if got := m.compactAccountIndexAtPoint(x, y); got != 1 {
		t.Fatalf("mouse hit row 2 = %d, want beta index 1", got)
	}
	if !m.selectCompactAccountAtPoint(x, y) || m.ActiveAccountIx != 1 {
		t.Fatalf("expected mouse selection to activate beta, active=%d", m.ActiveAccountIx)
	}
}

func TestCompactMouseHitTestingAccountsForCenteredTallLayout(t *testing.T) {
	m := compactUXTestModel()
	m.Width = 220
	m.Height = 60
	m.CompactPinApplied = false
	m.CompactSort = compactSortOriginal

	x, y := compactScreenPointContaining(t, m, "beta@example")
	if got := m.compactAccountIndexAtPoint(x, y); got != 1 {
		t.Fatalf("centered mouse hit = %d, want beta index 1 at %d,%d", got, x, y)
	}
}

func TestCompactMouseHitTestingSelectsWideColumns(t *testing.T) {
	m := compactUXTestModel()
	m.Width = 220
	m.Height = 30
	m.CompactPinApplied = false
	m.CompactSort = compactSortOriginal

	x, y := compactScreenPointContaining(t, m, "gamma@example")
	if got := m.compactAccountIndexAtPoint(x, y); got != 2 {
		t.Fatalf("wide-column mouse hit = %d, want gamma index 2 at %d,%d", got, x, y)
	}
}

func TestCompactMouseHitTestingIgnoresSectionHeaders(t *testing.T) {
	m := compactUXTestModel()
	m.Width = 180
	m.Height = 30

	x, y := compactScreenPointContaining(t, m, "Exhausted accounts")
	if got := m.compactAccountIndexAtPoint(x, y); got != -1 {
		t.Fatalf("section header mouse hit = %d, want no account", got)
	}
}

func compactScreenPointContaining(t *testing.T, m Model, needle string) (int, int) {
	t.Helper()
	for y, line := range strings.Split(ansi.Strip(m.View()), "\n") {
		if start := strings.Index(line, needle); start >= 0 {
			return ansi.StringWidth(line[:start]), y
		}
	}
	t.Fatalf("could not find %q in rendered view:\n%s", needle, ansi.Strip(m.View()))
	return 0, 0
}

func compactUXTestModel() Model {
	now := time.Now()
	accounts := []*config.Account{
		{Key: "a1", Label: "alpha@example.com", Email: "alpha@example.com", AccountID: "id-1", Source: config.SourceManaged, Writable: true},
		{Key: "a2", Label: "beta@example.com", Email: "beta@example.com", AccountID: "id-2", Source: config.SourceManaged, Writable: true},
		{Key: "a3", Label: "gamma@example.com", Email: "gamma@example.com", AccountID: "id-3", Source: config.SourceManaged, Writable: true},
		{Key: "a4", Label: "delta@example.com", Email: "delta@example.com", AccountID: "id-4", Source: config.SourceManaged, Writable: true},
	}
	m := InitialModel(accounts, map[string][]string{}, map[string][]string{}, true)
	m.Width = 120
	m.Height = 30
	m.Loading = false
	m.LoadingMap = map[string]bool{}
	m.ErrorsMap = map[string]error{"a3": errors.New("request failed with status 401: token expired")}
	m.UsageData = map[string]api.UsageData{
		"a1": {Windows: []api.QuotaWindow{{Label: "Weekly usage limit", WindowSec: 604800, UsedPercent: 20, LeftPercent: 80, ResetAt: now.Add(4 * time.Hour)}}},
		"a2": {Windows: []api.QuotaWindow{{Label: "Weekly usage limit", WindowSec: 604800, UsedPercent: 85, LeftPercent: 15, ResetAt: now.Add(2 * time.Hour)}}},
		"a4": {LimitReached: true, Windows: []api.QuotaWindow{{Label: "Weekly usage limit", WindowSec: 604800, UsedPercent: 100, LeftPercent: 0, ResetAt: now.Add(time.Hour)}}},
	}
	return m
}
