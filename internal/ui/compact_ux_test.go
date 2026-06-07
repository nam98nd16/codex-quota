package ui

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/lipgloss"
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
	m.CompactSearchQuery = "alpha"
	m.CompactSort = compactSortReset
	m.CompactFilter = compactFilterAvailable

	detailed := ansi.Strip(m.renderCompactRecordsStatus())
	for _, want := range []string{"Search \"alpha\"", "Filter available", "Sort reset"} {
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
	m.CompactSort = compactSortDefault

	firstListY := 1 + lipgloss.Height(m.compactViewPrefix())
	if got := m.compactAccountIndexAtPoint(2, firstListY+1); got != 1 {
		t.Fatalf("mouse hit row 2 = %d, want beta index 1", got)
	}
	if !m.selectCompactAccountAtPoint(2, firstListY+1) || m.ActiveAccountIx != 1 {
		t.Fatalf("expected mouse selection to activate beta, active=%d", m.ActiveAccountIx)
	}
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
