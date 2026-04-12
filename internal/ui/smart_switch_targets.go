package ui

import (
	"sort"
	"strings"
	"time"

	"github.com/deLiseLINO/codex-quota/internal/config"
)

func (m Model) currentAppliedAccountKeys() []string {
	assigned := map[config.Source]string{}
	for _, account := range m.Accounts {
		if account == nil || strings.TrimSpace(account.Key) == "" {
			continue
		}
		for source := range m.activeTargetSourcesForAccount(account) {
			if source != config.SourceCodex && source != config.SourceOpenCode {
				continue
			}
			if assigned[source] == "" {
				assigned[source] = account.Key
			}
		}
	}

	ordered := []config.Source{config.SourceCodex, config.SourceOpenCode}
	seen := make(map[string]bool, len(ordered))
	keys := make([]string, 0, len(ordered))
	for _, source := range ordered {
		key := strings.TrimSpace(assigned[source])
		if key == "" || seen[key] {
			continue
		}
		seen[key] = true
		keys = append(keys, key)
	}
	return keys
}

func (m Model) currentAppliedAccountKeySet() map[string]bool {
	keys := m.currentAppliedAccountKeys()
	if len(keys) == 0 {
		return nil
	}
	set := make(map[string]bool, len(keys))
	for _, key := range keys {
		set[key] = true
	}
	return set
}

func (m Model) isCurrentAppliedAccountKey(accountKey string) bool {
	accountKey = strings.TrimSpace(accountKey)
	if accountKey == "" {
		return false
	}
	return m.currentAppliedAccountKeySet()[accountKey]
}

func (m *Model) resetSmartSwitchState() {
	if m == nil {
		return
	}
	m.PendingSmartSwitchManual = false
	for key := range m.PendingSmartSwitchKeys {
		delete(m.PendingSmartSwitchKeys, key)
	}
	for key := range m.SmartSwitchBurstPending {
		delete(m.SmartSwitchBurstPending, key)
	}
}

func (m *Model) setPendingSmartSwitchKeys(keys []string, manual bool) {
	if m == nil {
		return
	}
	if m.PendingSmartSwitchKeys == nil {
		m.PendingSmartSwitchKeys = make(map[string]bool)
	}
	for key := range m.PendingSmartSwitchKeys {
		delete(m.PendingSmartSwitchKeys, key)
	}
	for _, key := range keys {
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		m.PendingSmartSwitchKeys[key] = true
	}
	m.PendingSmartSwitchManual = manual && len(m.PendingSmartSwitchKeys) > 0
	if !m.PendingSmartSwitchManual {
		for key := range m.PendingSmartSwitchKeys {
			delete(m.PendingSmartSwitchKeys, key)
		}
	}
}

func (m Model) isPendingSmartSwitchKey(accountKey string) bool {
	accountKey = strings.TrimSpace(accountKey)
	if accountKey == "" || len(m.PendingSmartSwitchKeys) == 0 {
		return false
	}
	return m.PendingSmartSwitchKeys[accountKey]
}

func (m *Model) finishPendingSmartSwitchKey(accountKey string) {
	if m == nil {
		return
	}
	accountKey = strings.TrimSpace(accountKey)
	if accountKey == "" || len(m.PendingSmartSwitchKeys) == 0 {
		return
	}
	delete(m.PendingSmartSwitchKeys, accountKey)
	if len(m.PendingSmartSwitchKeys) == 0 {
		m.PendingSmartSwitchManual = false
	}
}

func (m Model) smartSwitchBurstOrder() []string {
	if len(m.SmartSwitchBurstPending) == 0 {
		return nil
	}
	ordered := make([]string, 0, len(m.SmartSwitchBurstPending))
	seen := make(map[string]bool, len(m.SmartSwitchBurstPending))

	watchKeys := m.currentAppliedAccountKeys()
	if m.PendingSmartSwitchManual && len(m.PendingSmartSwitchKeys) > 0 {
		watchKeys = make([]string, 0, len(m.PendingSmartSwitchKeys))
		watchSet := make(map[string]bool, len(m.PendingSmartSwitchKeys))
		for _, key := range m.currentAppliedAccountKeys() {
			if m.PendingSmartSwitchKeys[key] {
				watchKeys = append(watchKeys, key)
				watchSet[key] = true
			}
		}
		remaining := make([]string, 0, len(m.PendingSmartSwitchKeys))
		for key := range m.PendingSmartSwitchKeys {
			if !watchSet[key] {
				remaining = append(remaining, key)
			}
		}
		sort.Strings(remaining)
		watchKeys = append(watchKeys, remaining...)
	}

	for _, key := range watchKeys {
		if m.SmartSwitchBurstPending[key] && !seen[key] {
			ordered = append(ordered, key)
			seen[key] = true
		}
	}

	excluded := m.currentAppliedAccountKeySet()
	if excluded == nil {
		excluded = make(map[string]bool)
	}
	for _, key := range ordered {
		excluded[key] = true
	}
	for _, account := range m.bestReplacementAccounts(excluded, len(m.Accounts)) {
		if account == nil || !m.SmartSwitchBurstPending[account.Key] || seen[account.Key] {
			continue
		}
		ordered = append(ordered, account.Key)
		seen[account.Key] = true
	}

	remaining := make([]string, 0, len(m.SmartSwitchBurstPending))
	for key := range m.SmartSwitchBurstPending {
		if !seen[key] {
			remaining = append(remaining, key)
		}
	}
	sort.Strings(remaining)
	return append(ordered, remaining...)
}

func (m *Model) queueSmartSwitchBurst(watchKeys []string, skipKeys map[string]bool, limit int) {
	if m == nil || limit <= 0 {
		return
	}
	if m.SmartSwitchBurstPending == nil {
		m.SmartSwitchBurstPending = make(map[string]bool)
	}
	if m.AutoRefreshPending == nil {
		m.AutoRefreshPending = make(map[string]bool)
	}

	ordered := make([]string, 0, limit)
	excluded := make(map[string]bool, len(watchKeys)+len(skipKeys))
	for key := range skipKeys {
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		excluded[key] = true
	}
	for _, key := range watchKeys {
		key = strings.TrimSpace(key)
		if key == "" || excluded[key] {
			continue
		}
		ordered = append(ordered, key)
		excluded[key] = true
		if len(ordered) == limit {
			break
		}
	}
	if len(ordered) < limit {
		for _, account := range m.bestReplacementAccounts(excluded, len(m.Accounts)) {
			if account == nil || strings.TrimSpace(account.Key) == "" || excluded[account.Key] {
				continue
			}
			ordered = append(ordered, account.Key)
			excluded[account.Key] = true
			if len(ordered) == limit {
				break
			}
		}
	}

	for _, key := range ordered {
		if m.LoadingMap[key] || m.BackgroundLoadingMap[key] {
			continue
		}
		m.SmartSwitchBurstPending[key] = true
		delete(m.AutoRefreshPending, key)
	}
}

func (m *Model) maybeQueueAutoSmartSwitchBurst(accountKey string, now time.Time) {
	if m == nil || !m.Settings.AutoSwitchExhausted {
		return
	}
	accountKey = strings.TrimSpace(accountKey)
	if accountKey == "" || !m.isCurrentAppliedAccountKey(accountKey) {
		return
	}
	if _, ok := m.smartSwitchInterval(accountKey, now); !ok {
		return
	}
	skip := map[string]bool{accountKey: true}
	m.queueSmartSwitchBurst(m.currentAppliedAccountKeys(), skip, 3)
}

func (m Model) priorityFetchAccounts() []*config.Account {
	orderedKeys := m.smartSwitchBurstOrder()
	if len(orderedKeys) == 0 {
		return nil
	}
	accounts := make([]*config.Account, 0, len(orderedKeys))
	for _, key := range orderedKeys {
		if account := m.findAccountByKey(key); account != nil {
			accounts = append(accounts, account)
		}
	}
	return accounts
}
