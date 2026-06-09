package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"github.com/deLiseLINO/codex-quota/internal/api"
	"github.com/deLiseLINO/codex-quota/internal/config"
)

type compactListRow struct {
	line           string
	accountIndex   int
	accountIndices []int
}

func (m Model) renderCompactView() string {
	return m.renderCompactViewWithin(0)
}

func (m Model) renderCompactViewWithin(maxRows int) string {
	if len(m.Accounts) == 0 {
		return "No accounts.\n"
	}

	return m.renderCompactRowsWithin(maxRows)
}

func (m Model) compactRows() []compactListRow {
	accountWidth := m.compactAccountWidth()
	rows := []compactListRow{}
	sections := m.compactIndexSections()
	if len(sections) == 0 {
		return []compactListRow{{line: ActionMenuHintStyle.Render("No matching accounts"), accountIndex: -1}}
	}
	for _, section := range sections {
		if section.header != "" {
			if len(rows) > 0 {
				rows = append(rows, compactListRow{accountIndex: -1})
			}
			rows = append(rows, compactListRow{line: CompactExhaustedHeaderStyle.Render(section.header), accountIndex: -1})
		}
		rows = append(rows, m.compactAccountRows(section.indices, accountWidth)...)
	}

	return rows
}

func (m Model) compactAccountRows(rowIndexes []int, accountWidth int) []compactListRow {
	return m.compactAccountRowsForWidth(rowIndexes, accountWidth, m.preferredContentWidth())
}

func (m Model) compactAccountRowsForWidth(rowIndexes []int, accountWidth int, limit int) []compactListRow {
	if limit <= 0 {
		limit = m.preferredContentWidth()
	}
	if limit <= 0 && m.Width > 0 {
		limit = m.Width
	}
	density := compactAccountRowDensity(limit)
	renderer := m
	if limit > 0 {
		renderer.Width = limit
	}
	rows := make([]compactListRow, 0, len(rowIndexes))
	for _, i := range rowIndexes {
		if i < 0 || i >= len(m.Accounts) {
			continue
		}
		acc := m.Accounts[i]
		if acc == nil {
			continue
		}
		row := renderer.renderCompactAccountRow(i, acc, accountWidth, density, limit)
		// Guard against style-induced line wraps on very narrow terminals.
		row = strings.ReplaceAll(row, "\n", " ")
		if limit > 0 && ansi.StringWidth(row) > limit {
			row = ansi.Cut(row, 0, limit)
		}
		rows = append(rows, compactListRow{line: row, accountIndex: i, accountIndices: []int{i}})
	}
	return rows
}

func (m Model) renderCompactAccountRow(index int, acc *config.Account, accountWidth int, density compactRowDensity, rowWidth int) string {
	var s strings.Builder
	isActive := index == m.ActiveAccountIx
	prefix := "  "
	if isActive {
		prefix = "● "
	}

	name := m.displayAccountLabel(acc)
	subscribed := m.hasSubscription(acc)
	badgeWidth := m.activeSourceBadgesDisplayWidth(acc)
	if density == compactRowDensityUltra {
		badgeWidth = 0
	}
	refreshIndicator := m.renderAccountRefreshIndicator(acc, isActive)
	refreshWidth := m.accountRefreshIndicatorDisplayWidth(acc)
	nameWidth := accountWidth
	if badgeWidth > 0 {
		nameWidth = accountWidth - badgeWidth - 1
	}
	if refreshWidth > 0 {
		nameWidth -= refreshWidth + 1
	}
	if nameWidth < 4 {
		nameWidth = 4
	}
	name = truncateLabel(name, nameWidth-1)
	alignedName := fmt.Sprintf("%-*s", nameWidth, name)
	leftWidth := ansi.StringWidth(prefix) + nameWidth + 1
	if badgeWidth > 0 {
		leftWidth += badgeWidth + 1
	}
	if refreshWidth > 0 {
		leftWidth += refreshWidth + 1
	}
	barWidth, percentWidth, resetWidth := m.compactRowLayout(leftWidth, density, rowWidth)

	s.WriteString(prefix)
	if badgeWidth > 0 {
		s.WriteString(m.renderActiveSourceBadges(acc, isActive))
		s.WriteString(" ")
	}
	if refreshWidth > 0 {
		s.WriteString(refreshIndicator)
		s.WriteString(" ")
	}
	labelStyle := LabelStyle
	switch {
	case subscribed && isActive:
		labelStyle = SubscribedLabelActiveStyle
	case subscribed:
		labelStyle = SubscribedLabelMutedStyle
	case isActive:
		labelStyle = TabActiveStyle
	}
	if density == compactRowDensityUltra {
		labelStyle = m.ultraDenseAppliedLabelStyle(acc, isActive, labelStyle)
	}
	s.WriteString(labelStyle.Render(alignedName))
	s.WriteString(" ")

	if err := m.ErrorsMap[acc.Key]; err != nil && !(m.BackgroundErrorMap[acc.Key] && hasRenderableQuotaData(m.UsageData[acc.Key])) {
		status := "Error: " + compactErrorText(err)
		s.WriteString(m.renderCompactStatusRow(status, subscribed, barWidth, percentWidth, resetWidth))
		return s.String()
	}
	if m.LoadingMap[acc.Key] {
		s.WriteString(m.renderCompactStatusRow("Loading...", subscribed, barWidth, percentWidth, resetWidth))
		return s.String()
	}

	data, ok := m.UsageData[acc.Key]
	if !ok {
		s.WriteString(m.renderCompactStatusRow("Queued...", subscribed, barWidth, percentWidth, resetWidth))
		return s.String()
	}

	window, ok := compactPrimaryWindow(data)
	if !ok {
		s.WriteString(m.renderCompactStatusRow("No quota data", subscribed, barWidth, percentWidth, resetWidth))
		return s.String()
	}

	ratio := m.compactBarRatio(acc.Key, clampRatio(window.LeftPercent/100))
	gradientStart, gradientEnd := barGradientForWindow(window.WindowSec)
	s.WriteString(renderSmoothBar(barWidth, ratio, gradientStart, gradientEnd))
	s.WriteString(" ")
	s.WriteString(m.renderCompactPercentValue(window.LeftPercent, subscribed, percentWidth))
	resetText := compactResetText(window.ResetAt)
	if density.usesRelativeReset() {
		resetText = compactDenseResetText(window.ResetAt)
	}
	reset := truncateLabelStrict(resetText, resetWidth)
	if resetWidth > 0 && strings.TrimSpace(reset) != "" {
		s.WriteString(ResetTimeStyle.Copy().Width(resetWidth).Render(reset))
	}
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
	data, ok := m.UsageData[accountKey]
	if !ok {
		return false
	}
	if err := m.ErrorsMap[accountKey]; err != nil && !(m.BackgroundErrorMap[accountKey] && hasRenderableQuotaData(data)) {
		return false
	}
	return isConfirmedExhausted(data)
}

func hasRenderableQuotaData(data api.UsageData) bool {
	return len(data.Windows) > 0
}

func (m Model) renderCompactStatusRow(status string, subscribed bool, barWidth, percentWidth, resetWidth int) string {
	statusWidth := barWidth + 1 + percentWidth
	style := ActionMenuHintStyle.Copy().Width(statusWidth)
	if strings.HasPrefix(strings.ToLower(strings.TrimSpace(status)), "error") {
		style = ErrorStyle.Copy().Width(statusWidth)
	}
	row := style.Render(truncateLabelStrict(compactPlaceholderStatus(status), statusWidth))
	if resetWidth > 0 {
		row += ResetTimeStyle.Copy().Width(resetWidth).Render("")
	}
	return row
}

func compactPlaceholderStatus(status string) string {
	switch strings.TrimSpace(status) {
	case "Loading...":
		return "loading"
	case "Queued...":
		return "queued"
	case "No quota data":
		return "no quota"
	}
	return strings.TrimSpace(status)
}

func (m Model) renderCompactPercent(value string, subscribed bool, width int) string {
	value = truncateLabelStrict(value, width)
	style := PercentStyle.Copy().Width(width)
	if !subscribed {
		return style.Render(value)
	}

	return style.Copy().Foreground(lipgloss.Color("177")).Render(value)
}

func (m Model) renderCompactPercentValue(leftPercent float64, subscribed bool, width int) string {
	value := fmt.Sprintf("%.0f%%", leftPercent)
	style := compactPercentSeverityStyle(leftPercent).Copy().Width(width)
	if subscribed && leftPercent > 25 {
		style = style.Foreground(lipgloss.Color("177"))
	}
	return style.Render(truncateLabelStrict(value, width))
}

func compactPercentSeverityStyle(leftPercent float64) lipgloss.Style {
	switch {
	case leftPercent <= 10:
		return PercentDangerStyle
	case leftPercent <= 25:
		return PercentWarningStyle
	default:
		return PercentStyle
	}
}

func (m Model) compactAccountWidth() int {
	return m.compactAccountWidthForViewport(m.Width)
}

func (m Model) compactAccountWidthForViewport(width int) int {
	if width <= 0 {
		width = m.preferredContentWidth()
	}
	switch {
	case width >= 140:
		return 30
	case width >= 120:
		return 24
	case width >= 100:
		return 20
	case width >= 84:
		return 16
	case width >= 72:
		return 18
	case width >= 60:
		return 18
	case width >= 44:
		return 12
	case width >= 36:
		return 10
	default:
		return 8
	}
}

func (m Model) compactAccountWidthForColumn(width int, columns int) int {
	if columns < 5 {
		return m.compactAccountWidthForViewport(width)
	}
	switch {
	case width >= 48:
		return 16
	case width >= 44:
		return 14
	case width >= 40:
		return 12
	case width >= 36:
		return 10
	default:
		return 8
	}
}

func (m Model) compactRowLayout(leftWidth int, density compactRowDensity, rowWidth int) (barWidth, percentWidth, resetWidth int) {
	contentWidth := rowWidth
	if contentWidth <= 0 {
		contentWidth = m.preferredContentWidth()
	}
	switch density {
	case compactRowDensityUltra:
		barWidth = min(m.defaultBarWidth(), 6)
		percentWidth = 4
		resetWidth = 3
	case compactRowDensityDense:
		barWidth = min(m.defaultBarWidth(), 10)
		percentWidth = 4
		resetWidth = 4
	default:
		barWidth = min(m.defaultBarWidth(), compactBarMaxWidth(contentWidth))
		percentWidth = 5
		resetWidth = compactResetWidth(contentWidth)
	}

	available := contentWidth - leftWidth
	if available <= 0 {
		return 6, 3, 0
	}

	const (
		minBarWidth     = 4
		minPercentWidth = 4
		minResetWidth   = 0
		gapWidth        = 1
		resetMarginLeft = 2
	)

	used := barWidth + gapWidth + percentWidth + resetMarginLeft + resetWidth
	if density == compactRowDensityUltra || density == compactRowDensityDense {
		barBudget := available - gapWidth - percentWidth - resetMarginLeft - resetWidth
		if barBudget > barWidth {
			barWidth = min(barBudget, min(m.defaultBarWidth(), compactDenseBarMaxWidth(density)))
			used = barWidth + gapWidth + percentWidth + resetMarginLeft + resetWidth
		}
	}
	shortage := used - available
	if shortage <= 0 {
		return
	}

	reduce := func(current, minimum int) int {
		if shortage <= 0 {
			return current
		}
		can := current - minimum
		if can <= 0 {
			return current
		}
		if can > shortage {
			can = shortage
		}
		shortage -= can
		return current - can
	}

	if density == compactRowDensityUltra {
		resetWidth = reduce(resetWidth, minResetWidth)
		barWidth = reduce(barWidth, minBarWidth)
		return
	}

	barWidth = reduce(barWidth, minBarWidth)
	resetWidth = reduce(resetWidth, minResetWidth)
	percentWidth = reduce(percentWidth, minPercentWidth)

	return
}

func compactBarMaxWidth(contentWidth int) int {
	switch {
	case contentWidth <= 60:
		return 18
	case contentWidth <= 72:
		return 22
	case contentWidth <= 88:
		return 26
	case contentWidth <= 120:
		return 32
	default:
		return 40
	}
}

func compactResetWidth(contentWidth int) int {
	switch {
	case contentWidth <= 60:
		return 10
	case contentWidth <= 72:
		return 11
	case contentWidth <= 88:
		return 12
	default:
		return 14
	}
}
