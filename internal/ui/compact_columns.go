package ui

import "strings"

const (
	compactColumnGap              = 4
	compactTwoColumnMinWidth      = 170
	compactMinColumnViewportWidth = 81
)

func (m Model) renderCompactRowsWithin(viewportHeight int) string {
	columns, columnWidth, gap := m.compactColumnLayout()
	if viewportHeight <= 0 || columns == 1 {
		return m.renderCompactRowsLinear(m.compactRows(), viewportHeight)
	}

	rows := m.compactRowsForWidth(columnWidth)
	capacity := viewportHeight * columns
	start := m.clampedCompactScrollOffset(len(rows), capacity)
	visible := rows[start:min(start+capacity, len(rows))]
	columnLineWidth := compactColumnLineWidth(columnWidth)

	var s strings.Builder
	for i := 0; i < viewportHeight && i < len(visible); i++ {
		left := visible[i].line
		rightIndex := i + viewportHeight
		if rightIndex < len(visible) {
			s.WriteString(padANSI(left, columnLineWidth))
			s.WriteString(strings.Repeat(" ", gap))
			s.WriteString(visible[rightIndex].line)
		} else {
			s.WriteString(left)
		}
		s.WriteString("\n")
	}
	return s.String()
}

func (m Model) renderCompactRowsLinear(rows []compactListRow, viewportHeight int) string {
	if viewportHeight > 0 && len(rows) > viewportHeight {
		start := m.clampedCompactScrollOffset(len(rows), viewportHeight)
		rows = rows[start:min(start+viewportHeight, len(rows))]
	}

	var s strings.Builder
	for _, row := range rows {
		s.WriteString(row.line)
		s.WriteString("\n")
	}
	return s.String()
}

func (m Model) compactRowsForWidth(width int) []compactListRow {
	if width <= 0 {
		return m.compactRows()
	}
	narrow := m
	narrow.Width = width
	return narrow.compactRows()
}

func (m Model) compactColumnLayout() (columns int, columnWidth int, gap int) {
	contentWidth := m.compactContentWidth()
	if m.CompactMode && m.Width >= compactTwoColumnMinWidth && contentWidth >= (compactMinColumnViewportWidth*2)+compactColumnGap {
		return 2, (contentWidth - compactColumnGap) / 2, compactColumnGap
	}
	return 1, contentWidth, 0
}

func (m Model) compactContentWidth() int {
	if m.Width <= 0 {
		return m.preferredContentWidth()
	}
	contentWidth := m.Width - 4
	if contentWidth < 1 {
		return m.Width
	}
	return contentWidth
}

func compactColumnLineWidth(columnWidth int) int {
	if columnWidth <= 12 {
		return columnWidth
	}
	return columnWidth - 4
}
