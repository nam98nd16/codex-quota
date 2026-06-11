package ui

import (
	"math"
	"sort"
	"strings"
	"time"

	"github.com/deLiseLINO/codex-quota/internal/config"
)

func (m Model) sortCompactIndices(indices []int) {
	if m.CompactSort == compactSortOriginal || len(indices) < 2 {
		return
	}
	now := time.Now()
	sort.SliceStable(indices, func(left, right int) bool {
		leftAccount := m.Accounts[indices[left]]
		rightAccount := m.Accounts[indices[right]]
		switch m.CompactSort {
		case compactSortSubscriptions:
			return m.compactSubscriptionSortLess(leftAccount, rightAccount, now)
		case compactSortQuota:
			return m.compactQuotaSortKey(leftAccount) < m.compactQuotaSortKey(rightAccount)
		case compactSortReset:
			leftReset := m.compactResetSortKey(leftAccount, now)
			rightReset := m.compactResetSortKey(rightAccount, now)
			if leftReset != rightReset {
				return leftReset < rightReset
			}
			return m.compactNameSortKey(leftAccount) < m.compactNameSortKey(rightAccount)
		case compactSortSource:
			return m.compactSourceSortKey(leftAccount) < m.compactSourceSortKey(rightAccount)
		case compactSortStatus:
			return m.compactStatusSortKey(leftAccount) < m.compactStatusSortKey(rightAccount)
		default:
			return false
		}
	})
}

func (m Model) compactSubscriptionSortLess(left, right *config.Account, now time.Time) bool {
	leftSubscribed := m.hasSubscription(left)
	rightSubscribed := m.hasSubscription(right)
	if leftSubscribed != rightSubscribed {
		return leftSubscribed
	}

	leftQuota := m.compactQuotaSortKey(left)
	rightQuota := m.compactQuotaSortKey(right)
	if leftQuota != rightQuota {
		return leftQuota < rightQuota
	}

	leftReset := m.compactResetSortKey(left, now)
	rightReset := m.compactResetSortKey(right, now)
	if leftReset != rightReset {
		return leftReset < rightReset
	}

	return m.compactNameSortKey(left) < m.compactNameSortKey(right)
}

func (m Model) compactQuotaSortKey(account *config.Account) int {
	if account == nil {
		return 101
	}
	if window, ok := compactPrimaryWindow(m.UsageData[account.Key]); ok {
		return int(math.Round(clampRatio(window.LeftPercent/100) * 100))
	}
	if m.compactAccountHasForegroundError(account) {
		return 102
	}
	return 101
}

func (m Model) compactResetSortKey(account *config.Account, now time.Time) int64 {
	if account == nil {
		return 1<<62 - 1
	}
	if window, ok := compactPrimaryWindow(m.UsageData[account.Key]); ok && !window.ResetAt.IsZero() {
		fetchedAt := m.UsageDataFetchedAt[account.Key]
		if fetchedAt.IsZero() {
			fetchedAt = now
		}
		return compactResetHorizonSortKey(window.ResetAt.Sub(fetchedAt))
	}
	return 1<<62 - 1
}

func compactResetHorizonSortKey(horizon time.Duration) int64 {
	if horizon <= 0 {
		return 0
	}
	if horizon < time.Minute {
		return 1
	}
	if horizon < time.Hour {
		return 1_000 + int64(horizon/time.Minute)
	}
	if horizon < 24*time.Hour {
		return 1_000_000 + int64(horizon/time.Hour)
	}
	return 2_000_000_000 + int64(horizon/(24*time.Hour))
}

func (m Model) compactSourceSortKey(account *config.Account) string {
	if account == nil {
		return ""
	}
	return strings.ToLower(account.SourceLabel() + " " + m.compactNameSortKey(account))
}

func (m Model) compactNameSortKey(account *config.Account) string {
	if account == nil {
		return ""
	}
	return strings.ToLower(m.displayAccountLabel(account))
}

func (m Model) compactStatusSortKey(account *config.Account) string {
	if account == nil {
		return "9"
	}
	status := "4-available"
	if m.compactAccountHasForegroundError(account) {
		status = "0-error"
	} else if m.LoadingMap[account.Key] || m.BackgroundLoadingMap[account.Key] {
		status = "1-loading"
	} else if _, ok := m.UsageData[account.Key]; !ok {
		status = "2-queued"
	} else if m.isCompactAccountExhausted(account.Key) {
		status = "3-exhausted"
	}
	return status + " " + strings.ToLower(m.displayAccountLabel(account))
}
