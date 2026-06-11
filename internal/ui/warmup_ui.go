package ui

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/deLiseLINO/codex-quota/internal/api"
	"github.com/deLiseLINO/codex-quota/internal/config"
)

func (m Model) beginWarmup(mode warmupMode) (tea.Model, tea.Cmd) {
	accounts := m.warmupAccounts(mode)
	if len(accounts) == 0 {
		m.resetActionMenuState()
		m.Notice = "no accounts available for warmup"
		m.noticeSeq++
		return m, scheduleNoticeClearCmd(m.noticeSeq)
	}

	m.resetHelpState()
	m.resetActionMenuState()
	m.resetSettingsState()
	m.closeCompactDetail()
	m.closeCompactSearch()
	m.resetDeleteState()
	m.resetApplyState()
	m.WarmupSelect = false
	m.ShowInfo = false
	m.Err = nil
	m.Notice = ""
	m.WarmupMode = mode

	if mode == warmupSelected {
		return m.startWarmup(accounts, mode)
	}

	m.WarmupConfirm = true
	return m, nil
}

func (m Model) beginWarmupSelect() (tea.Model, tea.Cmd) {
	if len(m.Accounts) == 0 {
		m.Notice = "no accounts available for warmup"
		m.noticeSeq++
		return m, scheduleNoticeClearCmd(m.noticeSeq)
	}

	m.resetHelpState()
	m.resetActionMenuState()
	m.resetSettingsState()
	m.closeCompactDetail()
	m.closeCompactSearch()
	m.resetDeleteState()
	m.resetApplyState()
	m.ShowInfo = false
	m.Err = nil
	m.Notice = ""
	m.WarmupConfirm = false
	m.WarmupMode = ""
	m.WarmupSelect = true
	return m, nil
}

func (m Model) handleWarmupSelect(keyStr string) (tea.Model, tea.Cmd) {
	switch keyStr {
	case "q", "ctrl+c":
		return m, tea.Quit
	case "esc":
		m.resetWarmupState()
		return m, nil
	case "s":
		return m.beginWarmup(warmupSelected)
	case "f":
		return m.beginWarmup(warmupFree)
	case "a":
		return m.beginWarmup(warmupAll)
	}

	return m, nil
}

func (m Model) handleWarmupConfirm(keyStr string) (tea.Model, tea.Cmd) {
	switch keyStr {
	case "q", "ctrl+c":
		return m, tea.Quit
	case "esc":
		m.resetWarmupState()
		return m, nil
	case "enter":
		accounts := m.warmupAccounts(m.WarmupMode)
		if len(accounts) == 0 {
			m.resetWarmupState()
			m.Notice = "no accounts available for warmup"
			m.noticeSeq++
			return m, scheduleNoticeClearCmd(m.noticeSeq)
		}
		return m.startWarmup(accounts, m.WarmupMode)
	}

	return m, nil
}

func (m Model) startWarmup(accounts []*config.Account, mode warmupMode) (tea.Model, tea.Cmd) {
	accountSnapshots := cloneAccounts(accounts)
	m.WarmupConfirm = false
	m.WarmupSelect = false
	m.WarmupRunning = true
	m.WarmupMode = mode
	m.WarmupAccounts = accountSnapshots
	m.WarmupResults = nil
	m.WarmupTotal = len(accountSnapshots)
	m.WarmupCompleted = 0
	m.WarmupWarmed = 0
	m.WarmupSkipped = 0
	m.WarmupFailed = 0
	m.WarmupStartedAt = time.Now()
	m.WarmupSaveErr = nil
	m.warmupState = config.WarmupState{}
	m.warmupStateChanged = false
	m.WarmupCurrentLabel = "loading warmup state"
	if len(accountSnapshots) > 0 {
		m.WarmupCurrentLabel = warmupAccountLabel(accountSnapshots[0])
	}
	m.Loading = true
	m.Err = nil
	m.Notice = ""
	return m, LoadWarmupStateCmd()
}

func (m *Model) resetWarmupState() {
	if m.WarmupRunning {
		return
	}
	m.WarmupConfirm = false
	m.WarmupSelect = false
	m.WarmupMode = ""
	m.resetWarmupProgressState()
}

func (m Model) warmupAccounts(mode warmupMode) []*config.Account {
	switch mode {
	case warmupSelected:
		if account := m.activeAccount(); account != nil {
			return []*config.Account{account}
		}
		return nil
	case warmupFree:
		return m.knownFreeWarmupAccounts()
	case warmupAll:
		return m.Accounts
	default:
		return nil
	}
}

func (m Model) nextWarmupStepCmd(delay bool) tea.Cmd {
	if !m.WarmupRunning || m.WarmupCompleted >= len(m.WarmupAccounts) {
		return nil
	}
	cmd := WarmupStepCmd(m.WarmupAccounts[m.WarmupCompleted], m.WarmupMode, m.warmupState)
	if delay && warmupRequestDelay > 0 {
		return tea.Tick(warmupRequestDelay, func(_ time.Time) tea.Msg {
			return cmd()
		})
	}
	return cmd
}

func (m *Model) applyWarmupStep(msg WarmupStepMsg) {
	m.warmupState = msg.State
	m.warmupStateChanged = m.warmupStateChanged || msg.StateChanged
	m.WarmupResults = append(m.WarmupResults, msg.Result)
	m.WarmupCompleted++
	switch {
	case msg.Result.Warmed:
		m.WarmupWarmed++
	case msg.Result.Err != nil:
		m.WarmupFailed++
	case msg.Result.Skipped:
		m.WarmupSkipped++
	}
	m.applyWarmupResult(msg.Result)
	m.WarmupCurrentLabel = ""
	if m.WarmupCompleted < len(m.WarmupAccounts) {
		m.WarmupCurrentLabel = warmupAccountLabel(m.WarmupAccounts[m.WarmupCompleted])
	}
}

func (m *Model) resetWarmupProgressState() {
	m.WarmupAccounts = nil
	m.WarmupResults = nil
	m.WarmupTotal = 0
	m.WarmupCompleted = 0
	m.WarmupWarmed = 0
	m.WarmupSkipped = 0
	m.WarmupFailed = 0
	m.WarmupCurrentLabel = ""
	m.WarmupStartedAt = time.Time{}
	m.WarmupSaveErr = nil
	m.warmupState = config.WarmupState{}
	m.warmupStateChanged = false
}

func (m *Model) applyWarmupResult(result WarmupAccountResult) {
	m.applyAccountSnapshot(result.AccountKey, result.Account)
	if !result.HasData || result.AccountKey == "" {
		return
	}
	if m.UsageData == nil {
		m.UsageData = make(map[string]api.UsageData)
	}
	if m.PlanTypeByAccount == nil {
		m.PlanTypeByAccount = make(map[string]string)
	}
	if m.ErrorsMap == nil {
		m.ErrorsMap = make(map[string]error)
	}
	if m.LoadingMap == nil {
		m.LoadingMap = make(map[string]bool)
	}
	m.UsageData[result.AccountKey] = result.Data
	m.setKnownPlanType(result.AccountKey, result.Data.PlanType)
	m.recordUsageDataFetch(result.AccountKey, time.Now())
	m.LoadingMap[result.AccountKey] = false
	delete(m.ErrorsMap, result.AccountKey)
	if result.AccountKey == m.activeAccountKey() {
		m.Data = result.Data
	}
}

func (m Model) renderWarmupConfirmModal() string {
	label, count := m.warmupConfirmScopeText()
	lines := []string{
		WarningStyle.Render("Warm quota"),
		InfoValueStyle.Render(fmt.Sprintf("Send a minimal Codex request for %s (%d accounts)?", label, count)),
		InfoValueStyle.Render("This can consume quota. Already-warmed reset windows are skipped."),
		"",
		ActionMenuHintStyle.Render("[enter] Confirm   [esc] Cancel"),
	}
	return InfoBoxStyle.Copy().Width(78).Render(strings.Join(lines, "\n"))
}

func (m Model) warmupConfirmScopeText() (string, int) {
	if m.WarmupMode == warmupFree {
		return "all known free accounts", m.knownFreeWarmupCount()
	}
	return warmupModeLabel(m.WarmupMode), len(m.warmupAccounts(m.WarmupMode))
}

func (m Model) warmupProgressScopeText() string {
	if m.WarmupMode == warmupFree {
		return "all known free accounts"
	}
	return warmupModeLabel(m.WarmupMode)
}

func (m Model) knownFreeWarmupAccounts() []*config.Account {
	accounts := make([]*config.Account, 0)
	for _, account := range m.Accounts {
		if account == nil || account.Key == "" {
			continue
		}
		if isFreePlan(m.PlanTypeByAccount[account.Key]) {
			accounts = append(accounts, account)
		}
	}
	return accounts
}

func (m Model) knownFreeWarmupCount() int {
	return len(m.knownFreeWarmupAccounts())
}

func (m Model) renderWarmupSelectModal() string {
	lines := []string{
		WarningStyle.Render("Warm quota"),
		InfoValueStyle.Render("Choose warmup scope:"),
		"",
		InfoValueStyle.Render("s  Selected account"),
		InfoValueStyle.Render("f  All free accounts"),
		InfoValueStyle.Render("a  All accounts"),
		"",
		ActionMenuHintStyle.Render("[s/f/a] Select   [esc] Cancel"),
	}
	return InfoBoxStyle.Copy().Width(52).Render(strings.Join(lines, "\n"))
}

func (m Model) renderWarmupProgressModal() string {
	lines := []string{
		WarningStyle.Render("Warm quota"),
		InfoValueStyle.Render(fmt.Sprintf("Scope: %s", m.warmupProgressScopeText())),
		InfoValueStyle.Render(fmt.Sprintf("Progress: %d / %d (%d%%)", m.WarmupCompleted, m.WarmupTotal, m.warmupProgressPercent())),
	}
	if current := strings.TrimSpace(m.WarmupCurrentLabel); current != "" {
		lines = append(lines, InfoValueStyle.Render("Current: "+truncateLabel(current, 48)))
	}
	lines = append(lines, "")
	lines = append(lines, InfoValueStyle.Render(fmt.Sprintf("Warmed %d • Skipped %d • Failed %d", m.WarmupWarmed, m.WarmupSkipped, m.WarmupFailed)))
	lines = append(lines, InfoValueStyle.Render("Elapsed "+formatWarmupElapsed(m.WarmupStartedAt)))
	if latest := warmupProgressResultText(m.WarmupResults); latest != "" {
		lines = append(lines, "")
		lines = append(lines, ActionMenuHintStyle.Render(latest))
	}
	return InfoBoxStyle.Copy().Width(72).Render(strings.Join(lines, "\n"))
}

func (m Model) warmupProgressPercent() int {
	if m.WarmupTotal <= 0 {
		return 0
	}
	return (m.WarmupCompleted * 100) / m.WarmupTotal
}

func warmupProgressResultText(results []WarmupAccountResult) string {
	if len(results) == 0 {
		return ""
	}
	result := results[len(results)-1]
	label := truncateLabel(warmupAccountLabel(result.Account), 36)
	switch {
	case result.Warmed:
		return "Last: warmed " + label
	case result.Err != nil:
		return "Last: failed " + label
	case result.Skipped:
		reason := strings.TrimSpace(result.SkipReason)
		if reason == "" {
			reason = "skipped"
		}
		return fmt.Sprintf("Last: skipped %s (%s)", label, reason)
	default:
		return "Last: checked " + label
	}
}

func formatWarmupElapsed(startedAt time.Time) string {
	if startedAt.IsZero() {
		return "00:00"
	}
	elapsed := time.Since(startedAt).Truncate(time.Second)
	if elapsed < 0 {
		elapsed = 0
	}
	hours := int(elapsed / time.Hour)
	minutes := int((elapsed % time.Hour) / time.Minute)
	seconds := int((elapsed % time.Minute) / time.Second)
	if hours > 0 {
		return fmt.Sprintf("%d:%02d:%02d", hours, minutes, seconds)
	}
	return fmt.Sprintf("%02d:%02d", minutes, seconds)
}
