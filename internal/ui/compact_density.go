package ui

import "time"

const compactDenseRowViewportWidth = 48

func compactDenseAccountRow(limit int) bool {
	return limit > 0 && limit <= compactDenseRowViewportWidth
}

func compactDenseResetText(resetAt time.Time) string {
	if resetAt.IsZero() {
		return "unknown"
	}

	remaining := time.Until(resetAt)
	if remaining <= 0 {
		return "now"
	}
	return compactRemainingUnit(remaining)
}
