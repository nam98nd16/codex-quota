package ui

import (
	"math"
	"sort"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/deLiseLINO/codex-quota/internal/api"
	"github.com/deLiseLINO/codex-quota/internal/config"
)

const (
	smartSwitchWarningThresholdPercent = 90.0
	smartSwitchMinimumRefreshInterval  = 5 * time.Second
	autoSwitchFallbackThresholdPercent = 3.0
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

type rankedReplacementCandidate struct {
	account *config.Account
	rank    replacementCandidateRank
}

func (m Model) beginSmartSwitchActive() (tea.Model, tea.Cmd) {
	watchKeys := m.currentAppliedAccountKeys()
	if len(watchKeys) == 0 {
		m.Notice = "no applied account found for smart switch"
		m.noticeSeq++
		return m, scheduleNoticeClearCmd(m.noticeSeq)
	}
	m.Loading = false
	m.Err = nil
	m.resetHelpState()
	m.resetActionMenuState()
	m.resetSettingsState()
	m.closeCompactDetail()
	m.closeCompactSearch()
	m.resetDeleteState()
	m.resetApplyState()
	m.Notice = ""
	m.resetSmartSwitchState()
	m.setPendingSmartSwitchKeys(watchKeys, true)
	m.queueSmartSwitchBurst(watchKeys, nil, 3)
	return m, m.fetchNextCmd()
}

func (m Model) smartSwitchInterval(accountKey string, now time.Time) (time.Duration, bool) {
	if strings.TrimSpace(accountKey) == "" || !m.isCurrentAppliedAccountKey(accountKey) {
		return 0, false
	}
	if m.LoadingMap[accountKey] || m.BackgroundLoadingMap[accountKey] || m.accountHasBlockingError(accountKey) {
		return 0, false
	}
	baseInterval, ok, inPeak := m.autoRefreshPeriod(now)
	if !ok || !inPeak {
		return 0, false
	}

	data, ok := m.UsageData[accountKey]
	if !ok {
		return 0, false
	}
	window, ok := watchedAutoSwitchWindow(data)
	if !ok || window.LeftPercent > smartSwitchWarningThresholdPercent {
		return 0, false
	}
	return smartSwitchRefreshInterval(baseInterval, window.LeftPercent), true
}

func smartSwitchRefreshInterval(baseInterval time.Duration, leftPercent float64) time.Duration {
	if baseInterval <= 0 {
		return 0
	}
	refreshPercent := leftPercent
	if leftPercent > 10 {
		refreshPercent = math.Ceil(leftPercent/10) * 10
	}
	interval := time.Duration(float64(baseInterval) * refreshPercent / 100)
	if interval < smartSwitchMinimumRefreshInterval {
		return smartSwitchMinimumRefreshInterval
	}
	return interval
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
	excluded := map[string]bool{}
	if strings.TrimSpace(excludeKey) != "" {
		excluded[excludeKey] = true
	}
	best := m.bestReplacementAccounts(excluded, 1)
	if len(best) == 0 {
		return nil
	}
	return best[0]
}

func (m Model) bestReplacementAccounts(excluded map[string]bool, limit int) []*config.Account {
	if limit <= 0 {
		return nil
	}
	ranked := make([]rankedReplacementCandidate, 0, len(m.Accounts))
	for index, account := range m.Accounts {
		if account == nil || strings.TrimSpace(account.Key) == "" {
			continue
		}
		if excluded[account.Key] || m.isCompactAccountExhausted(account.Key) {
			continue
		}
		ranked = append(ranked, rankedReplacementCandidate{
			account: account,
			rank:    m.replacementRank(account, index),
		})
	}
	sort.SliceStable(ranked, func(i, j int) bool {
		return ranked[i].rank.betterThan(ranked[j].rank)
	})
	if len(ranked) > limit {
		ranked = ranked[:limit]
	}
	accounts := make([]*config.Account, 0, len(ranked))
	for _, candidate := range ranked {
		accounts = append(accounts, candidate.account)
	}
	return accounts
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

	manualCheck := m.PendingSmartSwitchManual && m.isPendingSmartSwitchKey(accountKey)
	autoCheck := m.isCurrentAppliedAccountKey(accountKey) &&
		(m.autoSwitchFallbackEnabled() || m.autoSwitchConfirmedExhaustedFallbackEnabled())
	shouldCheck := manualCheck || autoCheck
	if !shouldCheck {
		return nil
	}
	if manualCheck {
		m.finishPendingSmartSwitchKey(accountKey)
	}

	if !m.shouldSwitchAfterRefresh(accountKey, manualCheck) {
		if manualCheck && !m.PendingSmartSwitchManual {
			for key := range m.SmartSwitchBurstPending {
				delete(m.SmartSwitchBurstPending, key)
			}
		}
		if !manualCheck {
			m.maybeQueueAutoSmartSwitchBurst(accountKey, time.Now())
		}
		return nil
	}

	replacement := m.autoSwitchReplacement(accountKey)
	if replacement == nil {
		if !manualCheck {
			return nil
		}
		m.resetSmartSwitchState()
		m.Notice = "applied account is exhausted; no replacement account available"
		m.noticeSeq++
		return scheduleNoticeClearCmd(m.noticeSeq)
	}
	return m.applyAutoSwitchReplacement(replacement)
}

func (m Model) autoSwitchEventsEnabled() bool {
	if !m.Settings.AutoSwitchExhausted {
		return false
	}
	return config.NormalizeSettings(m.Settings).AutoSwitchTrigger != config.AutoSwitchTriggerLegacyOnly
}

func (m Model) autoSwitchFallbackEnabled() bool {
	if !m.Settings.AutoSwitchExhausted {
		return false
	}
	trigger := config.NormalizeSettings(m.Settings).AutoSwitchTrigger
	if trigger == config.AutoSwitchTriggerEventOnly {
		return false
	}
	if trigger == config.AutoSwitchTriggerLegacyOnly {
		return true
	}
	return !m.OpenCodePluginInstalled
}

func (m Model) autoSwitchConfirmedExhaustedFallbackEnabled() bool {
	settings := config.NormalizeSettings(m.Settings)
	if !settings.AutoSwitchExhausted || !settings.AutoSwitchConfirmedExhaustedFallback {
		return false
	}
	return m.OpenCodePluginInstalled && settings.AutoSwitchTrigger == config.AutoSwitchTriggerEventFallback
}

func (m Model) shouldSwitchAfterRefresh(accountKey string, manualCheck bool) bool {
	if manualCheck {
		return m.isCompactAccountExhausted(accountKey) || m.accountAtFallbackSwitchThreshold(accountKey)
	}
	if !m.autoSwitchFallbackEnabled() {
		return m.autoSwitchConfirmedExhaustedFallbackEnabled() && m.accountConfirmedExhausted(accountKey)
	}
	return m.accountAtFallbackSwitchThreshold(accountKey)
}

func (m Model) accountConfirmedExhausted(accountKey string) bool {
	data, ok := m.UsageData[accountKey]
	return ok && isConfirmedExhausted(data)
}

func (m Model) accountAtFallbackSwitchThreshold(accountKey string) bool {
	data, ok := m.UsageData[accountKey]
	if !ok {
		return false
	}
	if isConfirmedExhausted(data) {
		return true
	}
	window, ok := watchedAutoSwitchWindow(data)
	return ok && window.LeftPercent <= autoSwitchFallbackThresholdPercent
}

func (m Model) autoSwitchReplacement(excludeKey string) *config.Account {
	excluded := m.currentAppliedAccountKeySet()
	if excluded == nil {
		excluded = map[string]bool{}
	}
	if strings.TrimSpace(excludeKey) != "" {
		excluded[excludeKey] = true
	}
	replacements := m.bestReplacementAccounts(excluded, 1)
	if len(replacements) == 0 {
		return nil
	}
	return replacements[0]
}

func (m *Model) applyAutoSwitchReplacement(replacement *config.Account) tea.Cmd {
	if replacement == nil {
		return nil
	}
	m.resetSmartSwitchState()
	if !m.selectActiveAccountByKey(replacement.Key) {
		return nil
	}
	return tea.Batch(
		m.syncAndFetchActiveAccount(),
		ApplyToTargetsCmd(replacement, applyTargetsOrdered()),
	)
}

func (m *Model) forceAutoSwitchAppliedOpenCodeAccount() tea.Cmd {
	if m == nil || !m.autoSwitchEventsEnabled() {
		return nil
	}
	accountKey := m.currentAppliedAccountKeyForSource(config.SourceOpenCode)
	if strings.TrimSpace(accountKey) == "" {
		return nil
	}
	replacement := m.autoSwitchReplacement(accountKey)
	if replacement == nil {
		return nil
	}
	m.Notice = "OpenCode reported quota exhausted; switching account"
	m.noticeSeq++
	return tea.Batch(m.applyAutoSwitchReplacement(replacement), scheduleNoticeClearCmd(m.noticeSeq))
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
		m.ensureCompactActiveVisible()
		return true
	}
	return false
}
