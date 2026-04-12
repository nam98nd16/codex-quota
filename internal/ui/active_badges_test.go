package ui

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"

	"github.com/deLiseLINO/codex-quota/internal/api"
	"github.com/deLiseLINO/codex-quota/internal/config"
)

func TestActiveSourceBadgesForAccount(t *testing.T) {
	account := &config.Account{
		Key:       "acc-1",
		Label:     "user@example.com",
		Email:     "user@example.com",
		AccountID: "acc-1",
		Source:    config.SourceManaged,
		Writable:  true,
	}

	m := InitialModel([]*config.Account{account}, map[string][]string{}, map[string][]string{
		"email-account:user@example.com|acc-1": []string{"codex", "opencode"},
	}, false)

	if got := m.activeSourceBadgesForAccount(account); got != "C•O" {
		t.Fatalf("badges mismatch: got %q, want %q", got, "C•O")
	}
}

func TestRenderAccountTabs_ShowsActiveBadges(t *testing.T) {
	account := &config.Account{
		Key:       "acc-1",
		Label:     "user@example.com",
		Email:     "user@example.com",
		AccountID: "acc-1",
		Source:    config.SourceManaged,
		Writable:  true,
	}

	m := InitialModel([]*config.Account{account}, map[string][]string{}, map[string][]string{
		"email-account:user@example.com|acc-1": []string{"codex"},
	}, false)

	out := ansi.Strip(m.renderAccountTabs())
	if !strings.Contains(out, "[C]") {
		t.Fatalf("expected [C] badge in tabs output, got: %s", out)
	}
}

func TestRenderCompactView_ShowsActiveBadges(t *testing.T) {
	account := &config.Account{
		Key:       "acc-1",
		Label:     "user@example.com",
		Email:     "user@example.com",
		AccountID: "acc-1",
		Source:    config.SourceManaged,
		Writable:  true,
	}

	m := InitialModel([]*config.Account{account}, map[string][]string{}, map[string][]string{
		"email-account:user@example.com|acc-1": []string{"opencode"},
	}, true)
	m.Loading = false
	m.Width = 120
	m.UsageData = map[string]api.UsageData{
		"acc-1": {
			Windows: []api.QuotaWindow{
				{
					Label:       "Weekly usage limit",
					WindowSec:   604800,
					LeftPercent: 10,
					ResetAt:     time.Now().Add(1 * time.Hour),
				},
			},
		},
	}
	m.LoadingMap = map[string]bool{}
	m.ErrorsMap = map[string]error{}

	out := ansi.Strip(m.renderCompactView())
	if !strings.Contains(out, "[O]") {
		t.Fatalf("expected [O] badge in compact output, got: %s", out)
	}
}

func TestRenderAccountTabs_ShowsRefreshIndicatorForBackgroundLoad(t *testing.T) {
	account := &config.Account{
		Key:       "acc-1",
		Label:     "user@example.com",
		Email:     "user@example.com",
		AccountID: "acc-1",
		Source:    config.SourceManaged,
		Writable:  true,
	}

	m := InitialModel([]*config.Account{account}, map[string][]string{}, map[string][]string{}, false)
	m.BackgroundLoadingMap = map[string]bool{"acc-1": true}

	out := ansi.Strip(m.renderAccountTabs())
	if !strings.Contains(out, "↻") {
		t.Fatalf("expected refresh indicator in tabs output, got: %s", out)
	}
}

func TestRenderCompactView_ShowsRefreshIndicatorForBackgroundLoad(t *testing.T) {
	account := &config.Account{
		Key:       "acc-1",
		Label:     "user@example.com",
		Email:     "user@example.com",
		AccountID: "acc-1",
		Source:    config.SourceManaged,
		Writable:  true,
	}

	m := InitialModel([]*config.Account{account}, map[string][]string{}, map[string][]string{}, true)
	m.Loading = false
	m.Width = 120
	m.BackgroundLoadingMap = map[string]bool{"acc-1": true}
	m.UsageData = map[string]api.UsageData{
		"acc-1": {
			Windows: []api.QuotaWindow{{
				Label:       "Weekly usage limit",
				WindowSec:   604800,
				LeftPercent: 10,
				ResetAt:     time.Now().Add(time.Hour),
			}},
		},
	}
	m.LoadingMap = map[string]bool{}
	m.ErrorsMap = map[string]error{}

	out := ansi.Strip(m.renderCompactView())
	if !strings.Contains(out, "↻") {
		t.Fatalf("expected refresh indicator in compact output, got: %s", out)
	}
}

func TestActiveSourceBadgesForAccount_MatchesByTokenFallback(t *testing.T) {
	account := &config.Account{
		Key:         "acc-2",
		Label:       "n/a",
		AccessToken: "same-access-token",
		Source:      config.SourceManaged,
		Writable:    true,
	}

	activeAccount := &config.Account{
		AccessToken: "same-access-token",
	}
	keys := config.ExactActiveIdentityKeys(activeAccount)
	if len(keys) == 0 {
		t.Fatalf("expected non-empty active identity keys for token")
	}

	activeMap := map[string][]string{}
	for _, key := range keys {
		activeMap[key] = []string{"codex"}
	}

	m := InitialModel([]*config.Account{account}, map[string][]string{}, activeMap, false)
	if got := m.activeSourceBadgesForAccount(account); got != "C" {
		t.Fatalf("badges mismatch with token fallback: got %q, want %q", got, "C")
	}
}

func TestActiveSourceBadgesDisplayWidth_IncludesBrackets(t *testing.T) {
	account := &config.Account{
		Key:       "acc-3",
		Label:     "user@example.com",
		Email:     "user@example.com",
		AccountID: "acc-3",
		Source:    config.SourceManaged,
	}
	m := InitialModel([]*config.Account{account}, map[string][]string{}, map[string][]string{
		"email-account:user@example.com|acc-3": []string{"codex", "opencode"},
	}, false)

	if got := m.activeSourceBadgesDisplayWidth(account); got != 5 {
		t.Fatalf("expected display width 5 for [C•O], got %d", got)
	}
}

func TestActiveSourceBadgesForAccount_DoesNotMatchSharedAccountIDWhenUserDiffers(t *testing.T) {
	sharedAccountID := "shared-account-id"
	activeAccount := &config.Account{
		Label:        "active@example.com",
		Email:        "active@example.com",
		UserID:       "user-active",
		AccountID:    sharedAccountID,
		AccessToken:  "active-token",
		RefreshToken: "active-refresh",
	}
	activeMap := map[string][]string{}
	for _, key := range config.ExactActiveIdentityKeys(activeAccount) {
		activeMap[key] = []string{"codex", "opencode"}
	}

	activeRow := &config.Account{
		Key:          "active",
		Label:        "active@example.com",
		Email:        "active@example.com",
		UserID:       "user-active",
		AccountID:    sharedAccountID,
		AccessToken:  "active-token",
		RefreshToken: "active-refresh",
		Source:       config.SourceManaged,
	}
	differentUserSameAccount := &config.Account{
		Key:          "other",
		Label:        "other@example.com",
		Email:        "other@example.com",
		UserID:       "user-other",
		AccountID:    sharedAccountID,
		AccessToken:  "other-token",
		RefreshToken: "other-refresh",
		Source:       config.SourceManaged,
	}

	m := InitialModel([]*config.Account{activeRow, differentUserSameAccount}, map[string][]string{}, activeMap, false)

	if got := m.activeSourceBadgesForAccount(activeRow); got != "C•O" {
		t.Fatalf("expected active row badges %q, got %q", "C•O", got)
	}
	if got := m.activeSourceBadgesForAccount(differentUserSameAccount); got != "" {
		t.Fatalf("expected no badge for different user sharing account id, got %q", got)
	}
}

func TestActiveSourceBadgesForAccount_DoesNotMatchSameUserWhenAccountDiffers(t *testing.T) {
	activeAccount := &config.Account{
		Label:        "user@example.com",
		Email:        "user@example.com",
		UserID:       "user-1",
		AccountID:    "account-a",
		AccessToken:  "active-token",
		RefreshToken: "active-refresh",
	}
	activeMap := map[string][]string{}
	for _, key := range config.ExactActiveIdentityKeys(activeAccount) {
		activeMap[key] = []string{"codex", "opencode"}
	}

	activeRow := &config.Account{
		Key:          "active",
		Label:        "user@example.com",
		Email:        "user@example.com",
		UserID:       "user-1",
		AccountID:    "account-a",
		AccessToken:  "active-token",
		RefreshToken: "active-refresh",
		Source:       config.SourceManaged,
	}
	differentWorkspace := &config.Account{
		Key:          "workspace",
		Label:        "user@example.com",
		Email:        "user@example.com",
		UserID:       "user-1",
		AccountID:    "account-b",
		AccessToken:  "workspace-token",
		RefreshToken: "workspace-refresh",
		Source:       config.SourceManaged,
	}

	m := InitialModel([]*config.Account{activeRow, differentWorkspace}, map[string][]string{}, activeMap, false)

	if got := m.activeSourceBadgesForAccount(activeRow); got != "C•O" {
		t.Fatalf("expected active row badges %q, got %q", "C•O", got)
	}
	if got := m.activeSourceBadgesForAccount(differentWorkspace); got != "" {
		t.Fatalf("expected no badge for same user on another account id, got %q", got)
	}
}
