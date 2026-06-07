package ui

import (
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/deLiseLINO/codex-quota/internal/api"
	"github.com/deLiseLINO/codex-quota/internal/config"
)

func (m Model) activeAccount() *config.Account {
	if len(m.Accounts) == 0 {
		return nil
	}
	if m.ActiveAccountIx < 0 || m.ActiveAccountIx >= len(m.Accounts) {
		return nil
	}
	return m.Accounts[m.ActiveAccountIx]
}

func (m Model) activeAccountKey() string {
	account := m.activeAccount()
	if account == nil {
		return ""
	}
	return account.Key
}

func (m Model) compactVisualOrderIndices() []int {
	if len(m.Accounts) == 0 {
		return nil
	}
	order := make([]int, 0, len(m.Accounts))
	for _, section := range m.compactIndexSections() {
		order = append(order, section.indices...)
	}
	return order
}

func (m *Model) moveActiveAccountCompact(delta int) {
	order := m.compactVisualOrderIndices()
	if len(order) == 0 {
		return
	}

	pos := -1
	for i, idx := range order {
		if idx == m.ActiveAccountIx {
			pos = i
			break
		}
	}
	if pos == -1 {
		m.ActiveAccountIx = order[0]
		return
	}

	next := (pos + delta) % len(order)
	if next < 0 {
		next += len(order)
	}
	m.ActiveAccountIx = order[next]
}

func (m *Model) moveActiveAccountCompactPage(delta int) bool {
	step := m.compactVisibleRowCapacity()
	if step < 1 {
		step = 1
	}
	return m.moveActiveAccountCompactClamped(delta * step)
}

func (m *Model) jumpActiveAccountCompact(position int) bool {
	return m.moveActiveAccountCompactToPosition(position)
}

func (m *Model) moveActiveAccountCompactClamped(delta int) bool {
	order := m.compactVisualOrderIndices()
	if len(order) == 0 {
		return false
	}

	pos := m.compactActivePosition(order)
	if pos < 0 {
		pos = 0
	}
	return m.moveActiveAccountCompactToPosition(pos + delta)
}

func (m *Model) moveActiveAccountCompactToPosition(position int) bool {
	order := m.compactVisualOrderIndices()
	if len(order) == 0 {
		return false
	}
	if position < 0 {
		position = 0
	}
	if position >= len(order) {
		position = len(order) - 1
	}

	next := order[position]
	if m.ActiveAccountIx == next {
		return false
	}
	m.ActiveAccountIx = next
	return true
}

func (m Model) compactActivePosition(order []int) int {
	for i, idx := range order {
		if idx == m.ActiveAccountIx {
			return i
		}
	}
	return -1
}

func (m *Model) syncActiveAccount() {
	m.Loading = true
	m.Err = nil
	m.resetDeleteState()
	m.resetApplyState()
	m.Notice = ""
	m.clearTabWindowAnimations()

	if acc := m.activeAccount(); acc != nil {
		if data, ok := m.UsageData[acc.Key]; ok {
			m.Data = data
			m.Loading = false
			if err := m.ErrorsMap[acc.Key]; err != nil && !m.BackgroundErrorMap[acc.Key] {
				m.Err = err
			} else {
				m.Err = nil
			}
			if !m.CompactMode {
				m.startTabWindowAnimationsFromZero(acc.Key, data, tabSwitchAnimationDuration)
			}
			return
		}
	}
	m.Data = api.UsageData{}
}

func (m *Model) normalizeActiveAccountForView(activeKey string) {
	activeKey = strings.TrimSpace(activeKey)
	if len(m.Accounts) == 0 {
		m.ActiveAccountIx = 0
		return
	}

	if activeKey != "" {
		for i, account := range m.Accounts {
			if account != nil && account.Key == activeKey {
				m.ActiveAccountIx = i
				return
			}
		}
	}

	if m.CompactMode {
		if order := m.compactVisualOrderIndices(); len(order) > 0 {
			m.ActiveAccountIx = order[0]
			return
		}
	}

	m.ActiveAccountIx = 0
}

func (m *Model) syncAndFetchActiveAccount() tea.Cmd {
	m.syncActiveAccount()
	return tea.Batch(m.fetchNextCmd(), m.ensureAnimationTickCmd(), m.nextAutoRefreshCmd(time.Now()))
}
