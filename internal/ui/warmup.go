package ui

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/deLiseLINO/codex-quota/internal/api"
	"github.com/deLiseLINO/codex-quota/internal/config"
)

type warmupMode string

const (
	warmupSelected warmupMode = "selected"
	warmupFree     warmupMode = "free"
	warmupAll      warmupMode = "all"
)

type WarmupFinishedMsg struct {
	Mode    warmupMode
	Results []WarmupAccountResult
	SaveErr error
}

type WarmupStateLoadedMsg struct {
	State config.WarmupState
	Err   error
}

type WarmupStepMsg struct {
	Result       WarmupAccountResult
	State        config.WarmupState
	StateChanged bool
}

type WarmupAccountResult struct {
	AccountKey string
	Label      string
	Account    *config.Account
	Data       api.UsageData
	HasData    bool
	Warmed     bool
	Skipped    bool
	SkipReason string
	Err        error
}

var (
	callWarmCodex      = api.WarmCodex
	loadWarmupState    = config.LoadWarmupState
	saveWarmupState    = config.SaveWarmupState
	warmupRequestDelay = 750 * time.Millisecond
)

func WarmupCmd(accounts []*config.Account, mode warmupMode) tea.Cmd {
	accountSnapshots := cloneAccounts(accounts)
	if len(accountSnapshots) == 0 {
		return nil
	}

	return func() tea.Msg {
		state, err := loadWarmupState()
		if err != nil {
			return ErrMsg{Err: fmt.Errorf("failed to load warmup state: %w", err)}
		}
		if state.Entries == nil {
			state.Entries = map[string]config.WarmupEntry{}
		}

		results := make([]WarmupAccountResult, 0, len(accountSnapshots))
		changed := false
		for _, account := range accountSnapshots {
			result := warmupOneAccount(account, mode, &state)
			if result.Warmed {
				changed = true
			}
			results = append(results, result)
			if result.Warmed && warmupRequestDelay > 0 {
				time.Sleep(warmupRequestDelay)
			}
		}

		var saveErr error
		if changed {
			saveErr = saveWarmupState(state)
		}

		return WarmupFinishedMsg{Mode: mode, Results: results, SaveErr: saveErr}
	}
}

func LoadWarmupStateCmd() tea.Cmd {
	return func() tea.Msg {
		state, err := loadWarmupState()
		if err != nil {
			return WarmupStateLoadedMsg{Err: fmt.Errorf("failed to load warmup state: %w", err)}
		}
		return WarmupStateLoadedMsg{State: state}
	}
}

func WarmupStepCmd(account *config.Account, mode warmupMode, state config.WarmupState) tea.Cmd {
	accountSnapshot := cloneAccount(account)
	return func() tea.Msg {
		if accountSnapshot == nil {
			return WarmupStepMsg{State: state}
		}

		result := warmupOneAccount(accountSnapshot, mode, &state)
		return WarmupStepMsg{
			Result:       result,
			State:        state,
			StateChanged: result.Warmed,
		}
	}
}

func SaveWarmupStateCmd(state config.WarmupState, changed bool, results []WarmupAccountResult, mode warmupMode) tea.Cmd {
	resultsSnapshot := append([]WarmupAccountResult(nil), results...)
	return func() tea.Msg {
		var saveErr error
		if changed {
			saveErr = saveWarmupState(state)
		}
		return WarmupFinishedMsg{Mode: mode, Results: resultsSnapshot, SaveErr: saveErr}
	}
}

func warmupOneAccount(account *config.Account, mode warmupMode, state *config.WarmupState) WarmupAccountResult {
	result := WarmupAccountResult{
		AccountKey: account.Key,
		Label:      warmupAccountLabel(account),
		Account:    account,
	}

	if strings.TrimSpace(account.AccessToken) == "" {
		result.Skipped = true
		result.SkipReason = "missing access token"
		return result
	}
	if strings.TrimSpace(account.AccountID) == "" {
		result.Skipped = true
		result.SkipReason = "missing account id"
		return result
	}

	if err := prepareWarmupToken(account); err != nil {
		result.Err = err
		return result
	}

	data, err := fetchWarmupQuota(account)
	if err != nil {
		result.Err = err
		return result
	}
	result.Data = data
	result.HasData = true
	result.Account = account

	if mode == warmupFree && !isFreePlan(data.PlanType) {
		result.Skipped = true
		result.SkipReason = "not free plan"
		return result
	}

	if alreadyWarmedForWindow(account, data, *state) {
		result.Skipped = true
		result.SkipReason = "already warmed"
		return result
	}

	err = callWarmCodex(account.AccessToken, account.AccountID)
	if err != nil && api.IsUnauthorized(err) && strings.TrimSpace(account.RefreshToken) != "" {
		if refreshErr := refreshAccountToken(account); refreshErr != nil {
			result.Err = fmt.Errorf("token refresh failed: %w", refreshErr)
			return result
		}
		err = callWarmCodex(account.AccessToken, account.AccountID)
	}
	if err != nil {
		result.Err = err
		return result
	}

	data, err = fetchWarmupQuota(account)
	if err != nil {
		result.Err = fmt.Errorf("warm succeeded but quota refresh failed: %w", err)
		return result
	}
	result.Data = data
	result.HasData = true
	result.Account = account
	result.Warmed = true

	if resetAt := warmupResetAt(data); !resetAt.IsZero() {
		state.Entries[config.WarmupStateKey(account)] = config.WarmupEntry{
			ResetAt:  resetAt,
			WarmedAt: time.Now(),
		}
	}

	return result
}

func prepareWarmupToken(account *config.Account) error {
	if isTokenExpired(account) {
		if err := refreshAccountToken(account); err != nil {
			return fmt.Errorf("token refresh failed: %w", err)
		}
		return nil
	}
	if shouldRefreshTokenSoon(account) && strings.TrimSpace(account.RefreshToken) != "" {
		_ = refreshAccountToken(account)
	}
	return nil
}

func fetchWarmupQuota(account *config.Account) (api.UsageData, error) {
	data, err := callQuotaAPI(account.AccessToken, account.AccountID)
	if err != nil && api.IsUnauthorized(err) && strings.TrimSpace(account.RefreshToken) != "" {
		if refreshErr := refreshAccountToken(account); refreshErr != nil {
			return api.UsageData{}, fmt.Errorf("token refresh failed: %w", refreshErr)
		}
		data, err = callQuotaAPI(account.AccessToken, account.AccountID)
	}
	return data, err
}

func alreadyWarmedForWindow(account *config.Account, data api.UsageData, state config.WarmupState) bool {
	resetAt := warmupResetAt(data)
	if resetAt.IsZero() {
		return false
	}
	entry, ok := state.Entries[config.WarmupStateKey(account)]
	return ok && entry.ResetAt.Equal(resetAt)
}

func warmupResetAt(data api.UsageData) time.Time {
	for _, window := range data.Windows {
		if !window.ResetAt.IsZero() {
			return window.ResetAt
		}
	}
	return time.Time{}
}

func isFreePlan(planType string) bool {
	return strings.EqualFold(strings.TrimSpace(planType), "free")
}

func cloneAccounts(accounts []*config.Account) []*config.Account {
	cloned := make([]*config.Account, 0, len(accounts))
	for _, account := range accounts {
		if snapshot := cloneAccount(account); snapshot != nil {
			cloned = append(cloned, snapshot)
		}
	}
	return cloned
}

func warmupAccountLabel(account *config.Account) string {
	if account == nil {
		return "account"
	}
	for _, value := range []string{account.Label, account.Email, account.AccountID, account.Key} {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return "account"
}

func warmupModeLabel(mode warmupMode) string {
	switch mode {
	case warmupSelected:
		return "selected account"
	case warmupFree:
		return "all free accounts"
	case warmupAll:
		return "all accounts"
	default:
		return "accounts"
	}
}

func warmupSummary(results []WarmupAccountResult, saveErr error) string {
	warmed, skipped, failed := 0, 0, 0
	for _, result := range results {
		switch {
		case result.Warmed:
			warmed++
		case result.Err != nil:
			failed++
		case result.Skipped:
			skipped++
		}
	}

	parts := []string{fmt.Sprintf("warmup complete: %d warmed", warmed)}
	if skipped > 0 {
		parts = append(parts, fmt.Sprintf("%d skipped", skipped))
	}
	if failed > 0 {
		parts = append(parts, fmt.Sprintf("%d failed", failed))
	}
	if saveErr != nil {
		parts = append(parts, "state save failed: "+saveErr.Error())
	}
	return strings.Join(parts, ", ")
}
