package ui

import (
	"fmt"
	"strings"

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
	m.WarmupConfirm = false
	m.WarmupRunning = true
	m.WarmupMode = mode
	m.Loading = true
	m.Err = nil
	m.Notice = fmt.Sprintf("warming %s...", warmupModeLabel(mode))
	return m, WarmupCmd(accounts, mode)
}

func (m *Model) resetWarmupState() {
	if m.WarmupRunning {
		return
	}
	m.WarmupConfirm = false
	m.WarmupMode = ""
}

func (m Model) warmupAccounts(mode warmupMode) []*config.Account {
	switch mode {
	case warmupSelected:
		if account := m.activeAccount(); account != nil {
			return []*config.Account{account}
		}
		return nil
	case warmupFree, warmupAll:
		return m.Accounts
	default:
		return nil
	}
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
	m.LoadingMap[result.AccountKey] = false
	delete(m.ErrorsMap, result.AccountKey)
	if result.AccountKey == m.activeAccountKey() {
		m.Data = result.Data
	}
}

func (m Model) renderWarmupConfirmModal() string {
	count := len(m.warmupAccounts(m.WarmupMode))
	label := warmupModeLabel(m.WarmupMode)
	lines := []string{
		WarningStyle.Render("Warm quota"),
		InfoValueStyle.Render(fmt.Sprintf("Send a minimal Codex request for %s (%d accounts)?", label, count)),
		InfoValueStyle.Render("This can consume quota. Already-warmed reset windows are skipped."),
		"",
		ActionMenuHintStyle.Render("[enter] Confirm   [esc] Cancel"),
	}
	return InfoBoxStyle.Copy().Width(78).Render(strings.Join(lines, "\n"))
}
