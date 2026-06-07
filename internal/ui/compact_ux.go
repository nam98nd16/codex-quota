package ui

import (
	"fmt"
	"sort"
	"strings"
	"unicode/utf8"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"

	"github.com/deLiseLINO/codex-quota/internal/config"
)

type compactFilterMode int

const (
	compactFilterAll compactFilterMode = iota
	compactFilterAvailable
	compactFilterExhausted
	compactFilterErrors
)

type compactSortMode int

const (
	compactSortSubscriptions compactSortMode = iota
	compactSortOriginal
	compactSortQuota
	compactSortReset
	compactSortSource
	compactSortStatus
)

const compactSortModeCount = 6

type compactIndexSection struct {
	header  string
	indices []int
}

func (m Model) compactIndexSections() []compactIndexSection {
	matched := make([]int, 0, len(m.Accounts))
	for i, account := range m.Accounts {
		if account == nil || !m.compactAccountMatchesControls(account) {
			continue
		}
		matched = append(matched, i)
	}

	pinned := []int{}
	normal := []int{}
	exhausted := []int{}
	for _, index := range matched {
		account := m.Accounts[index]
		if m.isCompactAccountExhausted(account.Key) {
			exhausted = append(exhausted, index)
			continue
		}
		if m.CompactPinApplied && m.activeSourceBadgesForAccount(account) != "" {
			pinned = append(pinned, index)
			continue
		}
		normal = append(normal, index)
	}

	m.sortCompactIndices(pinned)
	m.sortCompactIndices(normal)
	m.sortCompactIndices(exhausted)

	sections := []compactIndexSection{}
	if len(pinned) > 0 {
		sections = append(sections, compactIndexSection{header: "Applied accounts", indices: pinned})
	}
	if len(normal) > 0 {
		sections = append(sections, compactIndexSection{indices: normal})
	}
	if len(exhausted) > 0 {
		header := "Exhausted accounts"
		indices := exhausted
		if m.CompactExhaustedCollapsed {
			header = fmt.Sprintf("Exhausted accounts (%d hidden)", len(exhausted))
			indices = nil
		}
		sections = append(sections, compactIndexSection{header: header, indices: indices})
	}
	return sections
}

func (m Model) compactAccountMatchesControls(account *config.Account) bool {
	if account == nil {
		return false
	}
	if !m.compactAccountMatchesFilter(account) {
		return false
	}
	query := strings.ToLower(strings.TrimSpace(m.CompactSearchQuery))
	if query == "" {
		return true
	}
	return strings.Contains(m.compactAccountSearchText(account), query)
}

func (m Model) compactAccountMatchesFilter(account *config.Account) bool {
	switch m.CompactFilter {
	case compactFilterAvailable:
		return !m.isCompactAccountExhausted(account.Key) && !m.compactAccountHasForegroundError(account)
	case compactFilterExhausted:
		return m.isCompactAccountExhausted(account.Key)
	case compactFilterErrors:
		return m.compactAccountHasForegroundError(account)
	default:
		return true
	}
}

func (m Model) compactAccountSearchText(account *config.Account) string {
	parts := []string{
		m.displayAccountLabel(account),
		account.Label,
		account.Email,
		account.AccountID,
		account.SourceLabel(),
		strings.Join(m.SourcesByAccountID[account.AccountID], " "),
		m.PlanTypeByAccount[account.Key],
	}
	if badges := m.activeSourceBadgesForAccount(account); badges != "" {
		parts = append(parts, badges, sourceBadgeLegendText(badges))
	}
	return strings.ToLower(strings.Join(parts, " "))
}

func (m Model) compactAccountHasForegroundError(account *config.Account) bool {
	if account == nil {
		return false
	}
	err := m.ErrorsMap[account.Key]
	return err != nil && !(m.BackgroundErrorMap[account.Key] && hasRenderableQuotaData(m.UsageData[account.Key]))
}

func (m Model) sortCompactIndices(indices []int) {
	if m.CompactSort == compactSortOriginal || len(indices) < 2 {
		return
	}
	sort.SliceStable(indices, func(left, right int) bool {
		leftAccount := m.Accounts[indices[left]]
		rightAccount := m.Accounts[indices[right]]
		switch m.CompactSort {
		case compactSortSubscriptions:
			return m.compactSubscriptionSortLess(leftAccount, rightAccount)
		case compactSortQuota:
			return m.compactQuotaSortKey(leftAccount) < m.compactQuotaSortKey(rightAccount)
		case compactSortReset:
			return m.compactResetSortKey(leftAccount) < m.compactResetSortKey(rightAccount)
		case compactSortSource:
			return m.compactSourceSortKey(leftAccount) < m.compactSourceSortKey(rightAccount)
		case compactSortStatus:
			return m.compactStatusSortKey(leftAccount) < m.compactStatusSortKey(rightAccount)
		default:
			return false
		}
	})
}

func (m Model) compactSubscriptionSortLess(left, right *config.Account) bool {
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

	leftReset := m.compactResetSortKey(left)
	rightReset := m.compactResetSortKey(right)
	if leftReset != rightReset {
		return leftReset < rightReset
	}

	return m.compactNameSortKey(left) < m.compactNameSortKey(right)
}

func (m Model) compactQuotaSortKey(account *config.Account) float64 {
	if account == nil {
		return 101
	}
	if window, ok := compactPrimaryWindow(m.UsageData[account.Key]); ok {
		return window.LeftPercent
	}
	if m.compactAccountHasForegroundError(account) {
		return 102
	}
	return 101
}

func (m Model) compactResetSortKey(account *config.Account) int64 {
	if account == nil {
		return 1<<62 - 1
	}
	if window, ok := compactPrimaryWindow(m.UsageData[account.Key]); ok && !window.ResetAt.IsZero() {
		return window.ResetAt.Unix()
	}
	return 1<<62 - 1
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

func (m Model) compactFilterLabel() string {
	switch m.CompactFilter {
	case compactFilterAvailable:
		return "available"
	case compactFilterExhausted:
		return "exhausted"
	case compactFilterErrors:
		return "errors"
	default:
		return "all"
	}
}

func (m Model) compactSortLabel() string {
	switch m.CompactSort {
	case compactSortSubscriptions:
		return "subscriptions"
	case compactSortOriginal:
		return "original"
	case compactSortQuota:
		return "quota"
	case compactSortReset:
		return "reset"
	case compactSortSource:
		return "source"
	case compactSortStatus:
		return "status"
	default:
		return "subscriptions"
	}
}

func (m *Model) cycleCompactFilter() {
	m.CompactFilter = compactFilterMode((int(m.CompactFilter) + 1) % 4)
	m.normalizeCompactControls()
}

func (m *Model) cycleCompactSort() {
	m.CompactSort = compactSortMode((int(m.CompactSort) + 1) % compactSortModeCount)
	m.normalizeCompactControls()
}

func (m *Model) toggleCompactPinApplied() {
	m.CompactPinApplied = !m.CompactPinApplied
	m.normalizeCompactControls()
}

func (m *Model) toggleCompactExhaustedCollapsed() {
	m.CompactExhaustedCollapsed = !m.CompactExhaustedCollapsed
	m.normalizeCompactControls()
}

func (m *Model) toggleCompactStatusDensity() {
	m.CompactStatusMinimal = !m.CompactStatusMinimal
}

func (m *Model) openCompactSearch() {
	m.CompactSearchActive = true
	m.resetHelpState()
	m.resetActionMenuState()
	m.resetSettingsState()
	m.resetDeleteState()
	m.resetApplyState()
	m.ShowInfo = false
	m.Err = nil
	m.Notice = ""
}

func (m *Model) closeCompactSearch() {
	m.CompactSearchActive = false
}

func (m *Model) clearCompactSearch() {
	m.CompactSearchActive = false
	m.CompactSearchQuery = ""
	m.normalizeCompactControls()
}

func (m Model) handleCompactSearch(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()
	switch key {
	case "ctrl+c":
		return m, tea.Quit
	case "esc":
		m.closeCompactSearch()
		return m, nil
	case "enter":
		m.closeCompactSearch()
		return m, m.syncAndFetchActiveAccount()
	case "backspace", "delete":
		m.CompactSearchQuery = trimLastRune(m.CompactSearchQuery)
		m.normalizeCompactControls()
		return m, m.syncAndFetchActiveAccount()
	case "ctrl+u":
		m.CompactSearchQuery = ""
		m.normalizeCompactControls()
		return m, m.syncAndFetchActiveAccount()
	}
	if utf8.RuneCountInString(key) == 1 && key >= " " {
		m.CompactSearchQuery += key
		m.normalizeCompactControls()
		return m, m.syncAndFetchActiveAccount()
	}
	return m, nil
}

func (m Model) handleCompactDetailOverlay(keyStr string) (tea.Model, tea.Cmd) {
	switch keyStr {
	case "q", "ctrl+c":
		return m, tea.Quit
	case "esc", "d":
		m.closeCompactDetail()
		return m, nil
	}
	return m, nil
}

func (m Model) handleCompactControlKey(keyStr string) (tea.Model, tea.Cmd, bool) {
	switch keyStr {
	case "ctrl+f":
		m.openCompactSearch()
		return m, nil, true
	case "f":
		m.cycleCompactFilter()
		return m, m.syncAndFetchActiveAccount(), true
	case "g":
		m.cycleCompactSort()
		return m, m.syncAndFetchActiveAccount(), true
	case "p":
		m.toggleCompactPinApplied()
		return m, m.syncAndFetchActiveAccount(), true
	case "e":
		m.toggleCompactExhaustedCollapsed()
		return m, m.syncAndFetchActiveAccount(), true
	case "d":
		m.openCompactDetail()
		return m, nil, true
	case "z":
		m.toggleCompactStatusDensity()
		return m, nil, true
	}
	return m, nil, false
}

func trimLastRune(value string) string {
	value = strings.TrimRight(value, "\r\n")
	if value == "" {
		return ""
	}
	_, size := utf8.DecodeLastRuneInString(value)
	return value[:len(value)-size]
}

func (m *Model) normalizeCompactControls() {
	m.CompactScrollOffset = 0
	order := m.compactVisualOrderIndices()
	if len(order) == 0 {
		return
	}
	for _, index := range order {
		if index == m.ActiveAccountIx {
			m.ensureCompactActiveVisible()
			return
		}
	}
	m.ActiveAccountIx = order[0]
	m.ensureCompactActiveVisible()
}

func (m *Model) openCompactDetail() {
	if m.activeAccount() == nil {
		return
	}
	m.CompactDetailVisible = true
	m.resetHelpState()
	m.resetActionMenuState()
	m.resetSettingsState()
	m.resetDeleteState()
	m.resetApplyState()
	m.ShowInfo = false
	m.Err = nil
	m.Notice = ""
}

func (m *Model) closeCompactDetail() {
	m.CompactDetailVisible = false
}

func (m Model) renderCompactDetailModal() string {
	account := m.activeAccount()
	if account == nil {
		return ""
	}
	lines := []string{
		InfoTitleStyle.Render("Quota details"),
		InfoValueStyle.Render("Account: " + truncateLabel(m.displayAccountLabel(account), 54)),
	}
	if badges := m.activeSourceBadgesForAccount(account); badges != "" {
		lines = append(lines, ActionMenuHintStyle.Render("Applied: "+sourceBadgeLegendText(badges)))
	}
	if plan := strings.TrimSpace(m.PlanTypeByAccount[account.Key]); plan != "" {
		lines = append(lines, InfoValueStyle.Render("Plan: "+plan))
	}
	if err := m.ErrorsMap[account.Key]; err != nil && !m.BackgroundErrorMap[account.Key] {
		lines = append(lines, ErrorStyle.Render("Status: "+compactErrorText(err)))
	}
	data, ok := m.UsageData[account.Key]
	if !ok {
		lines = append(lines, ActionMenuHintStyle.Render("No quota loaded yet."))
	} else {
		lines = append(lines, "")
		for _, window := range data.Windows {
			label := strings.TrimSpace(windowHeader(window))
			if label == "" {
				label = strings.TrimSpace(window.Label)
			}
			if label == "" {
				label = "Quota"
			}
			lines = append(lines, InfoValueStyle.Render(fmt.Sprintf("%s: %.0f%% left, %.0f%% used", label, window.LeftPercent, window.UsedPercent)))
			lines = append(lines, ActionMenuHintStyle.Render("  "+formatResetText(window.ResetAt)))
		}
	}
	lines = append(lines, "", ActionMenuHintStyle.Render("[d/esc] Close"))
	return InfoBoxStyle.Copy().Width(72).Render(strings.Join(lines, "\n"))
}

func compactErrorText(err error) string {
	if err == nil {
		return "error"
	}
	message := strings.ToLower(strings.TrimSpace(err.Error()))
	switch {
	case strings.Contains(message, "401") || strings.Contains(message, "403") || strings.Contains(message, "unauthorized") || strings.Contains(message, "forbidden") || strings.Contains(message, "token"):
		return "auth error"
	case strings.Contains(message, "429") || strings.Contains(message, "rate limit"):
		return "rate limited"
	case strings.Contains(message, "500") || strings.Contains(message, "502") || strings.Contains(message, "503") || strings.Contains(message, "504"):
		return "server error"
	case strings.Contains(message, "timeout"):
		return "timeout"
	case strings.Contains(message, "network") || strings.Contains(message, "connection") || strings.Contains(message, "dial"):
		return "network error"
	}
	if message == "" {
		return "error"
	}
	return truncateLabelStrict(message, 18)
}

func (m Model) compactRefreshingCount() int {
	count := 0
	for _, account := range m.Accounts {
		if account == nil {
			continue
		}
		if m.BackgroundLoadingMap[account.Key] || m.AutoRefreshPending[account.Key] {
			count++
		}
	}
	return count
}

func (m Model) compactAccountIndexAtPoint(x, y int) int {
	if !m.CompactMode || len(m.Accounts) == 0 || m.Width <= 0 || m.Height <= 0 || x < 0 || y < 0 {
		return -1
	}
	viewportHeight := m.compactListViewportHeight()
	columns, columnWidth, gap := m.compactColumnLayout()
	lines := m.compactRenderedLines(viewportHeight, columns, columnWidth, gap)
	row, rowStartX, ok := m.compactRenderedRowAtScreenLine(lines, y)
	if !ok {
		return -1
	}

	accounts := row.accountIndices
	if len(accounts) == 0 {
		return -1
	}
	localX := x - rowStartX
	if localX < 0 {
		return -1
	}
	rowWidth := ansi.StringWidth(ansi.Strip(row.line))
	if localX >= rowWidth {
		return -1
	}
	if columns <= 1 {
		return accounts[0]
	}
	cellWidth := compactColumnLineWidth(columnWidth)
	column := localX / (cellWidth + gap)
	cellX := localX % (cellWidth + gap)
	if column < 0 || column >= columns || column >= len(accounts) || cellX >= cellWidth {
		return -1
	}
	return accounts[column]
}

func (m Model) compactRenderedRowAtScreenLine(rows []compactListRow, y int) (compactListRow, int, bool) {
	viewLines := strings.Split(ansi.Strip(m.View()), "\n")
	if y < 0 || y >= len(viewLines) {
		return compactListRow{}, 0, false
	}
	screenLine := viewLines[y]
	for _, row := range rows {
		if len(row.accountIndices) == 0 {
			continue
		}
		plainRow := strings.TrimRight(ansi.Strip(row.line), " ")
		if strings.TrimSpace(plainRow) == "" {
			continue
		}
		start := strings.Index(screenLine, plainRow)
		if start < 0 {
			continue
		}
		return row, ansi.StringWidth(screenLine[:start]), true
	}
	return compactListRow{}, 0, false
}

func (m *Model) selectCompactAccountAtPoint(x, y int) bool {
	index := m.compactAccountIndexAtPoint(x, y)
	if index < 0 || index >= len(m.Accounts) || index == m.ActiveAccountIx {
		return false
	}
	m.ActiveAccountIx = index
	m.ensureCompactActiveVisible()
	return true
}

func compactModeHintValue(enabled bool, on, off string) string {
	if enabled {
		return on
	}
	return off
}
