package ui

import "strings"

const (
	compactColumnGap              = 3
	compactMaxColumns             = 4
	compactMinColumnViewportWidth = 76
)

type compactListSection struct {
	header string
	rows   []compactListRow
}

func (m Model) renderCompactRowsWithin(viewportHeight int) string {
	columns, columnWidth, gap := m.compactColumnLayout()
	lines := m.compactRenderedLines(viewportHeight, columns, columnWidth, gap)

	var s strings.Builder
	for _, line := range lines {
		s.WriteString(line.line)
		s.WriteString("\n")
	}
	return s.String()
}

func (m Model) compactRenderedLines(viewportHeight int, columns int, columnWidth int, gap int) []compactListRow {
	if columns < 1 {
		columns = 1
	}
	start := m.clampedCompactScrollOffset(len(m.compactVisualOrderIndices()), m.compactVisibleRowCapacity())
	skip := start
	lines := []compactListRow{}

	for _, section := range m.compactListSectionsForWidth(columnWidth) {
		if skip >= len(section.rows) {
			skip -= len(section.rows)
			continue
		}

		rows := section.rows[skip:]
		skip = 0
		if len(rows) == 0 {
			continue
		}

		if section.header != "" {
			if viewportHeight > 0 && len(lines) >= viewportHeight {
				break
			}
			lines = append(lines, compactListRow{line: section.header, accountIndex: -1})
		}

		availableLines := 0
		if viewportHeight > 0 {
			availableLines = viewportHeight - len(lines)
			if availableLines <= 0 {
				break
			}
		}
		lines = append(lines, renderCompactSectionGrid(rows, columns, columnWidth, gap, availableLines)...)
		if viewportHeight > 0 && len(lines) >= viewportHeight {
			break
		}
	}

	return lines
}

func (m Model) compactListSectionsForWidth(width int) []compactListSection {
	accountWidth := m.compactAccountWidthForViewport(width)
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

	sections := []compactListSection{}
	if len(normalRows) > 0 {
		sections = append(sections, compactListSection{rows: m.compactAccountRowsForWidth(normalRows, accountWidth, width)})
	}
	if len(exhaustedRows) > 0 {
		sections = append(sections, compactListSection{
			header: CompactExhaustedHeaderStyle.Render("Exhausted accounts"),
			rows:   m.compactAccountRowsForWidth(exhaustedRows, accountWidth, width),
		})
	}
	return sections
}

func renderCompactSectionGrid(rows []compactListRow, columns int, columnWidth int, gap int, maxLines int) []compactListRow {
	if columns < 1 {
		columns = 1
	}
	if maxLines > 0 {
		capacity := maxLines * columns
		if len(rows) > capacity {
			rows = rows[:capacity]
		}
	}
	if len(rows) == 0 {
		return nil
	}

	columnLineWidth := compactColumnLineWidth(columnWidth)
	rowCount := (len(rows) + columns - 1) / columns
	if maxLines > 0 && rowCount > maxLines {
		rowCount = maxLines
	}

	lines := make([]compactListRow, 0, rowCount)
	for row := 0; row < rowCount; row++ {
		var s strings.Builder
		accountIndices := []int{}
		for column := 0; column < columns; column++ {
			if column > 0 {
				s.WriteString(strings.Repeat(" ", gap))
			}

			index := row + column*rowCount
			if index < len(rows) {
				s.WriteString(padANSI(rows[index].line, columnLineWidth))
				accountIndices = append(accountIndices, rows[index].accountIndices...)
			} else {
				s.WriteString(strings.Repeat(" ", columnLineWidth))
			}
		}
		lines = append(lines, compactListRow{line: s.String(), accountIndex: -1, accountIndices: accountIndices})
	}
	return lines
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
	if !m.CompactMode || contentWidth <= 0 {
		return 1, contentWidth, 0
	}

	columns = (contentWidth + compactColumnGap) / (compactMinColumnViewportWidth + compactColumnGap)
	if columns < 1 {
		columns = 1
	}
	if columns > compactMaxColumns {
		columns = compactMaxColumns
	}
	if columns == 1 {
		return 1, contentWidth, 0
	}
	return columns, (contentWidth - compactColumnGap*(columns-1)) / columns, compactColumnGap
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
