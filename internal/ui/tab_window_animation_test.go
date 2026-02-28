package ui

import (
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/deLiseLINO/codex-quota/internal/api"
	"github.com/deLiseLINO/codex-quota/internal/config"
)

func TestDataMsgStartsTabWindowAnimationsForActiveAccount(t *testing.T) {
	account := &config.Account{Key: "acc-1", Label: "user@example.com", AccountID: "acc-1", Source: config.SourceManaged}
	m := InitialModel([]*config.Account{account}, map[string][]string{}, map[string][]string{}, false)
	m.LoadingMap = map[string]bool{"acc-1": true}
	m.UsageData = map[string]api.UsageData{}

	updated, _ := m.Update(DataMsg{
		AccountKey: "acc-1",
		Data: api.UsageData{Windows: []api.QuotaWindow{
			{Label: "5 hour usage limit", WindowSec: 18000, LeftPercent: 20},
			{Label: "Weekly usage limit", WindowSec: 604800, LeftPercent: 70},
		}},
	})
	got := updated.(Model)

	if len(got.tabWindowAnimations) != 2 {
		t.Fatalf("expected 2 tab window animations, got %d", len(got.tabWindowAnimations))
	}
	if !got.animationTicking {
		t.Fatalf("expected tab window ticker to be active")
	}
}

func TestTabWindowAnimationAdvancesAndCompletes(t *testing.T) {
	account := &config.Account{Key: "acc-1", Label: "user@example.com", AccountID: "acc-1", Source: config.SourceManaged}
	m := InitialModel([]*config.Account{account}, map[string][]string{}, map[string][]string{}, false)
	m.LoadingMap = map[string]bool{"acc-1": true}

	updated, _ := m.Update(DataMsg{
		AccountKey: "acc-1",
		Data:       api.UsageData{Windows: []api.QuotaWindow{{Label: "Weekly usage limit", WindowSec: 604800, LeftPercent: 80}}},
	})
	got := updated.(Model)

	window := api.QuotaWindow{Label: "Weekly usage limit", WindowSec: 604800}
	key := tabWindowKey("acc-1", window)
	anim, ok := got.tabWindowAnimations[key]
	if !ok {
		t.Fatalf("expected tab window animation for weekly window")
	}

	halfNow := anim.StartedAt.Add(anim.Duration / 2)
	midUpdated, _ := got.Update(AnimationFrameMsg{Now: halfNow})
	mid := midUpdated.(Model)
	midAnim, ok := mid.tabWindowAnimations[key]
	if !ok {
		t.Fatalf("expected tab animation to still exist at half duration")
	}
	if midAnim.Current <= 0 || midAnim.Current >= 0.8 {
		t.Fatalf("expected intermediate tab animation value between 0 and 0.8, got %.3f", midAnim.Current)
	}

	endNow := anim.StartedAt.Add(anim.Duration + time.Millisecond)
	finalUpdated, _ := mid.Update(AnimationFrameMsg{Now: endNow})
	final := finalUpdated.(Model)
	if _, ok := final.tabWindowAnimations[key]; ok {
		t.Fatalf("expected tab animation to finish and be removed")
	}
	if final.animationTicking {
		t.Fatalf("expected tab animation ticker to stop after completion")
	}
}

func TestTabWindowAnimationStartsOnActiveAccountSwitch(t *testing.T) {
	accounts := []*config.Account{
		{Key: "acc-1", Label: "a@example.com", AccountID: "acc-1", Source: config.SourceManaged},
		{Key: "acc-2", Label: "b@example.com", AccountID: "acc-2", Source: config.SourceManaged},
	}
	m := InitialModel(accounts, map[string][]string{}, map[string][]string{}, false)
	m.UsageData = map[string]api.UsageData{
		"acc-1": {Windows: []api.QuotaWindow{{Label: "Weekly usage limit", WindowSec: 604800, LeftPercent: 55}}},
		"acc-2": {Windows: []api.QuotaWindow{{Label: "Weekly usage limit", WindowSec: 604800, LeftPercent: 10}}},
	}
	m.Loading = false

	navUpdated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRight})
	nav := navUpdated.(Model)
	if len(nav.tabWindowAnimations) == 0 {
		t.Fatalf("expected tab animations to start on active account switch")
	}
	key := tabWindowKey("acc-2", api.QuotaWindow{Label: "Weekly usage limit", WindowSec: 604800})
	anim, ok := nav.tabWindowAnimations[key]
	if !ok {
		t.Fatalf("expected animation for switched active account window")
	}
	if anim.Duration != tabSwitchAnimationDuration {
		t.Fatalf("expected switch animation duration %v, got %v", tabSwitchAnimationDuration, anim.Duration)
	}
	if anim.From != 0 {
		t.Fatalf("expected switch animation to start from zero, got %.3f", anim.From)
	}
}

func TestTabWindowAnimationDoesNotStartOnSwitchWithoutData(t *testing.T) {
	accounts := []*config.Account{
		{Key: "acc-1", Label: "a@example.com", AccountID: "acc-1", Source: config.SourceManaged},
		{Key: "acc-2", Label: "b@example.com", AccountID: "acc-2", Source: config.SourceManaged},
	}
	m := InitialModel(accounts, map[string][]string{}, map[string][]string{}, false)
	m.UsageData = map[string]api.UsageData{
		"acc-1": {Windows: []api.QuotaWindow{{Label: "Weekly usage limit", WindowSec: 604800, LeftPercent: 55}}},
	}
	m.Loading = false

	navUpdated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRight})
	nav := navUpdated.(Model)
	if len(nav.tabWindowAnimations) != 0 {
		t.Fatalf("did not expect switch animation when target account has no data, got %d animations", len(nav.tabWindowAnimations))
	}
}
