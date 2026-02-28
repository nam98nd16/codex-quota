package ui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/deLiseLINO/codex-quota/internal/api"
	"github.com/deLiseLINO/codex-quota/internal/config"
)

func TestPlanTypeCachePersistsAcrossActiveRefresh(t *testing.T) {
	account := &config.Account{Key: "acc-1", Label: "user@example.com", AccountID: "acc-1", Source: config.SourceManaged}
	m := InitialModel([]*config.Account{account}, map[string][]string{}, map[string][]string{}, false)
	m.LoadingMap = map[string]bool{"acc-1": true}
	m.UsageData = map[string]api.UsageData{}

	updated, _ := m.Update(DataMsg{
		AccountKey: "acc-1",
		Data: api.UsageData{
			PlanType: "pro",
			Windows:  []api.QuotaWindow{{Label: "Weekly usage limit", WindowSec: 604800, LeftPercent: 40}},
		},
	})
	got := updated.(Model)
	if !got.isPaidByKnownPlan("acc-1") {
		t.Fatalf("expected known paid plan after DataMsg")
	}

	refreshed, _ := got.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	afterRefresh := refreshed.(Model)
	if _, ok := afterRefresh.UsageData["acc-1"]; ok {
		t.Fatalf("expected usage data to be cleared for active refresh")
	}
	if !afterRefresh.isPaidByKnownPlan("acc-1") {
		t.Fatalf("expected known plan cache to survive active refresh")
	}
}

func TestKnownPlanTypePrunedForRemovedAccounts(t *testing.T) {
	accounts := []*config.Account{
		{Key: "acc-1", Label: "a@example.com", AccountID: "acc-1", Source: config.SourceManaged},
		{Key: "acc-2", Label: "b@example.com", AccountID: "acc-2", Source: config.SourceManaged},
	}
	m := InitialModel(accounts, map[string][]string{}, map[string][]string{}, false)
	m.PlanTypeByAccount["acc-1"] = "pro"
	m.PlanTypeByAccount["acc-2"] = "free"

	nextAccounts := []*config.Account{{Key: "acc-2", Label: "b@example.com", AccountID: "acc-2", Source: config.SourceManaged}}
	updated, _ := m.Update(AccountsMsg{Accounts: nextAccounts})
	got := updated.(Model)

	if _, ok := got.PlanTypeByAccount["acc-1"]; ok {
		t.Fatalf("expected removed account plan cache to be pruned")
	}
	if _, ok := got.PlanTypeByAccount["acc-2"]; !ok {
		t.Fatalf("expected existing account plan cache to remain")
	}
}
