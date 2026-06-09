package ui

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"
)

func TestCompactColumnLayoutThresholds(t *testing.T) {
	cases := []struct {
		width int
		want  int
	}{
		{width: 101, want: 1},
		{width: 102, want: 2},
		{width: 152, want: 3},
		{width: 202, want: 4},
		{width: 209, want: 4},
		{width: 210, want: 5},
	}

	for _, tc := range cases {
		m := testCompactScrollModel(80, tc.width, 24)
		columns, _, _ := m.compactColumnLayout()
		if columns != tc.want {
			t.Fatalf("width %d: compact columns = %d, want %d", tc.width, columns, tc.want)
		}
	}
}

func TestCompactExhaustedHeaderSpansFullWidthInMultiColumn(t *testing.T) {
	m := testCompactScrollModel(24, 200, 24)
	for i := 18; i < 24; i++ {
		m.ExhaustedSticky[m.Accounts[i].Key] = true
	}

	rendered := ansi.Strip(m.renderCompactViewWithin(m.compactListViewportHeight()))
	headerLine := ""
	for _, line := range strings.Split(rendered, "\n") {
		if strings.Contains(line, "Exhausted accounts") {
			headerLine = line
			break
		}
	}
	if headerLine == "" {
		t.Fatalf("expected exhausted header in multi-column output:\n%s", rendered)
	}
	if strings.Contains(headerLine, "user") {
		t.Fatalf("expected exhausted header to occupy its own row, got %q\n%s", headerLine, rendered)
	}
	if strings.TrimSpace(headerLine) != "Exhausted accounts" {
		t.Fatalf("expected full-width exhausted header row, got %q", headerLine)
	}
}

func TestCompactFourColumnRowsStayAlignedAndWithinWidth(t *testing.T) {
	m := testCompactScrollModel(80, 202, 24)
	rendered := ansi.Strip(m.renderCompactViewWithin(m.compactListViewportHeight()))
	contentWidth := m.compactContentWidth()

	for _, line := range strings.Split(rendered, "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		if width := ansi.StringWidth(line); width > contentWidth {
			t.Fatalf("line width = %d, want <= %d\n%s", width, contentWidth, line)
		}
		if count := strings.Count(line, "user"); count > 4 {
			t.Fatalf("rendered more than four account cells in one row: %q", line)
		}
	}
}

func TestCompactFourColumnRowsUseDenseRelativeResetText(t *testing.T) {
	m := testCompactScrollModel(80, 202, 24)
	rendered := ansi.Strip(m.renderCompactViewWithin(m.compactListViewportHeight()))

	foundAccountLine := false
	for _, line := range strings.Split(rendered, "\n") {
		if !strings.Contains(line, "user") {
			continue
		}
		foundAccountLine = true
		if strings.Contains(line, "(") || strings.Contains(line, ":") {
			t.Fatalf("expected dense four-column reset text to be relative-only, got %q\n%s", line, rendered)
		}
	}
	if !foundAccountLine {
		t.Fatalf("expected account rows in four-column output:\n%s", rendered)
	}
}

func TestCompactFiveColumnRowsStayAlignedAndWithinWidth(t *testing.T) {
	m := testCompactScrollModel(80, 210, 24)
	rendered := ansi.Strip(m.renderCompactViewWithin(m.compactListViewportHeight()))
	contentWidth := m.compactContentWidth()

	foundFiveCellRow := false
	for _, line := range strings.Split(rendered, "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		if width := ansi.StringWidth(line); width > contentWidth {
			t.Fatalf("line width = %d, want <= %d\n%s", width, contentWidth, line)
		}
		count := strings.Count(line, "user")
		if count > 5 {
			t.Fatalf("rendered more than five account cells in one row: %q", line)
		}
		if count == 5 {
			foundFiveCellRow = true
		}
	}
	if !foundFiveCellRow {
		t.Fatalf("expected a five-column account row:\n%s", rendered)
	}
}

func TestCompactFiveColumnRowsUseUltraDenseRelativeResetText(t *testing.T) {
	m := testCompactScrollModel(80, 210, 24)
	rendered := ansi.Strip(m.renderCompactViewWithin(m.compactListViewportHeight()))

	foundAccountLine := false
	for _, line := range strings.Split(rendered, "\n") {
		if !strings.Contains(line, "user") {
			continue
		}
		foundAccountLine = true
		if strings.Contains(line, "(") || strings.Contains(line, ":") || strings.Contains(line, "example") {
			t.Fatalf("expected ultra-dense five-column cells, got %q\n%s", line, rendered)
		}
	}
	if !foundAccountLine {
		t.Fatalf("expected account rows in five-column output:\n%s", rendered)
	}
}

func TestCompactActiveExhaustedAccountRemainsVisibleInMultiColumn(t *testing.T) {
	m := testCompactScrollModel(24, 200, 24)
	for i := 18; i < 24; i++ {
		m.ExhaustedSticky[m.Accounts[i].Key] = true
	}
	m.ActiveAccountIx = 23
	m.ensureCompactActiveVisible()

	rendered := ansi.Strip(m.renderCompactViewWithin(m.compactListViewportHeight()))
	if !strings.Contains(rendered, "● user23@example") {
		t.Fatalf("expected active exhausted account to remain visible:\n%s", rendered)
	}
}

func TestCompactResetTextUsesSingleRelativeUnit(t *testing.T) {
	hours := compactResetText(time.Now().Add(10*time.Hour + 50*time.Minute))
	if !strings.Contains(hours, "(10h)") || strings.Contains(hours, "50m") || strings.Contains(hours, "Resets") {
		t.Fatalf("unexpected compact hour reset text: %q", hours)
	}

	minutes := compactResetText(time.Now().Add(40*time.Minute + 30*time.Second))
	if !strings.Contains(minutes, "(40m)") {
		t.Fatalf("unexpected compact minute reset text: %q", minutes)
	}

	days := compactResetText(time.Now().Add(29*24*time.Hour + 23*time.Hour))
	absolute := strings.Split(days, " (")[0]
	if !strings.Contains(days, "(29d)") || strings.Contains(days, "23h") || strings.Contains(absolute, ":") {
		t.Fatalf("unexpected compact day reset text: %q", days)
	}
}
