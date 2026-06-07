package ui

import (
	"fmt"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"github.com/deLiseLINO/codex-quota/internal/api"
	"github.com/deLiseLINO/codex-quota/internal/config"
)

func TestCompactMouseWheelScrollsViewport(t *testing.T) {
	m := testCompactScrollModel(30, 140, 18)

	updated, _ := m.Update(tea.MouseMsg{Type: tea.MouseWheelDown, Button: tea.MouseButtonWheelDown})
	got := updated.(Model)
	if got.CompactScrollOffset != compactMouseScrollRows {
		t.Fatalf("CompactScrollOffset = %d, want %d", got.CompactScrollOffset, compactMouseScrollRows)
	}

	updated, _ = got.Update(tea.MouseMsg{Type: tea.MouseWheelUp, Button: tea.MouseButtonWheelUp})
	got = updated.(Model)
	if got.CompactScrollOffset != 0 {
		t.Fatalf("CompactScrollOffset = %d, want 0 after wheel up", got.CompactScrollOffset)
	}
}

func TestCompactViewClipsLargeListsAndKeepsFooter(t *testing.T) {
	m := testCompactScrollModel(30, 140, 18)

	out := ansi.Strip(m.View())
	if height := lipgloss.Height(out); height > m.Height {
		t.Fatalf("view height = %d, want <= %d\n%s", height, m.Height, out)
	}
	if !strings.Contains(out, "Ctrl+F Search") || !strings.Contains(out, "Enter Menu") {
		t.Fatalf("expected footer to remain visible, got:\n%s", out)
	}
	if !strings.Contains(out, "Records 1-") || !strings.Contains(out, "/ 30") {
		t.Fatalf("expected compact records status with total count, got:\n%s", out)
	}
	if strings.Contains(out, "user29@example.com") {
		t.Fatalf("expected tail account to be clipped before scrolling, got:\n%s", out)
	}
}

func TestCompactRecordsStatusUpdatesAfterScroll(t *testing.T) {
	m := testCompactScrollModel(30, 140, 18)
	m.scrollCompactRows(5)

	out := ansi.Strip(m.View())
	if !strings.Contains(out, "Records 6-") || !strings.Contains(out, "/ 30") {
		t.Fatalf("expected scrolled compact records status with total count, got:\n%s", out)
	}
}

func TestCompactRecordsStatusIncludesActiveAccountDetail(t *testing.T) {
	m := testCompactScrollModel(30, 170, 18)

	out := ansi.Strip(m.View())
	for _, want := range []string{"Active user00@example.com", "Weekly 95%", "("} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected compact records status to contain %q, got:\n%s", want, out)
		}
	}
}

func TestCompactRecordsStatusTracksActiveAccount(t *testing.T) {
	m := testCompactScrollModel(30, 170, 18)

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlE})
	got := updated.(Model)
	out := ansi.Strip(got.View())
	if !strings.Contains(out, "Active user29@example.com") {
		t.Fatalf("expected compact records status to track active account, got:\n%s", out)
	}
}

func TestCompactRecordsStatusStaysWithinNarrowWidth(t *testing.T) {
	m := testCompactScrollModel(30, 84, 18)

	out := ansi.Strip(m.View())
	for _, line := range strings.Split(out, "\n") {
		if width := ansi.StringWidth(line); width > m.Width {
			t.Fatalf("line width = %d, want <= %d\n%s", width, m.Width, line)
		}
	}
}

func TestCompactKeyboardNavigationKeepsActiveRowVisible(t *testing.T) {
	m := testCompactScrollModel(20, 140, 16)

	var updated tea.Model = m
	for i := 0; i < 15; i++ {
		updated, _ = updated.(Model).Update(tea.KeyMsg{Type: tea.KeyDown})
	}
	got := updated.(Model)

	out := ansi.Strip(got.View())
	if !strings.Contains(out, "● user15@example") {
		t.Fatalf("expected active account to be visible after keyboard navigation, got:\n%s", out)
	}
	if got.CompactScrollOffset == 0 {
		t.Fatalf("expected compact scroll offset to advance")
	}
}

func TestCompactPageNavigationSupportsMacFriendlyKeys(t *testing.T) {
	m := testCompactScrollModel(30, 140, 18)
	pageSize := m.compactVisibleRowCapacity()
	if pageSize <= 0 {
		t.Fatalf("expected positive compact page size")
	}

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlD})
	got := updated.(Model)
	if got.ActiveAccountIx != pageSize {
		t.Fatalf("ActiveAccountIx = %d after Ctrl+D, want %d", got.ActiveAccountIx, pageSize)
	}
	if got.CompactScrollOffset == 0 {
		t.Fatalf("expected Ctrl+D to advance compact scroll offset")
	}

	updated, _ = got.Update(tea.KeyMsg{Type: tea.KeyCtrlU})
	got = updated.(Model)
	if got.ActiveAccountIx != 0 {
		t.Fatalf("ActiveAccountIx = %d after Ctrl+U, want 0", got.ActiveAccountIx)
	}

	updated, _ = got.Update(tea.KeyMsg{Type: tea.KeyDown, Alt: true})
	got = updated.(Model)
	if got.ActiveAccountIx != pageSize {
		t.Fatalf("ActiveAccountIx = %d after Alt+Down, want %d", got.ActiveAccountIx, pageSize)
	}
}

func TestCompactHomeEndSupportsMacFriendlyAliases(t *testing.T) {
	m := testCompactScrollModel(30, 140, 18)

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlE})
	got := updated.(Model)
	if got.ActiveAccountIx != len(got.Accounts)-1 {
		t.Fatalf("ActiveAccountIx = %d after Ctrl+E, want last account", got.ActiveAccountIx)
	}

	updated, _ = got.Update(tea.KeyMsg{Type: tea.KeyCtrlA})
	got = updated.(Model)
	if got.ActiveAccountIx != 0 {
		t.Fatalf("ActiveAccountIx = %d after Ctrl+A, want first account", got.ActiveAccountIx)
	}
}

func TestCompactWideViewUsesTwoColumns(t *testing.T) {
	m := testCompactScrollModel(40, 170, 18)
	columns, _, _ := m.compactColumnLayout()
	if columns != 2 {
		t.Fatalf("compact columns = %d, want 2", columns)
	}

	viewportHeight := m.compactListViewportHeight()
	out := ansi.Strip(m.View())
	if !strings.Contains(out, fmt.Sprintf("user%02d@", viewportHeight)) {
		t.Fatalf("expected second column account to be visible, got:\n%s", out)
	}
	if height := lipgloss.Height(out); height > m.Height {
		t.Fatalf("view height = %d, want <= %d\n%s", height, m.Height, out)
	}
	for _, line := range strings.Split(out, "\n") {
		if width := ansi.StringWidth(line); width > m.Width {
			t.Fatalf("line width = %d, want <= %d\n%s", width, m.Width, line)
		}
	}
}

func TestCompactNarrowViewStaysSingleColumn(t *testing.T) {
	m := testCompactScrollModel(40, 120, 18)
	columns, _, _ := m.compactColumnLayout()
	if columns != 1 {
		t.Fatalf("compact columns = %d, want 1", columns)
	}

	viewportHeight := m.compactListViewportHeight()
	out := ansi.Strip(m.View())
	if strings.Contains(out, fmt.Sprintf("user%02d@", viewportHeight)) {
		t.Fatalf("did not expect second-column account in narrow view, got:\n%s", out)
	}
}

func TestCompactWideKeyboardNavigationKeepsSecondColumnActiveVisible(t *testing.T) {
	m := testCompactScrollModel(40, 170, 18)
	viewportHeight := m.compactListViewportHeight()
	steps := viewportHeight + 1

	var updated tea.Model = m
	for i := 0; i < steps; i++ {
		updated, _ = updated.(Model).Update(tea.KeyMsg{Type: tea.KeyDown})
	}
	got := updated.(Model)

	out := ansi.Strip(got.View())
	activeLabel := fmt.Sprintf("● user%02d@example", steps)
	if !strings.Contains(out, activeLabel) {
		t.Fatalf("expected second-column active account %q to be visible, got:\n%s", activeLabel, out)
	}
	if got.CompactScrollOffset != 0 {
		t.Fatalf("CompactScrollOffset = %d, want 0 while active row fits in second column", got.CompactScrollOffset)
	}
}

func TestCompactWideScrollClampsToTwoColumnCapacity(t *testing.T) {
	m := testCompactScrollModel(40, 170, 18)
	capacity := m.compactVisibleRowCapacity()
	want := len(m.compactRows()) - capacity
	if want < 0 {
		want = 0
	}

	m.scrollCompactRows(999)
	if m.CompactScrollOffset != want {
		t.Fatalf("CompactScrollOffset = %d, want %d", m.CompactScrollOffset, want)
	}
}

func TestCompactWideViewBalancesShortTwoColumnPage(t *testing.T) {
	m := testCompactScrollModel(14, 170, 18)
	viewportHeight := m.compactListViewportHeight()
	if viewportHeight >= len(m.compactRows()) {
		t.Fatalf("test requires more rows than viewport height")
	}
	if m.compactVisibleRowCapacity() <= len(m.compactRows()) {
		t.Fatalf("test requires rows to fit within two-column capacity")
	}

	rendered := ansi.Strip(m.renderCompactViewWithin(viewportHeight))
	lines := compactNonEmptyLines(rendered)
	wantLines := (len(m.compactRows()) + 1) / 2
	if len(lines) != wantLines {
		t.Fatalf("rendered lines = %d, want balanced two-column lines = %d\n%s", len(lines), wantLines, rendered)
	}
	for _, line := range lines {
		if strings.Count(line, "user") != 2 {
			t.Fatalf("expected every balanced row to contain both columns, got line %q\n%s", line, rendered)
		}
	}
}

func compactNonEmptyLines(rendered string) []string {
	lines := []string{}
	for _, line := range strings.Split(rendered, "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		lines = append(lines, line)
	}
	return lines
}

func testCompactScrollModel(count, width, height int) Model {
	accounts := make([]*config.Account, 0, count)
	usage := make(map[string]api.UsageData, count)
	for i := 0; i < count; i++ {
		key := fmt.Sprintf("managed:%02d", i)
		label := fmt.Sprintf("user%02d@example.com", i)
		accounts = append(accounts, &config.Account{
			Key:       key,
			Label:     label,
			Email:     label,
			AccountID: fmt.Sprintf("acc-%02d", i),
			Source:    config.SourceManaged,
			Writable:  true,
		})
		usage[key] = api.UsageData{Windows: []api.QuotaWindow{{Label: "Weekly usage limit", WindowSec: 604800, LeftPercent: 95.0, ResetAt: time.Now().Add(2 * time.Hour)}}}
	}

	m := InitialModel(accounts, map[string][]string{}, map[string][]string{}, true)
	m.Width = width
	m.Height = height
	m.Loading = false
	m.LoadingMap = map[string]bool{}
	m.ErrorsMap = map[string]error{}
	m.UsageData = usage
	m.Data = usage[accounts[0].Key]
	return m
}
