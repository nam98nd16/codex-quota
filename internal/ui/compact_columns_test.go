package ui

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"
)

func TestCompactColumnLayoutSupportsThreeAndFourColumns(t *testing.T) {
	m := testCompactScrollModel(80, 240, 24)
	columns, _, _ := m.compactColumnLayout()
	if columns != 3 {
		t.Fatalf("compact columns = %d, want 3", columns)
	}

	m.Width = 320
	columns, _, _ = m.compactColumnLayout()
	if columns != 4 {
		t.Fatalf("compact columns = %d, want 4", columns)
	}
}

func TestCompactExhaustedHeaderSpansFullWidthInMultiColumn(t *testing.T) {
	m := testCompactScrollModel(24, 240, 24)
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
	m := testCompactScrollModel(80, 320, 24)
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

func TestCompactActiveExhaustedAccountRemainsVisibleInMultiColumn(t *testing.T) {
	m := testCompactScrollModel(24, 240, 24)
	for i := 18; i < 24; i++ {
		m.ExhaustedSticky[m.Accounts[i].Key] = true
	}
	m.ActiveAccountIx = 23
	m.ensureCompactActiveVisible()

	rendered := ansi.Strip(m.renderCompactViewWithin(m.compactListViewportHeight()))
	if !strings.Contains(rendered, "> user23@example") {
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
