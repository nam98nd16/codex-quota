package ui

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/deLiseLINO/codex-quota/internal/api"
	"github.com/deLiseLINO/codex-quota/internal/config"
)

type rateLimitResetStage string

const (
	rateLimitResetConfirm rateLimitResetStage = "confirm"
	rateLimitResetRunning rateLimitResetStage = "running"
	rateLimitResetRetry   rateLimitResetStage = "retry"
	rateLimitResetMessage rateLimitResetStage = "message"
)

var newRateLimitResetID = randomRateLimitResetID

func (m Model) beginRateLimitResetFlow() (tea.Model, tea.Cmd) {
	account := m.activeAccount()
	if account == nil {
		return m, nil
	}
	available, ok := m.activeRateLimitResetCredits()
	if !ok || available <= 0 {
		m.Notice = "no rate-limit resets available for this account"
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
	m.resetWarmupState()
	m.ShowInfo = false
	m.Notice = ""
	m.Err = nil
	m.RateLimitResetVisible = true
	m.RateLimitResetStage = rateLimitResetConfirm
	m.RateLimitResetCursor = 0
	m.RateLimitResetAccountKey = account.Key
	m.RateLimitResetRequestID = newRateLimitResetID()
	m.RateLimitResetMessage = ""
	return m, nil
}

func (m Model) handleRateLimitResetOverlay(keyStr string) (tea.Model, tea.Cmd) {
	switch keyStr {
	case "q", "ctrl+c":
		return m, tea.Quit
	case "esc":
		if m.RateLimitResetStage != rateLimitResetRunning {
			if m.RateLimitResetStage == rateLimitResetConfirm || m.RateLimitResetStage == rateLimitResetRetry {
				return m.cancelRateLimitResetFlow()
			}
			m.resetRateLimitResetState()
		}
		return m, nil
	case "up", "k":
		m.moveRateLimitResetCursor(-1)
		return m, nil
	case "down", "j":
		m.moveRateLimitResetCursor(1)
		return m, nil
	case "1", "2":
		if m.RateLimitResetStage == rateLimitResetConfirm || m.RateLimitResetStage == rateLimitResetRetry {
			m.RateLimitResetCursor = int(keyStr[0] - '1')
			return m.confirmRateLimitResetSelection()
		}
		return m, nil
	case "y":
		if m.RateLimitResetStage == rateLimitResetConfirm || m.RateLimitResetStage == rateLimitResetRetry {
			m.RateLimitResetCursor = 0
			return m.confirmRateLimitResetSelection()
		}
		return m, nil
	case "n":
		if m.RateLimitResetStage == rateLimitResetConfirm || m.RateLimitResetStage == rateLimitResetRetry {
			return m.cancelRateLimitResetFlow()
		}
		return m, nil
	case "enter":
		return m.confirmRateLimitResetSelection()
	}
	return m, nil
}

func (m Model) confirmRateLimitResetSelection() (tea.Model, tea.Cmd) {
	switch m.RateLimitResetStage {
	case rateLimitResetConfirm, rateLimitResetRetry:
		if m.RateLimitResetCursor == 0 {
			return m.startRateLimitResetConsume()
		}
		return m.cancelRateLimitResetFlow()
	case rateLimitResetMessage:
		m.resetRateLimitResetState()
		return m, nil
	default:
		return m, nil
	}
}

func (m Model) cancelRateLimitResetFlow() (tea.Model, tea.Cmd) {
	m.resetRateLimitResetState()
	m.Notice = "rate-limit reset cancelled"
	m.noticeSeq++
	return m, scheduleNoticeClearCmd(m.noticeSeq)
}

func (m Model) startRateLimitResetConsume() (tea.Model, tea.Cmd) {
	account := m.findAccountByKey(m.RateLimitResetAccountKey)
	if account == nil {
		m.RateLimitResetStage = rateLimitResetMessage
		m.RateLimitResetMessage = "Account is no longer available."
		m.RateLimitResetCursor = 0
		return m, nil
	}
	m.RateLimitResetStage = rateLimitResetRunning
	m.RateLimitResetCursor = 0
	m.RateLimitResetMessage = ""
	m.Err = nil
	m.Notice = ""
	return m, ConsumeRateLimitResetCmd(account, m.RateLimitResetRequestID)
}

func (m Model) handleRateLimitResetConsumed(msg RateLimitResetConsumedMsg) (tea.Model, tea.Cmd) {
	if msg.AccountKey != "" {
		m.applyAccountSnapshot(msg.AccountKey, msg.Account)
	}
	if !m.RateLimitResetVisible || msg.RedeemRequestID != m.RateLimitResetRequestID {
		return m, nil
	}

	m.RateLimitResetCursor = 0
	if msg.Err != nil {
		m.RateLimitResetStage = rateLimitResetRetry
		m.RateLimitResetMessage = rateLimitResetErrorMessage(msg.Err)
		return m, nil
	}

	m.RateLimitResetStage = rateLimitResetMessage
	m.RateLimitResetMessage = rateLimitResetOutcomeMessage(msg.Result.Outcome)
	switch msg.Result.Outcome {
	case api.RateLimitResetOutcomeReset, api.RateLimitResetOutcomeAlreadyRedeemed:
		return m, m.refreshAfterRateLimitReset(msg.AccountKey, msg.Account)
	case api.RateLimitResetOutcomeNoCredit:
		m.setRateLimitResetCredits(msg.AccountKey, 0)
		return m, m.refreshAfterRateLimitReset(msg.AccountKey, msg.Account)
	default:
		return m, nil
	}
}

func ConsumeRateLimitResetCmd(account *config.Account, redeemRequestID string) tea.Cmd {
	accountSnapshot := cloneAccount(account)
	if accountSnapshot == nil {
		return nil
	}
	accountKey := accountSnapshot.Key

	return func() tea.Msg {
		workingAccount := *accountSnapshot
		if isTokenExpired(&workingAccount) {
			if err := refreshAccountToken(&workingAccount); err != nil {
				return RateLimitResetConsumedMsg{AccountKey: accountKey, Account: &workingAccount, RedeemRequestID: redeemRequestID, Err: fmt.Errorf("token refresh failed: %w", err)}
			}
		} else if shouldRefreshTokenSoon(&workingAccount) && strings.TrimSpace(workingAccount.RefreshToken) != "" {
			_ = refreshAccountToken(&workingAccount)
		}

		result, err := consumeResetCredit(workingAccount.AccessToken, workingAccount.AccountID, redeemRequestID)
		if err != nil && api.IsUnauthorized(err) && workingAccount.RefreshToken != "" {
			if refreshErr := refreshAccountToken(&workingAccount); refreshErr != nil {
				return RateLimitResetConsumedMsg{AccountKey: accountKey, Account: &workingAccount, RedeemRequestID: redeemRequestID, Err: fmt.Errorf("token refresh failed: %w", refreshErr)}
			}
			result, err = consumeResetCredit(workingAccount.AccessToken, workingAccount.AccountID, redeemRequestID)
		}

		return RateLimitResetConsumedMsg{AccountKey: accountKey, Account: &workingAccount, RedeemRequestID: redeemRequestID, Result: result, Err: err}
	}
}

func (m *Model) resetRateLimitResetState() {
	m.RateLimitResetVisible = false
	m.RateLimitResetStage = ""
	m.RateLimitResetCursor = 0
	m.RateLimitResetAccountKey = ""
	m.RateLimitResetRequestID = ""
	m.RateLimitResetMessage = ""
}

func (m Model) activeRateLimitResetCredits() (int64, bool) {
	account := m.activeAccount()
	if account == nil {
		return 0, false
	}
	data, ok := m.UsageData[account.Key]
	if !ok && account.Key == m.activeAccountKey() {
		data = m.Data
	}
	if data.AvailableRateLimitResetCredits == nil && account.Key == m.activeAccountKey() {
		data = m.Data
	}
	if data.AvailableRateLimitResetCredits == nil {
		return 0, false
	}
	return *data.AvailableRateLimitResetCredits, true
}

func (m *Model) setRateLimitResetCredits(accountKey string, count int64) {
	if accountKey == "" {
		return
	}
	if m.UsageData == nil {
		m.UsageData = make(map[string]api.UsageData)
	}
	data := m.UsageData[accountKey]
	data.AvailableRateLimitResetCredits = &count
	m.UsageData[accountKey] = data
	if accountKey == m.activeAccountKey() {
		m.Data.AvailableRateLimitResetCredits = &count
	}
}

func (m *Model) refreshAfterRateLimitReset(accountKey string, account *config.Account) tea.Cmd {
	if accountKey == "" {
		return nil
	}
	if account == nil {
		account = m.findAccountByKey(accountKey)
	}
	if account == nil {
		return nil
	}
	if m.LoadingMap == nil {
		m.LoadingMap = make(map[string]bool)
	}
	m.LoadingMap[accountKey] = true
	delete(m.ErrorsMap, accountKey)
	delete(m.BackgroundErrorMap, accountKey)
	delete(m.BackgroundLoadingMap, accountKey)
	delete(m.AutoRefreshPending, accountKey)
	if accountKey == m.activeAccountKey() {
		m.Loading = true
	}
	return FetchDataCmd(account, false)
}

func (m *Model) moveRateLimitResetCursor(delta int) {
	count := 0
	switch m.RateLimitResetStage {
	case rateLimitResetConfirm, rateLimitResetRetry:
		count = 2
	default:
		return
	}
	m.RateLimitResetCursor = (m.RateLimitResetCursor + delta + count) % count
}

func rateLimitResetOutcomeMessage(outcome api.RateLimitResetOutcome) string {
	switch outcome {
	case api.RateLimitResetOutcomeReset, api.RateLimitResetOutcomeAlreadyRedeemed:
		return "Usage reset. Checking remaining resets..."
	case api.RateLimitResetOutcomeNothingToReset:
		return "No eligible usage window to reset. Credit was not consumed."
	case api.RateLimitResetOutcomeNoCredit:
		return "No rate-limit resets are available. Credit was not consumed."
	default:
		return "Rate-limit reset finished."
	}
}

func rateLimitResetErrorMessage(err error) string {
	if err == nil {
		return "Couldn't reset usage. Please try again."
	}
	message := strings.TrimSpace(err.Error())
	if message == "" {
		return "Couldn't reset usage. Please try again."
	}
	return "Couldn't reset usage: " + truncateLabelStrict(message, 110)
}

func (m *Model) finishRateLimitResetRefreshMessage(accountKey string, data api.UsageData) {
	if !m.RateLimitResetVisible || m.RateLimitResetStage != rateLimitResetMessage || accountKey != m.RateLimitResetAccountKey {
		return
	}
	if !strings.HasPrefix(m.RateLimitResetMessage, "Usage reset.") {
		return
	}
	if data.AvailableRateLimitResetCredits == nil {
		m.RateLimitResetMessage = "Usage reset. Quota refreshed."
		return
	}
	count := *data.AvailableRateLimitResetCredits
	if count == 1 {
		m.RateLimitResetMessage = "Usage reset. You have 1 rate-limit reset left."
		return
	}
	m.RateLimitResetMessage = fmt.Sprintf("Usage reset. You have %d rate-limit resets left.", count)
}

func randomRateLimitResetID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("reset-%x", time.Now().UnixNano())
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	hexID := hex.EncodeToString(b)
	return fmt.Sprintf("%s-%s-%s-%s-%s", hexID[0:8], hexID[8:12], hexID[12:16], hexID[16:20], hexID[20:32])
}
