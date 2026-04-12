package ui

import (
	"fmt"
	"hash/fnv"
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

var autoRefreshLocation = time.FixedZone("GMT+7", 7*60*60)

func (m Model) autoRefreshPeriod(now time.Time) (time.Duration, bool, bool) {
	if !m.Settings.AutoRefreshEnabled {
		return 0, false, false
	}

	localNow := now.In(autoRefreshLocation)
	startMinutes, ok := parseClockMinutesUI(m.Settings.AutoRefreshPeakStart)
	if !ok {
		startMinutes = 8*60 + 30
	}
	endMinutes, ok := parseClockMinutesUI(m.Settings.AutoRefreshPeakEnd)
	if !ok {
		endMinutes = 22*60 + 30
	}
	currentMinutes := localNow.Hour()*60 + localNow.Minute()

	inPeak := false
	switch {
	case startMinutes == endMinutes:
		inPeak = true
	case startMinutes < endMinutes:
		inPeak = currentMinutes >= startMinutes && currentMinutes < endMinutes
	default:
		inPeak = currentMinutes >= startMinutes || currentMinutes < endMinutes
	}

	minutes := m.Settings.AutoRefreshOffPeakMinutes
	if inPeak {
		minutes = m.Settings.AutoRefreshPeakMinutes
	}
	if minutes <= 0 {
		return 0, false, inPeak
	}
	return time.Duration(minutes) * time.Minute, true, inPeak
}

func (m Model) autoRefreshInterval(now time.Time) (time.Duration, bool) {
	interval, ok, _ := m.autoRefreshPeriod(now)
	return interval, ok
}

func (m Model) autoRefreshDueAt(accountKey string, now time.Time) (time.Time, bool) {
	if strings.TrimSpace(accountKey) == "" {
		return time.Time{}, false
	}
	lastFetchAt, ok := m.LastQuotaFetchAt[accountKey]
	if !ok || lastFetchAt.IsZero() {
		return time.Time{}, false
	}
	if _, hasData := m.UsageData[accountKey]; !hasData {
		if _, hasErr := m.ErrorsMap[accountKey]; !hasErr {
			return time.Time{}, false
		}
	}
	interval, ok, _ := m.autoRefreshPeriod(now)
	if !ok {
		return time.Time{}, false
	}
	if fastInterval, ok := m.smartSwitchInterval(accountKey, now); ok {
		interval = fastInterval
	}
	return lastFetchAt.Add(interval).Add(autoRefreshJitter(accountKey, interval)), true
}

func autoRefreshJitter(accountKey string, interval time.Duration) time.Duration {
	if strings.TrimSpace(accountKey) == "" || interval <= 0 {
		return 0
	}
	maxJitter := interval / 10
	if interval <= 5*time.Minute {
		maxJitter = 10 * time.Second
	}
	if maxJitter > 30*time.Second {
		maxJitter = 30 * time.Second
	}
	if maxJitter <= 0 {
		return 0
	}

	h := fnv.New32a()
	_, _ = h.Write([]byte(fmt.Sprintf("%s|%d", accountKey, int(interval/time.Minute))))
	return time.Duration(h.Sum32() % uint32(maxJitter))
}

func (m *Model) nextAutoRefreshCmd(now time.Time) tea.Cmd {
	if m == nil {
		return nil
	}
	if !m.Settings.AutoRefreshEnabled {
		m.clearAutoRefreshPending()
		m.autoRefreshScheduledAt = 0
		return nil
	}

	earliest := time.Time{}
	for _, account := range m.Accounts {
		if account == nil || strings.TrimSpace(account.Key) == "" {
			continue
		}
		if m.LoadingMap[account.Key] || m.BackgroundLoadingMap[account.Key] || m.AutoRefreshPending[account.Key] {
			continue
		}
		dueAt, ok := m.autoRefreshDueAt(account.Key, now)
		if !ok {
			continue
		}
		if earliest.IsZero() || dueAt.Before(earliest) {
			earliest = dueAt
		}
	}
	if earliest.IsZero() {
		m.autoRefreshScheduledAt = 0
		return nil
	}

	delay := earliest.Sub(now)
	if delay < 0 {
		delay = 0
	}
	scheduledAt := earliest.UnixNano()
	m.autoRefreshScheduledAt = scheduledAt
	return tea.Tick(delay, func(now time.Time) tea.Msg {
		return AutoRefreshTickMsg{Now: now, ScheduledAtUnix: scheduledAt}
	})
}

func (m *Model) enqueueDueAutoRefreshes(now time.Time) bool {
	if m == nil || !m.Settings.AutoRefreshEnabled {
		return false
	}
	if m.AutoRefreshPending == nil {
		m.AutoRefreshPending = make(map[string]bool)
	}

	changed := false
	for _, account := range m.Accounts {
		if account == nil || strings.TrimSpace(account.Key) == "" {
			continue
		}
		if m.LoadingMap[account.Key] || m.BackgroundLoadingMap[account.Key] {
			continue
		}
		dueAt, ok := m.autoRefreshDueAt(account.Key, now)
		if !ok || dueAt.After(now) {
			continue
		}
		if !m.AutoRefreshPending[account.Key] {
			m.AutoRefreshPending[account.Key] = true
			changed = true
		}
	}
	return changed
}

func (m *Model) recordQuotaFetch(accountKey string, fetchedAt time.Time) {
	if m == nil || strings.TrimSpace(accountKey) == "" {
		return
	}
	if fetchedAt.IsZero() {
		fetchedAt = time.Now()
	}
	if m.LastQuotaFetchAt == nil {
		m.LastQuotaFetchAt = make(map[string]time.Time)
	}
	m.LastQuotaFetchAt[accountKey] = fetchedAt
	delete(m.AutoRefreshPending, accountKey)
}

func (m *Model) clearAutoRefreshPending() {
	if m == nil || len(m.AutoRefreshPending) == 0 {
		return
	}
	for key := range m.AutoRefreshPending {
		delete(m.AutoRefreshPending, key)
	}
}

func (m *Model) pruneAutoRefreshState() {
	if m == nil {
		return
	}
	valid := make(map[string]struct{}, len(m.Accounts))
	for _, account := range m.Accounts {
		if account == nil || strings.TrimSpace(account.Key) == "" {
			continue
		}
		valid[account.Key] = struct{}{}
	}
	for key := range m.LastQuotaFetchAt {
		if _, ok := valid[key]; !ok {
			delete(m.LastQuotaFetchAt, key)
		}
	}
	for key := range m.AutoRefreshPending {
		if _, ok := valid[key]; !ok {
			delete(m.AutoRefreshPending, key)
		}
	}
	for key := range m.BackgroundLoadingMap {
		if _, ok := valid[key]; !ok {
			delete(m.BackgroundLoadingMap, key)
		}
	}
	for key := range m.BackgroundErrorMap {
		if _, ok := valid[key]; !ok {
			delete(m.BackgroundErrorMap, key)
		}
	}
	for key := range m.SmartSwitchBurstPending {
		if _, ok := valid[key]; !ok {
			delete(m.SmartSwitchBurstPending, key)
		}
	}
}

func parseClockMinutesUI(value string) (int, bool) {
	parts := strings.Split(strings.TrimSpace(value), ":")
	if len(parts) != 2 {
		return 0, false
	}
	hour, err := strconv.Atoi(parts[0])
	if err != nil || hour < 0 || hour > 23 {
		return 0, false
	}
	minute, err := strconv.Atoi(parts[1])
	if err != nil || minute < 0 || minute > 59 {
		return 0, false
	}
	return hour*60 + minute, true
}
