package ui

import (
	"math"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/deLiseLINO/codex-quota/internal/api"
	"github.com/deLiseLINO/codex-quota/internal/config"
)

const (
	smartSwitchLowQuotaThresholdPercent = 10.0
	smartSwitchFastRefreshInterval      = 10 * time.Second
)

type replacementCandidateRank struct {
	subscribed   bool
	blockingErr  bool
	hasWeekly    bool
	weeklyLeft   float64
	hasFallback  bool
	fallbackLeft float64
	index        int
}

func (m Model) beginSmartSwitchActive() (tea.Model, tea.Cmd) {
	if m.activeAccount() == nil {
		return m, nil
	}
	m.PendingSmartSwitchKey = m.activeAccountKey()
	return m.beginRefreshActive()
}

func (m Model) smartSwitchInterval(accountKey string) (time.Duration, bool) {
	if !m.Settings.AutoSwitchExhausted {
		return 0, false
	}
	if strings.TrimSpace(accountKey) == "" || accountKey != m.activeAccountKey() {
		return 0, false
	}
	if m.LoadingMap[accountKey] || m.BackgroundLoadingMap[accountKey] || m.accountHasBlockingError(accountKey) {
		return 0, false
	}

	data, ok := m.UsageData[accountKey]
	if !ok {
		return 0, false
	}
	window, ok := watchedAutoSwitchWindow(data)
	if !ok || window.LeftPercent > smartSwitchLowQuotaThresholdPercent {
		return 0, false
	}
	return smartSwitchFastRefreshInterval, true
}

func watchedAutoSwitchWindow(data api.UsageData) (api.QuotaWindow, bool) {
	if window, ok := quotaWindowByDuration(data, 18000); ok {
		return window, true
	}
	return quotaWindowByDuration(data, 604800)
}

func weeklyQuotaWindow(data api.UsageData) (api.QuotaWindow, bool) {
	return quotaWindowByDuration(data, 604800)
}

func quotaWindowByDuration(data api.UsageData, windowSec int64) (api.QuotaWindow, bool) {
	for _, window := range data.Windows {
		if window.WindowSec == windowSec {
			return window, true
		}
	}
	return api.QuotaWindow{}, false
}

func (m Model) bestReplacementAccount(excludeKey string) *config.Account {
	var (
		best     *config.Account
		bestRank replacementCandidateRank
		haveBest bool
	)

	for index, account := range m.Accounts {
		if account == nil || strings.TrimSpace(account.Key) == "" || account.Key == excludeKey {
			continue
		}
		if m.isCompactAccountExhausted(account.Key) {
			continue
		}

		rank := m.replacementRank(account, index)
		if !haveBest || rank.betterThan(bestRank) {
			best = account
			bestRank = rank
			haveBest = true
		}
	}

	return best
}

func (m Model) replacementRank(account *config.Account, index int) replacementCandidateRank {
	rank := replacementCandidateRank{
		subscribed:  m.hasSubscription(account),
		blockingErr: m.accountHasBlockingError(account.Key),
		index:       index,
	}

	data, ok := m.UsageData[account.Key]
	if !ok {
		return rank
	}
	if weekly, ok := weeklyQuotaWindow(data); ok {
		rank.hasWeekly = true
		rank.weeklyLeft = weekly.LeftPercent
	}
	if fallback, ok := compactPrimaryWindow(data); ok {
		rank.hasFallback = true
		rank.fallbackLeft = fallback.LeftPercent
	}
	return rank
}

func (left replacementCandidateRank) betterThan(right replacementCandidateRank) bool {
	if left.subscribed != right.subscribed {
		return left.subscribed
	}
	if left.blockingErr != right.blockingErr {
		return !left.blockingErr
	}
	if left.hasWeekly != right.hasWeekly {
		return left.hasWeekly
	}
	if !samePercent(left.weeklyLeft, right.weeklyLeft) {
		return left.weeklyLeft > right.weeklyLeft
	}
	if left.hasFallback != right.hasFallback {
		return left.hasFallback
	}
	if !samePercent(left.fallbackLeft, right.fallbackLeft) {
		return left.fallbackLeft > right.fallbackLeft
	}
	return left.index < right.index
}

func samePercent(left, right float64) bool {
	return math.Abs(left-right) < 1e-9
}

func (m Model) accountHasBlockingError(accountKey string) bool {
	err := m.ErrorsMap[accountKey]
	if err == nil {
		return false
	}
	return !(m.BackgroundErrorMap[accountKey] && hasRenderableQuotaData(m.UsageData[accountKey]))
}

func (m *Model) maybeAutoSwitchAfterRefresh(accountKey string) tea.Cmd {
	if m == nil {
		return nil
	}
	accountKey = strings.TrimSpace(accountKey)
	if accountKey == "" {
		return nil
	}

	manualCheck := accountKey == m.PendingSmartSwitchKey
	shouldCheck := manualCheck || (m.Settings.AutoSwitchExhausted && accountKey == m.activeAccountKey())
	if !shouldCheck {
		return nil
	}
	m.PendingSmartSwitchKey = ""

	if !m.isCompactAccountExhausted(accountKey) {
		return nil
	}

	replacement := m.bestReplacementAccount(accountKey)
	if replacement == nil {
		if !manualCheck {
			return nil
		}
		m.Notice = "active account is exhausted; no replacement account available"
		m.noticeSeq++
		return scheduleNoticeClearCmd(m.noticeSeq)
	}

	if !m.selectActiveAccountByKey(replacement.Key) {
		return nil
	}

	return tea.Batch(
		m.syncAndFetchActiveAccount(),
		ApplyToTargetsCmd(replacement, applyTargetsOrdered()),
	)
}

func (m *Model) selectActiveAccountByKey(accountKey string) bool {
	if m == nil || strings.TrimSpace(accountKey) == "" {
		return false
	}
	for index, account := range m.Accounts {
		if account == nil || account.Key != accountKey {
			continue
		}
		m.ActiveAccountIx = index
		return true
	}
	return false
}
