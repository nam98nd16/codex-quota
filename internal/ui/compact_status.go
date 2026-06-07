package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/x/ansi"
)

func (m Model) renderCompactRecordsStatus() string {
	first, last, total := m.compactVisibleRecordRange()
	if total == 0 {
		return ActionMenuHintStyle.Render("Records 0 / 0")
	}

	status := fmt.Sprintf("Records %d-%d / %d", first, last, total)
	loading, errors, exhausted := m.compactRecordCounts()
	if loading > 0 {
		status += fmt.Sprintf(" • Loading %d", loading)
	}
	if errors > 0 {
		status += fmt.Sprintf(" • Errors %d", errors)
	}
	if exhausted > 0 {
		status += fmt.Sprintf(" • Exhausted %d", exhausted)
	}
	for _, part := range m.compactActiveStatusParts() {
		status += " • " + part
	}

	limit := m.preferredContentWidth()
	if limit > 0 && ansi.StringWidth(status) > limit {
		status = ansi.Cut(status, 0, limit)
	}
	return ActionMenuHintStyle.Render(status)
}

func (m Model) compactVisibleRecordRange() (first, last, total int) {
	order := m.compactVisualOrderIndices()
	total = len(order)
	if total == 0 {
		return 0, 0, 0
	}

	viewportHeight := m.compactListViewportHeight()
	columns, columnWidth, _ := m.compactColumnLayout()
	visible := m.compactRenderedLines(viewportHeight, columns, columnWidth, 0)
	positionByAccount := make(map[int]int, len(order))
	for i, accountIndex := range order {
		positionByAccount[accountIndex] = i + 1
	}

	for _, row := range visible {
		for _, accountIndex := range row.accountIndices {
			position, ok := positionByAccount[accountIndex]
			if !ok {
				continue
			}
			if first == 0 || position < first {
				first = position
			}
			if position > last {
				last = position
			}
		}
	}

	if first == 0 {
		position := positionByAccount[m.ActiveAccountIx]
		if position == 0 {
			position = 1
		}
		first = position
		last = position
	}
	return first, last, total
}

func (m Model) compactRecordCounts() (loading, errors, exhausted int) {
	for _, accountIndex := range m.compactVisualOrderIndices() {
		if accountIndex < 0 || accountIndex >= len(m.Accounts) {
			continue
		}
		account := m.Accounts[accountIndex]
		if account == nil {
			continue
		}
		if m.LoadingMap[account.Key] {
			loading++
		}
		if err := m.ErrorsMap[account.Key]; err != nil && !(m.BackgroundErrorMap[account.Key] && hasRenderableQuotaData(m.UsageData[account.Key])) {
			errors++
		}
		if m.isCompactAccountExhausted(account.Key) {
			exhausted++
		}
	}
	return loading, errors, exhausted
}

func (m Model) compactActiveStatusParts() []string {
	account := m.activeAccount()
	if account == nil {
		return nil
	}

	label := truncateLabelStrict(m.displayAccountLabel(account), 40)
	parts := []string{"Active " + label}
	if err := m.ErrorsMap[account.Key]; err != nil && !(m.BackgroundErrorMap[account.Key] && hasRenderableQuotaData(m.UsageData[account.Key])) {
		return append(parts, "Error")
	}
	if m.LoadingMap[account.Key] {
		return append(parts, "Loading")
	}

	data, ok := m.UsageData[account.Key]
	if !ok && account.Key == m.activeAccountKey() && hasRenderableQuotaData(m.Data) {
		data = m.Data
		ok = true
	}
	if !ok {
		return append(parts, "Queued")
	}

	window, ok := compactPrimaryWindow(data)
	if !ok {
		return append(parts, "No quota")
	}

	windowLabel := strings.TrimSpace(windowHeader(window))
	if windowLabel == "" {
		windowLabel = "Quota"
	}
	parts = append(parts, fmt.Sprintf("%s %.0f%%", windowLabel, window.LeftPercent))
	if reset := compactResetText(window.ResetAt); reset != "" {
		parts = append(parts, reset)
	}
	return parts
}

func compactResetText(resetAt time.Time) string {
	if resetAt.IsZero() {
		return "unknown"
	}

	remaining := time.Until(resetAt)
	if remaining <= 0 {
		return "now"
	}

	localReset := resetAt.Local()
	now := time.Now().Local()
	absolute := localReset.Format("01-02")
	if sameDay(localReset, now) {
		absolute = localReset.Format("15:04")
	} else if remaining <= 7*24*time.Hour {
		absolute = localReset.Format("Mon 15:04")
	}

	return fmt.Sprintf("%s (%s)", absolute, compactRemainingUnit(remaining))
}

func compactRemainingUnit(remaining time.Duration) string {
	if remaining < time.Minute {
		return "<1m"
	}
	if remaining < time.Hour {
		return fmt.Sprintf("%dm", int(remaining.Minutes()))
	}
	if remaining < 24*time.Hour {
		return fmt.Sprintf("%dh", int(remaining.Hours()))
	}
	return fmt.Sprintf("%dd", int(remaining.Hours())/24)
}
