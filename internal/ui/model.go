package ui

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/progress"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/deLiseLINO/codex-quota/internal/api"
	"github.com/deLiseLINO/codex-quota/internal/auth"
	"github.com/deLiseLINO/codex-quota/internal/config"
)

type Model struct {
	defaultProgress         progress.Model
	shortProgress           progress.Model
	Data                    api.UsageData
	Loading                 bool
	DeleteSourceSelect      bool
	DeleteSourceOptions     []config.Source
	DeleteSources           map[config.Source]bool
	DeleteSourceCursor      int
	DeleteConfirm           bool
	ApplyTargetSelect       bool
	ApplyTargets            map[config.Source]bool
	ApplyTargetCursor       int
	ApplyConfirm            bool
	ShowInfo                bool
	Notice                  string
	noticeSeq               int
	Err                     error
	Width                   int
	Height                  int
	CompactMode             bool
	UsageData               map[string]api.UsageData
	PlanTypeByAccount       map[string]string
	LoadingMap              map[string]bool
	ErrorsMap               map[string]error
	ExhaustedSticky         map[string]bool
	Accounts                []*config.Account
	SourcesByAccountID      map[string][]string
	ActiveSourcesByIdentity map[string][]string
	ActiveAccountIx         int
	compactBarAnimations    map[string]compactBarAnimation
	tabWindowAnimations     map[string]tabWindowAnimation
	animationTicking        bool
}

type compactBarAnimation struct {
	From      float64
	To        float64
	Current   float64
	StartedAt time.Time
	Duration  time.Duration
}

type tabWindowAnimation struct {
	From      float64
	To        float64
	Current   float64
	StartedAt time.Time
	Duration  time.Duration
}

const (
	animationFrameInterval       = 16 * time.Millisecond
	unifiedAnimationDuration     = 1000 * time.Millisecond
	compactLoadAnimationDuration = unifiedAnimationDuration
	tabLoadAnimationDuration     = unifiedAnimationDuration
	tabSwitchAnimationDuration   = unifiedAnimationDuration
)

func InitialModel(
	accounts []*config.Account,
	sourcesByAccountID map[string][]string,
	activeSourcesByIdentity map[string][]string,
	initialCompactMode bool,
) Model {
	return InitialModelWithUIState(
		accounts,
		sourcesByAccountID,
		activeSourcesByIdentity,
		config.UIState{CompactMode: initialCompactMode},
	)
}

func InitialModelWithUIState(
	accounts []*config.Account,
	sourcesByAccountID map[string][]string,
	activeSourcesByIdentity map[string][]string,
	uiState config.UIState,
) Model {
	defaultProgress := progress.New(
		progress.WithDefaultGradient(),
		progress.WithoutPercentage(),
	)
	shortProgress := progress.New(
		progress.WithGradient("#4285F4", "#34A853"),
		progress.WithoutPercentage(),
	)

	m := Model{
		defaultProgress:         defaultProgress,
		shortProgress:           shortProgress,
		Loading:                 len(accounts) > 0,
		Accounts:                nil,
		SourcesByAccountID:      sourcesByAccountID,
		ActiveSourcesByIdentity: activeSourcesByIdentity,
		ActiveAccountIx:         0,
		CompactMode:             uiState.CompactMode,
		UsageData:               make(map[string]api.UsageData),
		PlanTypeByAccount:       make(map[string]string),
		LoadingMap:              make(map[string]bool),
		ErrorsMap:               make(map[string]error),
		ExhaustedSticky:         make(map[string]bool),
		compactBarAnimations:    make(map[string]compactBarAnimation),
		tabWindowAnimations:     make(map[string]tabWindowAnimation),
	}
	m.Accounts = accounts

	for _, key := range uiState.ExhaustedAccountKeys {
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		m.ExhaustedSticky[key] = true
	}
	m.pruneExhaustedSticky()

	if len(m.Accounts) > 0 {
		m.LoadingMap[m.Accounts[0].Key] = true
	}

	return m
}

func (m Model) Init() tea.Cmd {
	titleCmd := tea.SetWindowTitle("🚀 Codex Quota Monitor")
	if account := m.activeAccount(); account != nil {
		return tea.Batch(titleCmd, FetchDataCmd(account), m.fetchNextCmd())
	}
	return titleCmd
}

func normalizeKey(key string) string {
	ruToEn := map[rune]rune{
		'й': 'q', 'ц': 'w', 'у': 'e', 'к': 'r', 'е': 't', 'н': 'y', 'г': 'u', 'ш': 'i', 'щ': 'o', 'з': 'p', 'х': '[', 'ъ': ']',
		'ф': 'a', 'ы': 's', 'в': 'd', 'а': 'f', 'п': 'g', 'р': 'h', 'о': 'j', 'л': 'k', 'д': 'l', 'ж': ';', 'э': '\'',
		'я': 'z', 'ч': 'x', 'с': 'c', 'м': 'v', 'и': 'b', 'т': 'n', 'ь': 'm', 'б': ',', 'ю': '.',
		'Й': 'Q', 'Ц': 'W', 'У': 'E', 'К': 'R', 'Е': 'T', 'Н': 'Y', 'Г': 'U', 'Ш': 'I', 'Щ': 'O', 'З': 'P', 'Х': '{', 'Ъ': '}',
		'Ф': 'A', 'Ы': 'S', 'В': 'D', 'А': 'F', 'П': 'G', 'Р': 'H', 'О': 'J', 'Л': 'K', 'Д': 'L', 'Ж': ':', 'Э': '"',
		'Я': 'Z', 'Ч': 'X', 'С': 'C', 'М': 'V', 'И': 'B', 'Т': 'N', 'Ь': 'M', 'Б': '<', 'Ю': '>',
	}

	if len(key) > 0 {
		runes := []rune(key)
		if len(runes) == 1 {
			if en, ok := ruToEn[runes[0]]; ok {
				return string(en)
			}
		}
	}
	return key
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		keyStr := normalizeKey(msg.String())

		if m.DeleteSourceSelect {
			return m.handleDeleteSourceSelection(keyStr)
		}
		if m.DeleteConfirm {
			return m.handleDeleteConfirm(keyStr)
		}
		if m.ApplyTargetSelect {
			return m.handleApplyTargetSelection(keyStr)
		}
		if m.ApplyConfirm {
			return m.handleApplyConfirm(keyStr)
		}

		switch keyStr {
		case "x", "delete":
			if len(m.Accounts) == 0 {
				return m, nil
			}
			account := m.activeAccount()
			if account == nil {
				return m, nil
			}
			if !account.Writable {
				m.resetDeleteState()
				m.resetApplyState()
				m.ShowInfo = false
				m.Err = nil
				m.Notice = "cannot delete this account (read-only)"
				m.noticeSeq++
				return m, scheduleNoticeClearCmd(m.noticeSeq)
			}

			sources := m.deletableSourcesForAccount(account)
			if len(sources) == 0 {
				m.resetDeleteState()
				m.resetApplyState()
				m.ShowInfo = false
				m.Err = nil
				m.Notice = "cannot delete this account (no writable source found)"
				m.noticeSeq++
				return m, scheduleNoticeClearCmd(m.noticeSeq)
			}

			m.startDeleteFlow(sources)
			m.ShowInfo = false
			m.Err = nil
			m.Notice = ""
			return m, nil

		case "esc":
			if m.ShowInfo {
				m.ShowInfo = false
				return m, nil
			}
			if m.Err != nil {
				m.Err = nil
				return m, nil
			}
			if m.Notice != "" {
				m.Notice = ""
				return m, nil
			}
			return m, tea.Quit

		case "q", "ctrl+c":
			return m, tea.Quit

		case "r":
			if m.activeAccount() == nil {
				return m, nil
			}
			m.Loading = true
			m.Err = nil
			m.resetDeleteState()
			m.resetApplyState()
			m.Notice = ""

			if m.LoadingMap == nil {
				m.LoadingMap = make(map[string]bool)
			}
			delete(m.UsageData, m.activeAccountKey())
			delete(m.ErrorsMap, m.activeAccountKey())
			delete(m.compactBarAnimations, m.activeAccountKey())
			m.clearTabWindowAnimations()
			return m, m.fetchNextCmd()

		case "R":
			m.Loading = true
			m.Err = nil
			m.resetDeleteState()
			m.resetApplyState()
			m.Notice = ""

			m.UsageData = make(map[string]api.UsageData)
			m.ErrorsMap = make(map[string]error)
			m.LoadingMap = make(map[string]bool)
			m.compactBarAnimations = make(map[string]compactBarAnimation)
			m.tabWindowAnimations = make(map[string]tabWindowAnimation)
			m.animationTicking = false

			return m, m.fetchNextCmd()

		case "i":
			m.ShowInfo = !m.ShowInfo
			m.resetDeleteState()
			m.resetApplyState()
			m.Notice = ""
			return m, nil

		case "v", "c":
			m.CompactMode = !m.CompactMode
			if m.CompactMode {
				m.clearTabWindowAnimations()
			} else {
				m.clearCompactBarAnimations()
			}
			m.resetDeleteState()
			m.resetApplyState()
			m.Notice = ""
			return m, tea.Batch(m.fetchNextCmd(), m.ensureAnimationTickCmd(), SaveUIStateSnapshotCmd(m.uiStateSnapshot()))

		case "n":
			m.Loading = true
			m.Err = nil
			m.resetDeleteState()
			m.resetApplyState()
			m.ShowInfo = false
			m.Notice = ""
			return m, AddAccountCmd()

		case "enter", "o":
			if m.activeAccount() == nil {
				return m, nil
			}
			m.resetDeleteState()
			m.startApplyFlow()
			m.ShowInfo = false
			m.Notice = ""
			m.Err = nil
			return m, nil

		case "right", "l", "down", "j":
			if len(m.Accounts) > 1 {
				if m.CompactMode {
					m.moveActiveAccountCompact(1)
				} else {
					m.ActiveAccountIx = (m.ActiveAccountIx + 1) % len(m.Accounts)
				}
				m.syncActiveAccount()
				return m, tea.Batch(m.fetchNextCmd(), m.ensureAnimationTickCmd())
			}

		case "left", "h", "up", "k":
			if len(m.Accounts) > 1 {
				if m.CompactMode {
					m.moveActiveAccountCompact(-1)
				} else {
					m.ActiveAccountIx = (m.ActiveAccountIx - 1 + len(m.Accounts)) % len(m.Accounts)
				}
				m.syncActiveAccount()
				return m, tea.Batch(m.fetchNextCmd(), m.ensureAnimationTickCmd())
			}
		}

	case tea.WindowSizeMsg:
		m.Width = msg.Width
		m.Height = msg.Height

		barWidth := msg.Width - 72
		if barWidth < 20 {
			barWidth = 20
		}
		if barWidth > 50 {
			barWidth = 50
		}
		m.defaultProgress.Width = barWidth
		m.shortProgress.Width = barWidth

	case AccountsMsg:
		m.Accounts = msg.Accounts
		m.SourcesByAccountID = msg.SourcesByAccountID
		m.ActiveSourcesByIdentity = msg.ActiveSourcesByIdentity
		m.ActiveAccountIx = 0
		m.Data = api.UsageData{}
		m.pruneCompactBarAnimations()
		m.pruneKnownPlanTypes()
		stickyPruned := m.pruneExhaustedSticky()
		m.clearTabWindowAnimations()
		m.resetDeleteState()
		m.resetApplyState()

		if len(m.Accounts) == 0 {
			m.Loading = false
			m.Err = fmt.Errorf("no accounts found; press n to add account")
			m.Notice = ""
			if stickyPruned {
				return m, SaveUIStateSnapshotCmd(m.uiStateSnapshot())
			}
			return m, nil
		}

		if msg.ActiveKey != "" {
			for i, account := range m.Accounts {
				if account != nil && account.Key == msg.ActiveKey {
					m.ActiveAccountIx = i
					break
				}
			}
		}

		m.Loading = true
		m.Err = nil
		m.Notice = msg.Notice

		if m.LoadingMap == nil {
			m.LoadingMap = make(map[string]bool)
		}

		var fetchCmd tea.Cmd
		if m.activeAccount() != nil {
			m.LoadingMap[m.activeAccountKey()] = true
			fetchCmd = FetchDataCmd(m.activeAccount())
		}

		cmds := []tea.Cmd{fetchCmd, m.fetchNextCmd()}
		if stickyPruned {
			cmds = append(cmds, SaveUIStateSnapshotCmd(m.uiStateSnapshot()))
		}
		if msg.Notice != "" {
			m.noticeSeq++
			cmds = append(cmds, scheduleNoticeClearCmd(m.noticeSeq))
		}
		return m, tea.Batch(cmds...)

	case DataMsg:
		prevSnapshot := cloneAccount(m.findAccountByKey(msg.AccountKey))
		m.applyAccountSnapshot(msg.AccountKey, msg.Account)

		if m.UsageData == nil {
			m.UsageData = make(map[string]api.UsageData)
			m.LoadingMap = make(map[string]bool)
			m.ErrorsMap = make(map[string]error)
		}
		if m.ExhaustedSticky == nil {
			m.ExhaustedSticky = make(map[string]bool)
		}

		var (
			prevData    api.UsageData
			hadPrevData bool
			wasLoading  bool
		)
		stickyChanged := false
		if msg.AccountKey != "" {
			prevData, hadPrevData = m.UsageData[msg.AccountKey]
			wasLoading = m.LoadingMap[msg.AccountKey]
			m.UsageData[msg.AccountKey] = msg.Data
			m.setKnownPlanType(msg.AccountKey, msg.Data.PlanType)
			stickyChanged = m.setExhaustedStickyIfConfirmed(msg.AccountKey, msg.Data) || stickyChanged
			m.LoadingMap[msg.AccountKey] = false
			delete(m.ErrorsMap, msg.AccountKey)
			if m.CompactMode {
				m.startCompactBarAnimation(msg.AccountKey, prevData, hadPrevData, msg.Data, wasLoading)
			} else {
				delete(m.compactBarAnimations, msg.AccountKey)
			}
			if msg.AccountKey == m.activeAccountKey() {
				m.startTabWindowAnimations(msg.AccountKey, prevData, hadPrevData, msg.Data, wasLoading, tabLoadAnimationDuration)
			}
		}
		cmds := []tea.Cmd{m.fetchNextCmd(), m.ensureAnimationTickCmd()}
		if stickyChanged {
			cmds = append(cmds, SaveUIStateSnapshotCmd(m.uiStateSnapshot()))
		}
		nextCmd := tea.Batch(cmds...)

		if msg.AccountKey != "" && msg.AccountKey != m.activeAccountKey() {
			return m, nextCmd
		}
		m.Data = msg.Data
		m.Loading = false
		m.Err = nil
		if msg.ReloadAccounts {
			activeKey := msg.ReloadActiveKey
			if activeKey == "" {
				activeKey = msg.AccountKey
			}
			return m, tea.Batch(ReloadAccountsCmd(activeKey), nextCmd)
		}
		if prevSnapshot != nil && msg.Account != nil {
			prevEmail := strings.TrimSpace(prevSnapshot.Email)
			nextEmail := strings.TrimSpace(msg.Account.Email)
			prevID := strings.TrimSpace(prevSnapshot.AccountID)
			nextID := strings.TrimSpace(msg.Account.AccountID)
			if (prevEmail == "" && nextEmail != "") || (prevID != "" && nextID != "" && prevID != nextID) {
				return m, tea.Batch(ReloadAccountsCmd(msg.AccountKey), nextCmd)
			}
		}
		return m, nextCmd

	case NoticeMsg:
		m.Loading = false
		m.Err = nil
		m.Notice = msg.Text
		if msg.Text == "" {
			return m, nil
		}
		m.noticeSeq++
		return m, scheduleNoticeClearCmd(m.noticeSeq)

	case NoticeTimeoutMsg:
		if msg.Seq != m.noticeSeq {
			return m, nil
		}
		m.Notice = ""
		return m, nil

	case ErrMsg:
		if m.ErrorsMap == nil {
			m.ErrorsMap = make(map[string]error)
			m.LoadingMap = make(map[string]bool)
		}
		if msg.AccountKey != "" {
			m.ErrorsMap[msg.AccountKey] = msg.Err
			m.LoadingMap[msg.AccountKey] = false
			delete(m.compactBarAnimations, msg.AccountKey)
			if msg.AccountKey == m.activeAccountKey() {
				m.clearTabWindowAnimations()
			}
		}
		nextCmd := tea.Batch(m.fetchNextCmd(), m.ensureAnimationTickCmd())
		if msg.AccountKey != "" && msg.AccountKey != m.activeAccountKey() {
			return m, nextCmd
		}
		m.Loading = false
		m.Err = msg.Err
		m.Notice = ""
		m.resetDeleteState()
		m.resetApplyState()
		return m, nextCmd

	case progress.FrameMsg:
		defaultModel, defaultCmd := m.defaultProgress.Update(msg)
		m.defaultProgress = defaultModel.(progress.Model)

		shortModel, shortCmd := m.shortProgress.Update(msg)
		m.shortProgress = shortModel.(progress.Model)

		return m, tea.Batch(defaultCmd, shortCmd)

	case AnimationFrameMsg:
		if !m.advanceAnimations(msg.Now) {
			m.animationTicking = false
			return m, nil
		}
		return m, animationTickCmd()
	}

	return m, nil
}

func (m Model) handleDeleteSourceSelection(keyStr string) (tea.Model, tea.Cmd) {
	switch keyStr {
	case "esc":
		m.resetDeleteState()
		return m, nil
	case "up", "k":
		m.moveDeleteSourceCursor(-1)
		return m, nil
	case "down", "j":
		m.moveDeleteSourceCursor(1)
		return m, nil
	case " ":
		m.toggleCurrentDeleteSource()
		return m, nil
	case "enter":
		if len(m.selectedDeleteSources()) == 0 {
			for _, source := range m.DeleteSourceOptions {
				m.setDeleteSourceSelected(source, true)
			}
		}
		m.DeleteSourceSelect = false
		m.DeleteConfirm = true
		return m, nil
	case "a":
		for _, source := range m.DeleteSourceOptions {
			m.setDeleteSourceSelected(source, true)
		}
		return m, nil
	}

	if len(keyStr) == 1 && keyStr[0] >= '1' && keyStr[0] <= '9' {
		index := int(keyStr[0] - '1')
		if index >= 0 && index < len(m.DeleteSourceOptions) {
			m.DeleteSourceCursor = index
			m.toggleCurrentDeleteSource()
		}
	}

	return m, nil
}

func (m Model) handleDeleteConfirm(keyStr string) (tea.Model, tea.Cmd) {
	switch keyStr {
	case "esc":
		m.resetDeleteState()
		return m, nil
	case "enter":
		account := m.activeAccount()
		if account == nil {
			m.resetDeleteState()
			return m, nil
		}

		sources := m.selectedDeleteSources()
		if len(sources) == 0 {
			sources = m.deletableSourcesForAccount(account)
		}
		if len(sources) == 0 {
			return m, nil
		}

		m.Loading = true
		m.Err = nil
		m.Notice = ""
		m.ShowInfo = false
		m.resetApplyState()
		m.resetDeleteState()
		m.Data = api.UsageData{}
		return m, DeleteAccountSourcesCmd(account, sources, account.Key)
	}

	return m, nil
}

func (m Model) handleApplyTargetSelection(keyStr string) (tea.Model, tea.Cmd) {
	switch keyStr {
	case "esc":
		m.resetApplyState()
		return m, nil
	case "up", "k":
		m.moveApplyTargetCursor(-1)
		return m, nil
	case "down", "j":
		m.moveApplyTargetCursor(1)
		return m, nil
	case " ":
		m.toggleCurrentApplyTargetSelection()
		return m, nil
	case "1":
		m.ApplyTargetCursor = 0
		m.toggleCurrentApplyTargetSelection()
		return m, nil
	case "2":
		m.ApplyTargetCursor = 1
		m.toggleCurrentApplyTargetSelection()
		return m, nil
	case "a":
		m.setApplyTargetsAll(true)
		return m, nil
	case "enter":
		if len(m.selectedApplyTargets()) == 0 {
			m.setApplyTargetsAll(true)
		}
		m.ApplyTargetSelect = false
		m.ApplyConfirm = true
		return m, nil
	}

	return m, nil
}

func (m Model) handleApplyConfirm(keyStr string) (tea.Model, tea.Cmd) {
	switch keyStr {
	case "esc":
		m.resetApplyState()
		return m, nil
	case "enter":
		account := m.activeAccount()
		if account == nil {
			m.resetApplyState()
			return m, nil
		}

		m.Loading = true
		m.Err = nil
		m.resetDeleteState()
		m.ShowInfo = false
		m.Notice = ""
		targets := m.selectedApplyTargets()
		if len(targets) == 0 {
			targets = applyTargetsOrdered()
		}
		m.resetApplyState()
		return m, ApplyToTargetsCmd(account, targets)
	}

	return m, nil
}

func AddAccountCmd() tea.Cmd {
	return func() tea.Msg {
		account, err := auth.LoginOpenAICodex()
		if err != nil {
			return ErrMsg{Err: fmt.Errorf("login failed: %w", err)}
		}
		if err := config.UpsertManagedAccount(account); err != nil {
			return ErrMsg{Err: fmt.Errorf("failed to save account: %w", err)}
		}

		result, err := config.LoadAllAccountsWithSources()
		if err != nil {
			return ErrMsg{Err: fmt.Errorf("failed to reload accounts: %w", err)}
		}

		note := "account added"
		if account.Email != "" {
			note = "account added: " + account.Email
		}

		return AccountsMsg{
			ActiveKey:               account.AccountID,
			Accounts:                result.Accounts,
			SourcesByAccountID:      result.SourcesByAccountID,
			ActiveSourcesByIdentity: result.ActiveSourcesByIdentity,
			Notice:                  note,
		}
	}
}

func ApplyToTargetsCmd(account *config.Account, targets []config.Source) tea.Cmd {
	accountSnapshot := cloneAccount(account)
	if accountSnapshot == nil {
		return nil
	}

	targetsSnapshot := dedupeApplyTargets(targets)

	return func() tea.Msg {
		appliedPaths, applyErrors := config.ApplyAccountToTargets(accountSnapshot, targetsSnapshot)
		if len(appliedPaths) == 0 {
			if len(applyErrors) == 0 {
				return ErrMsg{Err: fmt.Errorf("no apply target selected")}
			}
			return ErrMsg{Err: fmt.Errorf("apply failed: %s", formatTargetErrors(applyErrors))}
		}

		result, loadErr := config.LoadAllAccountsWithSources()
		if loadErr != nil {
			note := fmt.Sprintf("applied to: %s", sourceListText(mapKeysSortedBySource(appliedPaths)))
			if len(applyErrors) > 0 {
				note = fmt.Sprintf("%s (errors: %s)", note, formatTargetErrors(applyErrors))
			}
			return NoticeMsg{Text: note}
		}

		note := fmt.Sprintf("applied to: %s", sourceListText(mapKeysSortedBySource(appliedPaths)))
		if len(appliedPaths) == 1 {
			for source, path := range appliedPaths {
				note = fmt.Sprintf("applied to %s: %s", sourceDisplayName(source), filepath.Base(path))
			}
		}
		if len(applyErrors) > 0 {
			note = fmt.Sprintf("%s (errors: %s)", note, formatTargetErrors(applyErrors))
		}

		return AccountsMsg{
			ActiveKey:               accountSnapshot.Key,
			Accounts:                result.Accounts,
			SourcesByAccountID:      result.SourcesByAccountID,
			ActiveSourcesByIdentity: result.ActiveSourcesByIdentity,
			Notice:                  note,
		}
	}
}

func DeleteAccountSourcesCmd(account *config.Account, sources []config.Source, activeKey string) tea.Cmd {
	accountSnapshot := cloneAccount(account)
	if accountSnapshot == nil {
		return nil
	}

	sourcesSnapshot := dedupeSources(sources)
	activeKeySnapshot := activeKey

	return func() tea.Msg {
		deleted := make([]config.Source, 0, len(sourcesSnapshot))
		errorsList := make([]string, 0)
		for _, source := range sourcesSnapshot {
			if err := config.DeleteAccountFromSource(accountSnapshot, source); err != nil {
				errorsList = append(errorsList, fmt.Sprintf("%s: %v", sourceDisplayName(source), err))
				continue
			}
			deleted = append(deleted, source)
		}

		if len(deleted) == 0 {
			errorText := "failed to delete account"
			if len(errorsList) > 0 {
				errorText = fmt.Sprintf("%s: %s", errorText, strings.Join(errorsList, "; "))
			}
			return ErrMsg{Err: fmt.Errorf("%s", errorText)}
		}

		result, err := config.LoadAllAccountsWithSources()
		if err != nil {
			return ErrMsg{Err: fmt.Errorf("failed to reload accounts: %w", err)}
		}

		note := fmt.Sprintf("account deleted from: %s", sourceListText(deleted))
		if len(errorsList) > 0 {
			note = fmt.Sprintf("%s (errors: %s)", note, strings.Join(errorsList, "; "))
		}

		return AccountsMsg{
			ActiveKey:               activeKeySnapshot,
			Accounts:                result.Accounts,
			SourcesByAccountID:      result.SourcesByAccountID,
			ActiveSourcesByIdentity: result.ActiveSourcesByIdentity,
			Notice:                  note,
		}
	}
}

func FetchDataCmd(account *config.Account) tea.Cmd {
	accountSnapshot := cloneAccount(account)
	if accountSnapshot == nil {
		return nil
	}

	accountKey := accountSnapshot.Key

	return func() tea.Msg {
		workingAccount := *accountSnapshot
		reloadAccounts := false

		if auth.IsExpired(&workingAccount) {
			if err := auth.RefreshToken(&workingAccount); err != nil {
				return ErrMsg{AccountKey: accountKey, Err: fmt.Errorf("token refresh failed: %w", err)}
			}
		}

		data, err := api.CallAPI(workingAccount.AccessToken, workingAccount.AccountID)
		if err != nil && api.IsUnauthorized(err) && workingAccount.RefreshToken != "" {
			if refreshErr := auth.RefreshToken(&workingAccount); refreshErr != nil {
				return ErrMsg{AccountKey: accountKey, Err: fmt.Errorf("token refresh failed: %w", refreshErr)}
			}
			data, err = api.CallAPI(workingAccount.AccessToken, workingAccount.AccountID)
		}

		if err != nil {
			return ErrMsg{AccountKey: accountKey, Err: err}
		}

		if strings.TrimSpace(workingAccount.Email) == "" {
			email, name, fetchErr := auth.FetchUserEmail(workingAccount.AccessToken)
			if fetchErr == nil && strings.TrimSpace(email) != "" {
				workingAccount.Email = strings.TrimSpace(email)
				workingAccount.Label = workingAccount.Email
				if strings.TrimSpace(workingAccount.Label) == "" {
					workingAccount.Label = strings.TrimSpace(name)
				}
				_ = config.SaveAccount(&workingAccount)
				reloadAccounts = true
			}
		}

		reloadActiveKey := accountKey
		if strings.TrimSpace(workingAccount.AccountID) != "" {
			reloadActiveKey = workingAccount.AccountID
		}
		return DataMsg{
			AccountKey:      accountKey,
			Data:            data,
			Account:         &workingAccount,
			ReloadAccounts:  reloadAccounts,
			ReloadActiveKey: reloadActiveKey,
		}
	}
}

func ReloadAccountsCmd(activeKey string) tea.Cmd {
	return func() tea.Msg {
		result, err := config.LoadAllAccountsWithSources()
		if err != nil {
			return ErrMsg{Err: fmt.Errorf("failed to reload accounts: %w", err)}
		}
		return AccountsMsg{
			ActiveKey:               activeKey,
			Accounts:                result.Accounts,
			SourcesByAccountID:      result.SourcesByAccountID,
			ActiveSourcesByIdentity: result.ActiveSourcesByIdentity,
		}
	}
}

func scheduleNoticeClearCmd(seq int) tea.Cmd {
	return tea.Tick(1500*time.Millisecond, func(_ time.Time) tea.Msg {
		return NoticeTimeoutMsg{Seq: seq}
	})
}

func animationTickCmd() tea.Cmd {
	return tea.Tick(animationFrameInterval, func(now time.Time) tea.Msg {
		return AnimationFrameMsg{Now: now}
	})
}

func SaveUIStateCmd(compact bool) tea.Cmd {
	return SaveUIStateSnapshotCmd(config.UIState{CompactMode: compact})
}

func SaveUIStateSnapshotCmd(state config.UIState) tea.Cmd {
	return func() tea.Msg {
		_ = config.SaveUIState(state)
		return nil
	}
}

func cloneAccount(account *config.Account) *config.Account {
	if account == nil {
		return nil
	}

	cloned := *account
	return &cloned
}

func (m Model) findAccountByKey(accountKey string) *config.Account {
	if accountKey == "" {
		return nil
	}
	for _, account := range m.Accounts {
		if account == nil {
			continue
		}
		if account.Key == accountKey {
			return account
		}
	}
	return nil
}

func (m *Model) applyAccountSnapshot(accountKey string, snapshot *config.Account) {
	if snapshot == nil || accountKey == "" {
		return
	}

	for _, account := range m.Accounts {
		if account == nil || account.Key != accountKey {
			continue
		}

		account.AccessToken = snapshot.AccessToken
		account.RefreshToken = snapshot.RefreshToken
		account.ExpiresAt = snapshot.ExpiresAt
		if snapshot.ClientID != "" {
			account.ClientID = snapshot.ClientID
		}
		if snapshot.AccountID != "" {
			account.AccountID = snapshot.AccountID
		}
		if snapshot.Email != "" {
			account.Email = snapshot.Email
		}
		if snapshot.Label != "" {
			account.Label = snapshot.Label
		}

		return
	}
}

func (m Model) activeAccount() *config.Account {
	if len(m.Accounts) == 0 {
		return nil
	}
	if m.ActiveAccountIx < 0 || m.ActiveAccountIx >= len(m.Accounts) {
		return nil
	}
	return m.Accounts[m.ActiveAccountIx]
}

func (m Model) compactVisualOrderIndices() []int {
	if len(m.Accounts) == 0 {
		return nil
	}

	normal := make([]int, 0, len(m.Accounts))
	exhausted := make([]int, 0, len(m.Accounts))
	for i, acc := range m.Accounts {
		if acc == nil {
			continue
		}
		if m.isCompactAccountExhausted(acc.Key) {
			exhausted = append(exhausted, i)
		} else {
			normal = append(normal, i)
		}
	}
	return append(normal, exhausted...)
}

func (m *Model) moveActiveAccountCompact(delta int) {
	order := m.compactVisualOrderIndices()
	if len(order) == 0 {
		return
	}

	pos := -1
	for i, idx := range order {
		if idx == m.ActiveAccountIx {
			pos = i
			break
		}
	}
	if pos == -1 {
		m.ActiveAccountIx = order[0]
		return
	}

	next := (pos + delta) % len(order)
	if next < 0 {
		next += len(order)
	}
	m.ActiveAccountIx = order[next]
}

func (m *Model) syncActiveAccount() {
	m.Loading = true
	m.Err = nil
	m.resetDeleteState()
	m.resetApplyState()
	m.Notice = ""
	m.clearTabWindowAnimations()

	if acc := m.activeAccount(); acc != nil {
		if data, ok := m.UsageData[acc.Key]; ok {
			m.Data = data
			m.Loading = false
			m.Err = m.ErrorsMap[acc.Key]
			if !m.CompactMode {
				m.startTabWindowAnimationsFromZero(acc.Key, data, tabSwitchAnimationDuration)
			}
			return
		}
	}
	m.Data = api.UsageData{}
}

func (m *Model) fetchNextCmd() tea.Cmd {
	if m.UsageData == nil {
		m.UsageData = make(map[string]api.UsageData)
	}
	if m.LoadingMap == nil {
		m.LoadingMap = make(map[string]bool)
	}
	if m.ErrorsMap == nil {
		m.ErrorsMap = make(map[string]error)
	}

	const maxConcurrentLoads = 2
	currentlyLoading := 0
	for _, isLoading := range m.LoadingMap {
		if isLoading {
			currentlyLoading++
		}
	}
	if currentlyLoading >= maxConcurrentLoads {
		return nil
	}
	availableSlots := maxConcurrentLoads - currentlyLoading

	checkAccount := func(acc *config.Account) tea.Cmd {
		if acc == nil || acc.Key == "" {
			return nil
		}
		if m.LoadingMap[acc.Key] {
			return nil
		}
		_, hasData := m.UsageData[acc.Key]
		_, hasErr := m.ErrorsMap[acc.Key]

		if !hasData && !hasErr {
			m.LoadingMap[acc.Key] = true
			return FetchDataCmd(acc)
		}
		return nil
	}

	cmds := make([]tea.Cmd, 0, availableSlots)
	for _, acc := range m.Accounts {
		if cmd := checkAccount(acc); cmd != nil {
			cmds = append(cmds, cmd)
			availableSlots--
			if availableSlots == 0 {
				return tea.Batch(cmds...)
			}
		}
	}
	if len(cmds) == 0 {
		return nil
	}
	return tea.Batch(cmds...)
}

func (m Model) activeAccountKey() string {
	account := m.activeAccount()
	if account == nil {
		return ""
	}
	return account.Key
}

func (m *Model) ensureAnimationTickCmd() tea.Cmd {
	if !m.hasActiveAnimations() {
		m.animationTicking = false
		return nil
	}
	if m.animationTicking {
		return nil
	}
	m.animationTicking = true
	return animationTickCmd()
}

func (m *Model) startCompactBarAnimation(accountKey string, prevData api.UsageData, hadPrevData bool, nextData api.UsageData, wasLoading bool) {
	if accountKey == "" {
		return
	}
	target, ok := compactPrimaryRatio(nextData)
	if !ok {
		delete(m.compactBarAnimations, accountKey)
		return
	}

	from := 0.0
	if hadPrevData {
		if prevRatio, hasPrevRatio := compactPrimaryRatio(prevData); hasPrevRatio {
			from = prevRatio
		}
	}
	if !hadPrevData || wasLoading {
		from = 0
	}
	if from == target {
		delete(m.compactBarAnimations, accountKey)
		return
	}

	if m.compactBarAnimations == nil {
		m.compactBarAnimations = make(map[string]compactBarAnimation)
	}
	m.compactBarAnimations[accountKey] = compactBarAnimation{
		From:      from,
		To:        target,
		Current:   from,
		StartedAt: time.Now(),
		Duration:  compactLoadAnimationDuration,
	}
}

func (m *Model) advanceCompactBarAnimations(now time.Time) bool {
	if len(m.compactBarAnimations) == 0 {
		return false
	}
	for key, anim := range m.compactBarAnimations {
		if anim.Duration <= 0 {
			delete(m.compactBarAnimations, key)
			continue
		}
		elapsed := now.Sub(anim.StartedAt)
		if elapsed <= 0 {
			continue
		}
		progress := float64(elapsed) / float64(anim.Duration)
		if progress >= 1 {
			delete(m.compactBarAnimations, key)
			continue
		}
		if progress < 0 {
			progress = 0
		}
		eased := 1 - (1-progress)*(1-progress)
		anim.Current = anim.From + (anim.To-anim.From)*eased
		m.compactBarAnimations[key] = anim
	}
	return len(m.compactBarAnimations) > 0
}

func (m Model) compactBarRatio(accountKey string, fallback float64) float64 {
	anim, ok := m.compactBarAnimations[accountKey]
	if !ok {
		return fallback
	}
	return anim.Current
}

func (m *Model) pruneCompactBarAnimations() {
	if len(m.compactBarAnimations) == 0 {
		return
	}
	valid := make(map[string]struct{}, len(m.Accounts))
	for _, acc := range m.Accounts {
		if acc == nil || acc.Key == "" {
			continue
		}
		valid[acc.Key] = struct{}{}
	}
	for key := range m.compactBarAnimations {
		if _, ok := valid[key]; !ok {
			delete(m.compactBarAnimations, key)
		}
	}
}

func (m *Model) clearCompactBarAnimations() {
	if len(m.compactBarAnimations) == 0 {
		m.animationTicking = false
		return
	}
	for key := range m.compactBarAnimations {
		delete(m.compactBarAnimations, key)
	}
	m.animationTicking = false
}

func (m *Model) clearTabWindowAnimations() {
	if len(m.tabWindowAnimations) == 0 {
		m.animationTicking = false
		return
	}
	for key := range m.tabWindowAnimations {
		delete(m.tabWindowAnimations, key)
	}
	m.animationTicking = false
}

func tabWindowKey(accountKey string, window api.QuotaWindow) string {
	return fmt.Sprintf("%s|%d|%s", accountKey, window.WindowSec, strings.TrimSpace(window.Label))
}

func (m *Model) startTabWindowAnimations(accountKey string, prevData api.UsageData, hadPrevData bool, nextData api.UsageData, wasLoading bool, duration time.Duration) {
	if accountKey == "" {
		return
	}
	m.removeTabWindowAnimationsForAccount(accountKey)

	previousByKey := make(map[string]float64, len(prevData.Windows))
	for _, window := range prevData.Windows {
		previousByKey[tabWindowKey(accountKey, window)] = clampRatio(window.LeftPercent / 100)
	}

	for _, window := range nextData.Windows {
		key := tabWindowKey(accountKey, window)
		target := clampRatio(window.LeftPercent / 100)
		from := 0.0
		if hadPrevData && !wasLoading {
			if prev, ok := previousByKey[key]; ok {
				from = prev
			}
		}
		if from == target {
			delete(m.tabWindowAnimations, key)
			continue
		}
		if m.tabWindowAnimations == nil {
			m.tabWindowAnimations = make(map[string]tabWindowAnimation)
		}
		m.tabWindowAnimations[key] = tabWindowAnimation{
			From:      from,
			To:        target,
			Current:   from,
			StartedAt: time.Now(),
			Duration:  duration,
		}
	}
}

func (m *Model) startTabWindowAnimationsFromZero(accountKey string, nextData api.UsageData, duration time.Duration) {
	if accountKey == "" {
		return
	}
	m.removeTabWindowAnimationsForAccount(accountKey)
	for _, window := range nextData.Windows {
		key := tabWindowKey(accountKey, window)
		target := clampRatio(window.LeftPercent / 100)
		if target == 0 {
			delete(m.tabWindowAnimations, key)
			continue
		}
		if m.tabWindowAnimations == nil {
			m.tabWindowAnimations = make(map[string]tabWindowAnimation)
		}
		m.tabWindowAnimations[key] = tabWindowAnimation{
			From:      0,
			To:        target,
			Current:   0,
			StartedAt: time.Now(),
			Duration:  duration,
		}
	}
}

func (m *Model) removeTabWindowAnimationsForAccount(accountKey string) {
	if accountKey == "" || len(m.tabWindowAnimations) == 0 {
		return
	}
	prefix := accountKey + "|"
	for key := range m.tabWindowAnimations {
		if strings.HasPrefix(key, prefix) {
			delete(m.tabWindowAnimations, key)
		}
	}
}

func (m *Model) advanceTabWindowAnimations(now time.Time) bool {
	if len(m.tabWindowAnimations) == 0 {
		return false
	}
	for key, anim := range m.tabWindowAnimations {
		if anim.Duration <= 0 {
			delete(m.tabWindowAnimations, key)
			continue
		}
		elapsed := now.Sub(anim.StartedAt)
		if elapsed <= 0 {
			continue
		}
		progress := float64(elapsed) / float64(anim.Duration)
		if progress >= 1 {
			delete(m.tabWindowAnimations, key)
			continue
		}
		if progress < 0 {
			progress = 0
		}
		eased := 1 - (1-progress)*(1-progress)
		anim.Current = anim.From + (anim.To-anim.From)*eased
		m.tabWindowAnimations[key] = anim
	}
	return len(m.tabWindowAnimations) > 0
}

func (m *Model) advanceAnimations(now time.Time) bool {
	advanced := false
	if m.CompactMode {
		advanced = m.advanceCompactBarAnimations(now)
	} else {
		advanced = m.advanceTabWindowAnimations(now)
	}
	return advanced
}

func (m *Model) hasActiveAnimations() bool {
	if m.CompactMode {
		return len(m.compactBarAnimations) > 0
	}
	return len(m.tabWindowAnimations) > 0
}

func (m Model) tabWindowRatio(accountKey string, window api.QuotaWindow, fallback float64) float64 {
	if accountKey == "" || len(m.tabWindowAnimations) == 0 {
		return fallback
	}
	anim, ok := m.tabWindowAnimations[tabWindowKey(accountKey, window)]
	if !ok {
		return fallback
	}
	return anim.Current
}

func (m *Model) startDeleteFlow(sources []config.Source) {
	m.resetDeleteState()
	m.resetApplyState()

	m.DeleteSourceOptions = dedupeSources(sources)
	m.DeleteSources = make(map[config.Source]bool, len(m.DeleteSourceOptions))
	for _, source := range m.DeleteSourceOptions {
		m.DeleteSources[source] = true
	}

	m.DeleteSourceCursor = 0
	m.DeleteSourceSelect = len(m.DeleteSourceOptions) > 1
	m.DeleteConfirm = !m.DeleteSourceSelect
}

func (m *Model) resetDeleteState() {
	m.DeleteSourceSelect = false
	m.DeleteSourceOptions = nil
	m.DeleteSources = nil
	m.DeleteSourceCursor = 0
	m.DeleteConfirm = false
}

func (m *Model) resetApplyState() {
	m.ApplyTargetSelect = false
	m.ApplyConfirm = false
	m.ApplyTargets = nil
	m.ApplyTargetCursor = 0
}

func (m *Model) startApplyFlow() {
	m.resetApplyState()
	m.ApplyTargetSelect = true
	m.ApplyTargets = map[config.Source]bool{
		config.SourceCodex:    true,
		config.SourceOpenCode: true,
	}
	m.ApplyTargetCursor = 0
}

func (m *Model) toggleApplyTargetSelection(source config.Source) {
	if source != config.SourceCodex && source != config.SourceOpenCode {
		return
	}
	if m.ApplyTargets == nil {
		m.ApplyTargets = map[config.Source]bool{}
	}
	if m.ApplyTargets[source] && m.selectedApplyTargetCount() <= 1 {
		return
	}
	m.ApplyTargets[source] = !m.ApplyTargets[source]
}

func (m *Model) toggleCurrentApplyTargetSelection() {
	targets := applyTargetsOrdered()
	if len(targets) == 0 {
		return
	}
	if m.ApplyTargetCursor < 0 || m.ApplyTargetCursor >= len(targets) {
		m.ApplyTargetCursor = 0
	}
	m.toggleApplyTargetSelection(targets[m.ApplyTargetCursor])
}

func (m *Model) moveApplyTargetCursor(delta int) {
	targets := applyTargetsOrdered()
	if len(targets) == 0 {
		m.ApplyTargetCursor = 0
		return
	}
	m.ApplyTargetCursor = (m.ApplyTargetCursor + delta + len(targets)) % len(targets)
}

func (m *Model) setApplyTargetsAll(selected bool) {
	if m.ApplyTargets == nil {
		m.ApplyTargets = map[config.Source]bool{}
	}
	for _, source := range applyTargetsOrdered() {
		m.ApplyTargets[source] = selected
	}
}

func (m Model) selectedApplyTargets() []config.Source {
	targets := make([]config.Source, 0, 2)
	for _, source := range applyTargetsOrdered() {
		if m.ApplyTargets != nil && m.ApplyTargets[source] {
			targets = append(targets, source)
		}
	}
	return targets
}

func (m Model) selectedApplyTargetCount() int {
	count := 0
	for _, source := range applyTargetsOrdered() {
		if m.ApplyTargets != nil && m.ApplyTargets[source] {
			count++
		}
	}
	return count
}

func applyTargetsOrdered() []config.Source {
	return []config.Source{config.SourceCodex, config.SourceOpenCode}
}

func dedupeApplyTargets(targets []config.Source) []config.Source {
	seen := map[config.Source]bool{}
	for _, target := range targets {
		if target != config.SourceCodex && target != config.SourceOpenCode {
			continue
		}
		seen[target] = true
	}

	output := make([]config.Source, 0, len(seen))
	for _, source := range applyTargetsOrdered() {
		if seen[source] {
			output = append(output, source)
		}
	}
	return output
}

func mapKeysSortedBySource(values map[config.Source]string) []config.Source {
	keys := make([]config.Source, 0, len(values))
	for source := range values {
		keys = append(keys, source)
	}
	return dedupeApplyTargets(keys)
}

func formatTargetErrors(errorsByTarget map[config.Source]error) string {
	if len(errorsByTarget) == 0 {
		return ""
	}
	parts := make([]string, 0, len(errorsByTarget))
	for _, source := range applyTargetsOrdered() {
		err, ok := errorsByTarget[source]
		if !ok || err == nil {
			continue
		}
		parts = append(parts, fmt.Sprintf("%s: %v", sourceDisplayName(source), err))
	}
	return strings.Join(parts, "; ")
}

func (m Model) deletableSourcesForAccount(account *config.Account) []config.Source {
	if account == nil {
		return nil
	}

	seen := m.collectKnownSourcesForAccount(account)

	if len(seen) == 0 {
		if account.Source == config.SourceManaged || account.Source == config.SourceOpenCode || account.Source == config.SourceCodex {
			seen[account.Source] = true
		}
	}

	if strings.TrimSpace(account.AccountID) == "" && strings.TrimSpace(account.Email) == "" {
		delete(seen, config.SourceManaged)
	}

	return orderedSources(seen)
}

func (m Model) collectKnownSourcesForAccount(account *config.Account) map[config.Source]bool {
	seen := map[config.Source]bool{}
	if account == nil {
		return seen
	}

	appendLabels := func(labels []string) {
		for _, label := range labels {
			source, ok := sourceFromLabel(label)
			if !ok {
				continue
			}
			seen[source] = true
		}
	}

	if m.SourcesByAccountID != nil {
		if accountID := strings.TrimSpace(account.AccountID); accountID != "" {
			appendLabels(m.SourcesByAccountID[accountID])
		}
		if email := strings.ToLower(strings.TrimSpace(account.Email)); email != "" {
			appendLabels(m.SourcesByAccountID["email:"+email])
		}
	}

	if m.ActiveSourcesByIdentity != nil {
		for _, key := range config.ActiveIdentityKeys(account) {
			appendLabels(m.ActiveSourcesByIdentity[key])
		}
	}

	return seen
}

func orderedSources(sourceMap map[config.Source]bool) []config.Source {
	if len(sourceMap) == 0 {
		return nil
	}

	ordered := []config.Source{config.SourceManaged, config.SourceOpenCode, config.SourceCodex}
	out := make([]config.Source, 0, len(sourceMap))
	for _, source := range ordered {
		if sourceMap[source] {
			out = append(out, source)
		}
	}
	return out
}

func sourceFromLabel(label string) (config.Source, bool) {
	switch strings.ToLower(strings.TrimSpace(label)) {
	case "app", "managed":
		return config.SourceManaged, true
	case "opencode":
		return config.SourceOpenCode, true
	case "codex":
		return config.SourceCodex, true
	default:
		return "", false
	}
}

func dedupeSources(sources []config.Source) []config.Source {
	seen := make(map[config.Source]bool, len(sources))
	for _, source := range sources {
		if source != config.SourceManaged && source != config.SourceOpenCode && source != config.SourceCodex {
			continue
		}
		seen[source] = true
	}
	return orderedSources(seen)
}

func sourceDisplayName(source config.Source) string {
	switch source {
	case config.SourceManaged:
		return "app"
	case config.SourceOpenCode:
		return "opencode"
	case config.SourceCodex:
		return "codex"
	default:
		return string(source)
	}
}

func sourceListText(sources []config.Source) string {
	if len(sources) == 0 {
		return "n/a"
	}
	parts := make([]string, 0, len(sources))
	for _, source := range sources {
		parts = append(parts, sourceDisplayName(source))
	}
	return strings.Join(parts, ", ")
}

func (m Model) activeSourceBadgesForAccount(account *config.Account) string {
	if account == nil || len(m.ActiveSourcesByIdentity) == 0 {
		return ""
	}

	hasCodex := false
	hasOpenCode := false
	appendLabels := func(labels []string) {
		for _, label := range labels {
			source, ok := sourceFromLabel(label)
			if !ok {
				continue
			}
			if source == config.SourceCodex {
				hasCodex = true
			}
			if source == config.SourceOpenCode {
				hasOpenCode = true
			}
		}
	}

	for _, key := range config.ActiveIdentityKeys(account) {
		appendLabels(m.ActiveSourcesByIdentity[key])
	}

	if !hasCodex && !hasOpenCode {
		return ""
	}

	parts := make([]string, 0, 2)
	if hasCodex {
		parts = append(parts, "C")
	}
	if hasOpenCode {
		parts = append(parts, "O")
	}
	return strings.Join(parts, "•")
}

func (m Model) hasSubscription(account *config.Account) bool {
	if account == nil || account.Key == "" {
		return false
	}
	return m.isPaidByKnownPlan(account.Key)
}

func (m *Model) setKnownPlanType(accountKey string, planType string) {
	if accountKey == "" {
		return
	}
	normalized := strings.ToLower(strings.TrimSpace(planType))
	if normalized == "" {
		return
	}
	if m.PlanTypeByAccount == nil {
		m.PlanTypeByAccount = make(map[string]string)
	}
	m.PlanTypeByAccount[accountKey] = normalized
}

func (m Model) isPaidByKnownPlan(accountKey string) bool {
	if accountKey == "" || m.PlanTypeByAccount == nil {
		return false
	}
	planType := strings.ToLower(strings.TrimSpace(m.PlanTypeByAccount[accountKey]))
	if planType == "" {
		return false
	}
	return planType != "free"
}

func (m *Model) pruneKnownPlanTypes() {
	if len(m.PlanTypeByAccount) == 0 {
		return
	}
	valid := make(map[string]struct{}, len(m.Accounts))
	for _, acc := range m.Accounts {
		if acc == nil || acc.Key == "" {
			continue
		}
		valid[acc.Key] = struct{}{}
	}
	for key := range m.PlanTypeByAccount {
		if _, ok := valid[key]; !ok {
			delete(m.PlanTypeByAccount, key)
		}
	}
}

func (m *Model) setExhaustedStickyIfConfirmed(accountKey string, data api.UsageData) bool {
	if accountKey == "" || !isConfirmedExhausted(data) {
		return false
	}
	if m.ExhaustedSticky == nil {
		m.ExhaustedSticky = make(map[string]bool)
	}
	if m.ExhaustedSticky[accountKey] {
		return false
	}
	m.ExhaustedSticky[accountKey] = true
	return true
}

func (m *Model) pruneExhaustedSticky() bool {
	if len(m.ExhaustedSticky) == 0 {
		return false
	}
	valid := make(map[string]struct{}, len(m.Accounts))
	for _, acc := range m.Accounts {
		if acc == nil || acc.Key == "" {
			continue
		}
		valid[acc.Key] = struct{}{}
	}

	changed := false
	for key := range m.ExhaustedSticky {
		if _, ok := valid[key]; ok {
			continue
		}
		delete(m.ExhaustedSticky, key)
		changed = true
	}
	return changed
}

func (m Model) exhaustedStickyKeys() []string {
	if len(m.ExhaustedSticky) == 0 {
		return nil
	}
	keys := make([]string, 0, len(m.ExhaustedSticky))
	for key, exhausted := range m.ExhaustedSticky {
		if !exhausted || strings.TrimSpace(key) == "" {
			continue
		}
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func (m Model) uiStateSnapshot() config.UIState {
	return config.UIState{
		CompactMode:          m.CompactMode,
		ExhaustedAccountKeys: m.exhaustedStickyKeys(),
	}
}

func (m Model) renderActiveSourceBadges(account *config.Account, isRowActive bool) string {
	raw := m.activeSourceBadgesForAccount(account)
	if raw == "" {
		return ""
	}

	cStyle := SourceCodexBadgeMutedStyle
	oStyle := SourceOpenCodeBadgeMutedStyle
	if isRowActive {
		cStyle = SourceCodexBadgeActiveStyle
		oStyle = SourceOpenCodeBadgeActiveStyle
	}

	var b strings.Builder
	b.WriteString(SourceBadgeBracketStyle.Render("["))
	for _, r := range raw {
		switch r {
		case 'C':
			b.WriteString(cStyle.Render("C"))
		case 'O':
			b.WriteString(oStyle.Render("O"))
		case '•':
			b.WriteString(SourceBadgeSeparatorStyle.Render("•"))
		default:
			b.WriteRune(r)
		}
	}
	b.WriteString(SourceBadgeBracketStyle.Render("]"))
	return b.String()
}

func (m Model) activeSourceBadgesDisplayWidth(account *config.Account) int {
	raw := m.activeSourceBadgesForAccount(account)
	if raw == "" {
		return 0
	}
	// Include wrapping brackets: [C•O]
	return len([]rune(raw)) + 2
}

func (m *Model) selectedDeleteSources() []config.Source {
	if len(m.DeleteSourceOptions) == 0 {
		return nil
	}
	selected := make([]config.Source, 0, len(m.DeleteSourceOptions))
	for _, source := range m.DeleteSourceOptions {
		if m.isDeleteSourceSelected(source) {
			selected = append(selected, source)
		}
	}
	return selected
}

func (m *Model) toggleDeleteSource(source config.Source) {
	if m.DeleteSources == nil {
		m.DeleteSources = map[config.Source]bool{}
	}
	if m.DeleteSources[source] && m.deleteSourceCount() <= 1 {
		return
	}
	m.DeleteSources[source] = !m.DeleteSources[source]
}

func (m *Model) toggleCurrentDeleteSource() {
	if len(m.DeleteSourceOptions) == 0 {
		return
	}
	if m.DeleteSourceCursor < 0 || m.DeleteSourceCursor >= len(m.DeleteSourceOptions) {
		m.DeleteSourceCursor = 0
	}
	m.toggleDeleteSource(m.DeleteSourceOptions[m.DeleteSourceCursor])
}

func (m *Model) moveDeleteSourceCursor(delta int) {
	if len(m.DeleteSourceOptions) == 0 {
		m.DeleteSourceCursor = 0
		return
	}
	m.DeleteSourceCursor = (m.DeleteSourceCursor + delta + len(m.DeleteSourceOptions)) % len(m.DeleteSourceOptions)
}

func (m *Model) setDeleteSourceSelected(source config.Source, selected bool) {
	if m.DeleteSources == nil {
		m.DeleteSources = map[config.Source]bool{}
	}
	m.DeleteSources[source] = selected
}

func (m Model) isDeleteSourceSelected(source config.Source) bool {
	if m.DeleteSources == nil {
		return false
	}
	return m.DeleteSources[source]
}

func (m Model) deleteSourceCount() int {
	count := 0
	for _, source := range m.DeleteSourceOptions {
		if m.isDeleteSourceSelected(source) {
			count++
		}
	}
	return count
}
