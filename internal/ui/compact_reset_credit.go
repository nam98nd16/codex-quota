package ui

import (
	"fmt"

	"github.com/deLiseLINO/codex-quota/internal/api"
)

const compactResetCreditWidth = 3

func (m Model) hasCompactResetCreditColumn() bool {
	for _, data := range m.UsageData {
		count, ok := compactResetCreditCount(data)
		if ok && count > 0 {
			return true
		}
	}
	count, ok := compactResetCreditCount(m.Data)
	return ok && count > 0
}

func renderCompactResetCredit(data api.UsageData, width int) string {
	if width <= 0 {
		return ""
	}
	count, ok := compactResetCreditCount(data)
	label := ""
	if ok && count > 0 {
		label = compactResetCreditLabel(count)
	}
	return ResetCreditStyle.Copy().Width(width).Render(truncateLabelStrict(label, width))
}

func compactResetCreditCount(data api.UsageData) (int64, bool) {
	if data.AvailableRateLimitResetCredits == nil {
		return 0, false
	}
	return *data.AvailableRateLimitResetCredits, true
}

func compactResetCreditLabel(count int64) string {
	if count > 9 {
		return "R9+"
	}
	return fmt.Sprintf("R%d", count)
}
