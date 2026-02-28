package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

func TestRenderSmoothBarWidthIsStable(t *testing.T) {
	bar := renderSmoothBar(20, 0.42, defaultBarGradientStart, defaultBarGradientEnd)
	if got := ansi.StringWidth(ansi.Strip(bar)); got != 20 {
		t.Fatalf("expected smooth bar width 20, got %d", got)
	}
}

func TestRenderSmoothBarClampBounds(t *testing.T) {
	below := ansi.Strip(renderSmoothBar(10, -1, defaultBarGradientStart, defaultBarGradientEnd))
	if strings.Contains(below, "█") || strings.ContainsAny(below, "▏▎▍▌▋▊▉") {
		t.Fatalf("expected no filled cells for ratio below 0, got %q", below)
	}

	above := ansi.Strip(renderSmoothBar(10, 2, defaultBarGradientStart, defaultBarGradientEnd))
	if strings.Count(above, "█") != 10 {
		t.Fatalf("expected full bar for ratio above 1, got %q", above)
	}
}

func TestRenderSmoothBarUsesPartialBlock(t *testing.T) {
	bar := ansi.Strip(renderSmoothBar(10, 0.46, defaultBarGradientStart, defaultBarGradientEnd))
	if !strings.ContainsAny(bar, "▏▎▍▌▋▊▉") {
		t.Fatalf("expected partial block in smooth bar, got %q", bar)
	}
}

func TestRenderSmoothBarZeroAndFull(t *testing.T) {
	zero := ansi.Strip(renderSmoothBar(8, 0, defaultBarGradientStart, defaultBarGradientEnd))
	if strings.Contains(zero, "█") || strings.ContainsAny(zero, "▏▎▍▌▋▊▉") {
		t.Fatalf("expected zero bar to have no filled blocks, got %q", zero)
	}
	if zero != strings.Repeat("·", 8) {
		t.Fatalf("expected dotted empty track for zero ratio, got %q", zero)
	}

	full := ansi.Strip(renderSmoothBar(8, 1, defaultBarGradientStart, defaultBarGradientEnd))
	if full != strings.Repeat("█", 8) {
		t.Fatalf("expected fully filled bar, got %q", full)
	}
}
