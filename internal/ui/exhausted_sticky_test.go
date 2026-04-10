package ui

import (
	"testing"
	"time"

	"github.com/deLiseLINO/codex-quota/internal/api"
	"github.com/deLiseLINO/codex-quota/internal/config"
)

func TestInitialModelWithUIStateSeedsAndPrunesExhaustedSticky(t *testing.T) {
	accounts := []*config.Account{
		{Key: "acc-1", Label: "a@example.com", AccountID: "acc-1", Source: config.SourceManaged},
	}

	m := InitialModelWithUIState(
		accounts,
		map[string][]string{},
		map[string][]string{},
		config.UIState{
			CompactMode:          true,
			ExhaustedAccountKeys: []string{"acc-1", "ghost"},
		},
	)

	if !m.ExhaustedSticky["acc-1"] {
		t.Fatalf("expected acc-1 to be restored in exhausted sticky set")
	}
	if m.ExhaustedSticky["ghost"] {
		t.Fatalf("expected unknown account key to be pruned from exhausted sticky set")
	}
}

func TestDataMsgSetsExhaustedStickyWhenLimitReached(t *testing.T) {
	account := &config.Account{Key: "acc-1", Label: "user@example.com", AccountID: "acc-1", Source: config.SourceManaged}
	m := InitialModel([]*config.Account{account}, map[string][]string{}, map[string][]string{}, true)
	m.LoadingMap = map[string]bool{"acc-1": true}

	updated, _ := m.Update(DataMsg{
		AccountKey: "acc-1",
		Data: api.UsageData{
			LimitReached: true,
			Windows: []api.QuotaWindow{
				{Label: "Weekly usage limit", WindowSec: 604800, LeftPercent: 30, ResetAt: time.Now().Add(time.Hour)},
			},
		},
	})
	got := updated.(Model)
	if !got.ExhaustedSticky["acc-1"] {
		t.Fatalf("expected exhausted sticky to be set when limit_reached=true")
	}
}

func TestDataMsgSetsExhaustedStickyAtZeroWeeklyAndUnsetsAfterRecovery(t *testing.T) {
	account := &config.Account{Key: "acc-1", Label: "user@example.com", AccountID: "acc-1", Source: config.SourceManaged}
	m := InitialModel([]*config.Account{account}, map[string][]string{}, map[string][]string{}, true)
	m.LoadingMap = map[string]bool{"acc-1": true}

	firstUpdated, _ := m.Update(DataMsg{
		AccountKey: "acc-1",
		Data: api.UsageData{
			Windows: []api.QuotaWindow{
				{Label: "Weekly usage limit", WindowSec: 604800, LeftPercent: 0, ResetAt: time.Now().Add(time.Hour)},
			},
		},
	})
	afterExhausted := firstUpdated.(Model)
	if !afterExhausted.ExhaustedSticky["acc-1"] {
		t.Fatalf("expected exhausted sticky to be set at weekly 0.0%%")
	}

	secondUpdated, _ := afterExhausted.Update(DataMsg{
		AccountKey: "acc-1",
		Data: api.UsageData{
			Windows: []api.QuotaWindow{
				{Label: "Weekly usage limit", WindowSec: 604800, LeftPercent: 90, ResetAt: time.Now().Add(time.Hour)},
			},
		},
	})
	afterRecoveryData := secondUpdated.(Model)
	if afterRecoveryData.ExhaustedSticky["acc-1"] {
		t.Fatalf("expected exhausted sticky to be cleared after non-exhausted data")
	}
}

func TestDataMsgSetsExhaustedStickyAtZero5HourAndUnsetsAfterRecovery(t *testing.T) {
	account := &config.Account{Key: "acc-1", Label: "user@example.com", AccountID: "acc-1", Source: config.SourceManaged}
	m := InitialModel([]*config.Account{account}, map[string][]string{}, map[string][]string{}, true)
	m.LoadingMap = map[string]bool{"acc-1": true}

	firstUpdated, _ := m.Update(DataMsg{
		AccountKey: "acc-1",
		Data: api.UsageData{
			Windows: []api.QuotaWindow{
				{Label: "Weekly usage limit", WindowSec: 604800, LeftPercent: 70, ResetAt: time.Now().Add(24 * time.Hour)},
				{Label: "5 hour usage limit", WindowSec: 18000, LeftPercent: 0, ResetAt: time.Now().Add(time.Hour)},
			},
		},
	})
	afterExhausted := firstUpdated.(Model)
	if !afterExhausted.ExhaustedSticky["acc-1"] {
		t.Fatalf("expected exhausted sticky to be set at 5 hour 0.0%%")
	}

	secondUpdated, _ := afterExhausted.Update(DataMsg{
		AccountKey: "acc-1",
		Data: api.UsageData{
			Windows: []api.QuotaWindow{
				{Label: "Weekly usage limit", WindowSec: 604800, LeftPercent: 70, ResetAt: time.Now().Add(24 * time.Hour)},
				{Label: "5 hour usage limit", WindowSec: 18000, LeftPercent: 90, ResetAt: time.Now().Add(time.Hour)},
			},
		},
	})
	afterRecoveryData := secondUpdated.(Model)
	if afterRecoveryData.ExhaustedSticky["acc-1"] {
		t.Fatalf("expected exhausted sticky to be cleared after 5 hour recovery")
	}
}

func TestExhaustedStickyPrunedForRemovedAccounts(t *testing.T) {
	accounts := []*config.Account{
		{Key: "acc-1", Label: "a@example.com", AccountID: "acc-1", Source: config.SourceManaged},
		{Key: "acc-2", Label: "b@example.com", AccountID: "acc-2", Source: config.SourceManaged},
	}
	m := InitialModel(accounts, map[string][]string{}, map[string][]string{}, true)
	m.ExhaustedSticky["acc-1"] = true
	m.ExhaustedSticky["acc-2"] = true

	nextAccounts := []*config.Account{
		{Key: "acc-2", Label: "b@example.com", AccountID: "acc-2", Source: config.SourceManaged},
	}
	updated, _ := m.Update(AccountsMsg{Accounts: nextAccounts})
	got := updated.(Model)

	if got.ExhaustedSticky["acc-1"] {
		t.Fatalf("expected removed account to be pruned from exhausted sticky set")
	}
	if !got.ExhaustedSticky["acc-2"] {
		t.Fatalf("expected existing account to remain in exhausted sticky set")
	}
}
