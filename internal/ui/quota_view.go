package ui

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"

	"github.com/deLiseLINO/codex-quota/internal/api"
	"github.com/deLiseLINO/codex-quota/internal/config"
)

const windowRowIndent = "    "

var partialBarBlocks = [...]string{"", "▏", "▎", "▍", "▌", "▋", "▊", "▉"}

const (
	defaultBarGradientStart = "#6C63FF"
	defaultBarGradientEnd   = "#D46DFF"
	shortBarGradientStart   = "#4285F4"
	shortBarGradientEnd     = "#34A853"
)

func (m Model) renderCompactView() string {
	if len(m.Accounts) == 0 {
		return "No accounts.\n"
	}

	accountWidth := 30
	if m.Width > 0 && m.Width < 120 {
		accountWidth = 24
	}
	normalRows := make([]int, 0, len(m.Accounts))
	exhaustedRows := make([]int, 0, len(m.Accounts))

	for i, acc := range m.Accounts {
		if acc == nil {
			continue
		}
		if m.isCompactAccountExhausted(acc.Key) {
			exhaustedRows = append(exhaustedRows, i)
			continue
		}
		normalRows = append(normalRows, i)
	}

	var s strings.Builder
	m.renderCompactRows(&s, normalRows, accountWidth)

	if len(exhaustedRows) > 0 {
		if len(normalRows) > 0 {
			s.WriteString("\n")
		}
		s.WriteString(CompactExhaustedHeaderStyle.Render("Exhausted accounts"))
		s.WriteString("\n")
		m.renderCompactRows(&s, exhaustedRows, accountWidth)
	}

	return s.String()
}

func (m Model) renderCompactRows(s *strings.Builder, rowIndexes []int, accountWidth int) {
	for _, i := range rowIndexes {
		if i < 0 || i >= len(m.Accounts) {
			continue
		}
		acc := m.Accounts[i]
		if acc == nil {
			continue
		}
		s.WriteString(m.renderCompactAccountRow(i, acc, accountWidth))
		s.WriteString("\n")
	}
}

func (m Model) renderCompactAccountRow(index int, acc *config.Account, accountWidth int) string {
	var s strings.Builder
	isActive := index == m.ActiveAccountIx
	prefix := "  "
	if isActive {
		prefix = "> "
	}

	name := acc.Label
	if name == "" {
		name = acc.SourceLabel()
	}
	subscribed := m.hasSubscription(acc)
	badgeWidth := m.activeSourceBadgesDisplayWidth(acc)
	nameWidth := accountWidth
	if badgeWidth > 0 {
		nameWidth = accountWidth - badgeWidth - 1
		if nameWidth < 4 {
			nameWidth = 4
		}
	}
	name = truncateLabel(name, nameWidth-1)
	alignedName := fmt.Sprintf("%-*s", nameWidth, name)

	s.WriteString(prefix)
	if badgeWidth > 0 {
		s.WriteString(m.renderActiveSourceBadges(acc, isActive))
		s.WriteString(" ")
	}
	if subscribed && isActive {
		s.WriteString(SubscribedLabelActiveStyle.Render(alignedName))
	} else if subscribed {
		s.WriteString(SubscribedLabelMutedStyle.Render(alignedName))
	} else if isActive {
		s.WriteString(TabActiveStyle.Render(alignedName))
	} else {
		s.WriteString(LabelStyle.Render(alignedName))
	}
	s.WriteString(" ")

	if err := m.ErrorsMap[acc.Key]; err != nil {
		status := truncateLabel("Error: "+err.Error(), 24)
		s.WriteString(m.renderCompactStatusRow(status, subscribed))
		return s.String()
	}
	if m.LoadingMap[acc.Key] {
		s.WriteString(m.renderCompactStatusRow("Loading...", subscribed))
		return s.String()
	}

	data, ok := m.UsageData[acc.Key]
	if !ok {
		s.WriteString(m.renderCompactStatusRow("Queued...", subscribed))
		return s.String()
	}

	window, ok := compactPrimaryWindow(data)
	if !ok {
		s.WriteString(m.renderCompactStatusRow("No quota data", subscribed))
		return s.String()
	}

	ratio := m.compactBarRatio(acc.Key, clampRatio(window.LeftPercent/100))
	s.WriteString(renderSmoothBar(m.defaultBarWidth(), ratio, defaultBarGradientStart, defaultBarGradientEnd))
	s.WriteString(" ")
	s.WriteString(m.renderCompactPercent(fmt.Sprintf("%.1f%%", window.LeftPercent), subscribed))
	s.WriteString(ResetTimeStyle.Render(formatResetText(window.ResetAt)))
	return s.String()
}

func (m Model) isCompactAccountExhausted(accountKey string) bool {
	if accountKey == "" {
		return false
	}
	if m.ExhaustedSticky[accountKey] {
		return true
	}
	if m.LoadingMap[accountKey] {
		return false
	}
	if err := m.ErrorsMap[accountKey]; err != nil {
		return false
	}

	data, ok := m.UsageData[accountKey]
	if !ok {
		return false
	}
	return isConfirmedExhausted(data)
}

func (m Model) renderCompactStatusRow(status string, subscribed bool) string {
	row := renderSmoothBar(m.defaultBarWidth(), 0, defaultBarGradientStart, defaultBarGradientEnd)
	row += " "
	row += m.renderCompactPercent("...", subscribed)
	row += ResetTimeStyle.Render(truncateLabel(status, 24))
	return TabInactiveStyle.Render(row)
}

func (m Model) renderCompactPercent(value string, subscribed bool) string {
	if !subscribed {
		return PercentStyle.Render(value)
	}

	// Keep the same fixed width as PercentStyle so the percent/reset columns stay aligned.
	return PercentStyle.Copy().Foreground(lipgloss.Color("177")).Render(value)
}

func (m Model) renderWindowsView() string {
	if len(m.Data.Windows) == 0 {
		return "No quota data.\n"
	}

	var s strings.Builder

	for i, window := range m.Data.Windows {
		if i > 0 {
			s.WriteString("\n")
		}
		s.WriteString(GroupHeaderStyle.Render(windowHeader(window)))
		s.WriteString("\n")
		s.WriteString(m.renderWindowRow(window))
		s.WriteString("\n")
	}

	return s.String()
}

func (m Model) renderWindowsLoadingSkeleton() string {
	windows := make([]api.QuotaWindow, 0, 2)
	if account := m.activeAccount(); account != nil && m.isPaidByKnownPlan(account.Key) {
		windows = append(windows, api.QuotaWindow{Label: "5 hour usage limit", WindowSec: 18000})
	}
	windows = append(windows, api.QuotaWindow{Label: "Weekly usage limit", WindowSec: 604800})
	var s strings.Builder
	for i, window := range windows {
		if i > 0 {
			s.WriteString("\n")
		}
		s.WriteString(GroupHeaderStyle.Render(windowHeader(window)))
		s.WriteString("\n")
		s.WriteString(m.renderWindowStatusRow(window, "Loading..."))
		s.WriteString("\n")
	}
	return s.String()
}

func (m Model) renderWindowRow(window api.QuotaWindow) string {
	var s strings.Builder

	ratio := clampRatio(window.LeftPercent / 100)
	ratio = m.tabWindowRatio(m.activeAccountKey(), window, ratio)

	name := window.Label
	if len(name) > 33 {
		name = name[:30] + "..."
	}
	alignedName := fmt.Sprintf("%-35s", name)
	barWidth := m.barWidthForWindow(window.WindowSec)
	gradientStart, gradientEnd := barGradientForWindow(window.WindowSec)

	s.WriteString(windowRowIndent)
	s.WriteString(LabelStyle.Render(alignedName))
	s.WriteString(" ")
	s.WriteString(renderSmoothBar(barWidth, ratio, gradientStart, gradientEnd))
	s.WriteString(" ")
	s.WriteString(PercentStyle.Render(fmt.Sprintf("%.1f%%", window.LeftPercent)))
	s.WriteString(ResetTimeStyle.Render(formatResetText(window.ResetAt)))

	return s.String()
}

func (m Model) renderWindowStatusRow(window api.QuotaWindow, status string) string {
	var s strings.Builder
	name := window.Label
	if len(name) > 33 {
		name = name[:30] + "..."
	}
	alignedName := fmt.Sprintf("%-35s", name)
	barWidth := m.barWidthForWindow(window.WindowSec)
	gradientStart, gradientEnd := barGradientForWindow(window.WindowSec)

	s.WriteString(windowRowIndent)
	s.WriteString(LabelStyle.Render(alignedName))
	s.WriteString(" ")
	s.WriteString(renderSmoothBar(barWidth, 0, gradientStart, gradientEnd))
	s.WriteString(" ")
	s.WriteString(PercentStyle.Render("..."))
	s.WriteString(ResetTimeStyle.Render(status))
	return s.String()
}

func formatResetText(resetAt time.Time) string {
	if resetAt.IsZero() {
		return "Resets unknown"
	}

	remaining := time.Until(resetAt)
	if remaining <= 0 {
		return "Resets now"
	}

	localReset := resetAt.Local()
	now := time.Now().Local()
	absolute := ""

	if sameDay(localReset, now) {
		absolute = localReset.Format("15:04")
	} else if remaining <= 7*24*time.Hour {
		absolute = localReset.Format("Mon 15:04")
	} else {
		absolute = localReset.Format("01-02 15:04")
	}

	return fmt.Sprintf("Resets %s (%s)", absolute, formatRemainingShort(remaining))
}

func formatRemainingShort(remaining time.Duration) string {
	if remaining <= 0 {
		return "now"
	}

	if remaining < time.Minute {
		return "<1m"
	}

	totalMinutes := int(remaining.Minutes())
	if totalMinutes < 60 {
		return fmt.Sprintf("%dm", totalMinutes)
	}

	totalHours := int(remaining.Hours())
	if totalHours < 24 {
		mins := totalMinutes % 60
		if mins == 0 {
			return fmt.Sprintf("%dh", totalHours)
		}
		return fmt.Sprintf("%dh %dm", totalHours, mins)
	}

	days := totalHours / 24
	hours := totalHours % 24
	if hours == 0 {
		return fmt.Sprintf("%dd", days)
	}
	return fmt.Sprintf("%dd %dh", days, hours)
}

func sameDay(a, b time.Time) bool {
	ay, am, ad := a.Date()
	by, bm, bd := b.Date()
	return ay == by && am == bm && ad == bd
}

func windowHeader(window api.QuotaWindow) string {
	if window.WindowSec == 18000 {
		return "5 hour"
	}
	if window.WindowSec == 604800 {
		return "Weekly"
	}
	return window.Label
}

func compactPrimaryWindow(data api.UsageData) (api.QuotaWindow, bool) {
	for _, window := range data.Windows {
		if window.WindowSec == 604800 {
			return window, true
		}
	}
	if len(data.Windows) == 0 {
		return api.QuotaWindow{}, false
	}
	return data.Windows[0], true
}

func compactPrimaryRatio(data api.UsageData) (float64, bool) {
	window, ok := compactPrimaryWindow(data)
	if !ok {
		return 0, false
	}
	return clampRatio(window.LeftPercent / 100), true
}

func isConfirmedExhausted(data api.UsageData) bool {
	if data.LimitReached {
		return true
	}
	window, ok := compactPrimaryWindow(data)
	if !ok {
		return false
	}
	return clampRatio(window.LeftPercent/100) <= 0
}

func clampRatio(ratio float64) float64 {
	if ratio < 0 {
		return 0
	}
	if ratio > 1 {
		return 1
	}
	return ratio
}

func renderSmoothBar(width int, ratio float64, startHex string, endHex string) string {
	if width <= 0 {
		width = 40
	}

	ratio = clampRatio(ratio)
	total := ratio * float64(width)
	fullCells := int(math.Floor(total))
	if fullCells < 0 {
		fullCells = 0
	}
	if fullCells > width {
		fullCells = width
	}

	fractional := total - float64(fullCells)
	partialIndex := int(math.Round(fractional * 8))
	if partialIndex >= len(partialBarBlocks) {
		partialIndex = 0
		fullCells++
	}
	if fullCells > width {
		fullCells = width
		partialIndex = 0
	}
	if fullCells == width {
		partialIndex = 0
	}

	var b strings.Builder
	if fullCells > 0 {
		for i := 0; i < fullCells; i++ {
			cellStyle := gradientCellStyle(startHex, endHex, i, width)
			b.WriteString(cellStyle.Render("█"))
		}
	}

	usedCells := fullCells
	if partialIndex > 0 && usedCells < width {
		cellStyle := gradientCellStyle(startHex, endHex, usedCells, width)
		b.WriteString(cellStyle.Render(partialBarBlocks[partialIndex]))
		usedCells++
	}

	if usedCells < width {
		b.WriteString(BarEmptyStyle.Render(strings.Repeat("·", width-usedCells)))
	}

	return b.String()
}

func (m Model) defaultBarWidth() int {
	if m.defaultProgress.Width > 0 {
		return m.defaultProgress.Width
	}
	return 40
}

func (m Model) barWidthForWindow(windowSec int64) int {
	if windowSec == 18000 && m.shortProgress.Width > 0 {
		return m.shortProgress.Width
	}
	return m.defaultBarWidth()
}

func barGradientForWindow(windowSec int64) (string, string) {
	if windowSec == 18000 {
		return shortBarGradientStart, shortBarGradientEnd
	}
	return defaultBarGradientStart, defaultBarGradientEnd
}

func gradientCellStyle(startHex string, endHex string, pos int, width int) lipgloss.Style {
	if width <= 1 {
		return lipgloss.NewStyle().Foreground(lipgloss.Color(startHex))
	}
	t := float64(pos) / float64(width-1)
	return lipgloss.NewStyle().Foreground(lipgloss.Color(interpolateHexColor(startHex, endHex, t)))
}

func interpolateHexColor(startHex string, endHex string, t float64) string {
	if t < 0 {
		t = 0
	}
	if t > 1 {
		t = 1
	}
	sr, sg, sb := parseHexColor(startHex)
	er, eg, eb := parseHexColor(endHex)
	r := int(math.Round(float64(sr) + (float64(er)-float64(sr))*t))
	g := int(math.Round(float64(sg) + (float64(eg)-float64(sg))*t))
	b := int(math.Round(float64(sb) + (float64(eb)-float64(sb))*t))
	return fmt.Sprintf("#%02X%02X%02X", clampColor(r), clampColor(g), clampColor(b))
}

func parseHexColor(value string) (int, int, int) {
	trimmed := strings.TrimPrefix(strings.TrimSpace(value), "#")
	if len(trimmed) != 6 {
		return 255, 255, 255
	}
	parsed, err := strconv.ParseUint(trimmed, 16, 32)
	if err != nil {
		return 255, 255, 255
	}
	r := int((parsed >> 16) & 0xFF)
	g := int((parsed >> 8) & 0xFF)
	b := int(parsed & 0xFF)
	return r, g, b
}

func clampColor(v int) int {
	if v < 0 {
		return 0
	}
	if v > 255 {
		return 255
	}
	return v
}
