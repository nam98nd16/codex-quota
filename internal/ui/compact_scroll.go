package ui

import "github.com/charmbracelet/lipgloss"

const compactMouseScrollRows = 3

func (m Model) compactListViewportHeight() int {
	if !m.CompactMode || m.Height <= 0 {
		return 0
	}

	prefixHeight := lipgloss.Height(m.compactViewPrefix())
	footerHeight := lipgloss.Height(HelpStyle.Render("\n" + m.renderFooter()))
	available := m.Height - 2 - prefixHeight - footerHeight
	if available < 0 {
		return 0
	}
	return available
}

func (m Model) compactViewPrefix() string {
	prefix := m.renderHeader() + "\n"
	if len(m.Accounts) > 0 {
		prefix += "\n"
	}
	return prefix
}

func (m Model) clampedCompactScrollOffset(rowCount, viewportHeight int) int {
	if viewportHeight <= 0 || rowCount <= viewportHeight {
		return 0
	}
	maxOffset := rowCount - viewportHeight
	if m.CompactScrollOffset < 0 {
		return 0
	}
	if m.CompactScrollOffset > maxOffset {
		return maxOffset
	}
	return m.CompactScrollOffset
}

func (m *Model) clampCompactScrollOffset() {
	if !m.CompactMode {
		m.CompactScrollOffset = 0
		return
	}
	m.CompactScrollOffset = m.clampedCompactScrollOffset(len(m.compactRows()), m.compactListViewportHeight())
}

func (m *Model) scrollCompactRows(delta int) {
	m.CompactScrollOffset += delta
	m.clampCompactScrollOffset()
}

func (m *Model) ensureCompactActiveVisible() {
	viewportHeight := m.compactListViewportHeight()
	if viewportHeight <= 0 {
		return
	}

	rows := m.compactRows()
	activeRow := -1
	for i, row := range rows {
		if row.accountIndex == m.ActiveAccountIx {
			activeRow = i
			break
		}
	}
	if activeRow < 0 {
		m.clampCompactScrollOffset()
		return
	}

	offset := m.clampedCompactScrollOffset(len(rows), viewportHeight)
	if activeRow < offset {
		offset = activeRow
	} else if activeRow >= offset+viewportHeight {
		offset = activeRow - viewportHeight + 1
	}
	m.CompactScrollOffset = offset
	m.clampCompactScrollOffset()
}

func (m Model) compactScrollEnabled() bool {
	if !m.CompactMode || len(m.Accounts) == 0 {
		return false
	}
	if m.UpdatePromptVisible || m.HelpVisible || m.SettingsVisible || m.AddAccountLoginVisible || m.ActionMenuVisible || m.ShowInfo {
		return false
	}
	if m.DeleteSourceSelect || m.DeleteConfirm || m.ApplyTargetSelect || m.ApplyConfirm {
		return false
	}
	return m.Err == nil && m.Notice == ""
}
