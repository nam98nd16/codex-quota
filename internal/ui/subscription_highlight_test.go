package ui

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"

	"github.com/deLiseLINO/codex-quota/internal/api"
	"github.com/deLiseLINO/codex-quota/internal/config"
)

func forceTrueColor(t *testing.T) {
	t.Helper()
	previous := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.TrueColor)
	t.Cleanup(func() {
		lipgloss.SetColorProfile(previous)
	})
}

func TestHasSubscription_ByPlanType(t *testing.T) {
	account := &config.Account{
		Key:       "acc-1",
		Label:     "user@example.com",
		AccountID: "acc-1",
		Source:    config.SourceManaged,
	}

	m := InitialModel([]*config.Account{account}, map[string][]string{}, map[string][]string{}, false)
	m.PlanTypeByAccount[account.Key] = "free"
	if m.hasSubscription(account) {
		t.Fatalf("expected free plan to be non-subscription")
	}

	m.PlanTypeByAccount[account.Key] = "  Plus "
	if !m.hasSubscription(account) {
		t.Fatalf("expected plus plan to be treated as subscription")
	}

	delete(m.PlanTypeByAccount, account.Key)
	if m.hasSubscription(account) {
		t.Fatalf("expected missing plan data to be non-subscription")
	}
}

func TestRenderAccountTabs_DiffersForSubscription(t *testing.T) {
	forceTrueColor(t)

	account := &config.Account{
		Key:       "acc-1",
		Label:     "user@example.com",
		AccountID: "acc-1",
		Source:    config.SourceManaged,
	}

	m := InitialModel([]*config.Account{account}, map[string][]string{}, map[string][]string{}, false)
	m.PlanTypeByAccount[account.Key] = "free"
	freeView := m.renderAccountTabs()

	m.PlanTypeByAccount[account.Key] = "pro"
	proView := m.renderAccountTabs()

	if freeView == proView {
		t.Fatalf("expected tabs rendering to differ for subscribed account")
	}
}

func TestCompactView_DiffersForSubscriptionByTextHighlight(t *testing.T) {
	forceTrueColor(t)

	account := &config.Account{
		Key:       "acc-1",
		Label:     "user@example.com",
		AccountID: "acc-1",
		Source:    config.SourceManaged,
	}
	m := InitialModel([]*config.Account{account}, map[string][]string{}, map[string][]string{}, true)
	m.Width = 140

	// keep widths in sync as it would be on WindowSizeMsg
	m.defaultProgress.Width = 30
	m.LoadingMap = map[string]bool{}
	m.ErrorsMap = map[string]error{}

	m.UsageData[account.Key] = api.UsageData{
		Windows: []api.QuotaWindow{
			{Label: "Weekly usage limit", WindowSec: 604800, LeftPercent: 40, ResetAt: time.Now().Add(time.Hour)},
		},
	}
	m.PlanTypeByAccount[account.Key] = "free"
	freeView := m.renderCompactView()

	m.UsageData[account.Key] = api.UsageData{
		Windows: []api.QuotaWindow{
			{Label: "Weekly usage limit", WindowSec: 604800, LeftPercent: 40, ResetAt: time.Now().Add(time.Hour)},
		},
	}
	m.PlanTypeByAccount[account.Key] = "team"
	subscribedView := m.renderCompactView()

	if freeView == subscribedView {
		t.Fatalf("expected compact rendering to differ for subscribed account")
	}
}

func TestCompactView_SubscriptionRendersDistinctPercentStyleWithoutMarker(t *testing.T) {
	account := &config.Account{
		Key:       "acc-1",
		Label:     "user@example.com",
		AccountID: "acc-1",
		Source:    config.SourceManaged,
	}

	m := InitialModel([]*config.Account{account}, map[string][]string{}, map[string][]string{}, true)
	m.Width = 140
	m.defaultProgress.Width = 30
	m.LoadingMap = map[string]bool{}
	m.ErrorsMap = map[string]error{}

	m.UsageData[account.Key] = api.UsageData{
		Windows: []api.QuotaWindow{
			{Label: "Weekly usage limit", WindowSec: 604800, LeftPercent: 40, ResetAt: time.Now().Add(time.Hour)},
		},
	}
	m.PlanTypeByAccount[account.Key] = "team"
	subscribedView := m.renderCompactView()
	if !strings.Contains(subscribedView, "40.0%") {
		t.Fatalf("expected percent in compact view for subscribed account")
	}

	m.UsageData[account.Key] = api.UsageData{
		Windows: []api.QuotaWindow{
			{Label: "Weekly usage limit", WindowSec: 604800, LeftPercent: 40, ResetAt: time.Now().Add(time.Hour)},
		},
	}
	m.PlanTypeByAccount[account.Key] = "free"
	freeView := m.renderCompactView()
	if strings.Contains(freeView, "▸") {
		t.Fatalf("did not expect legacy subscription marker for free plan")
	}
	if strings.Contains(subscribedView, "▸") {
		t.Fatalf("did not expect legacy subscription marker for subscribed plan")
	}
}

func TestCompactStatusRow_SubscriptionHasNoLegacyMarkerOnLoadingAndNoQuotaData(t *testing.T) {
	account := &config.Account{
		Key:       "acc-1",
		Label:     "user@example.com",
		AccountID: "acc-1",
		Source:    config.SourceManaged,
	}

	m := InitialModel([]*config.Account{account}, map[string][]string{}, map[string][]string{}, true)
	m.Width = 140
	m.defaultProgress.Width = 30
	m.ErrorsMap = map[string]error{}
	m.UsageData = map[string]api.UsageData{"acc-1": {}}
	m.PlanTypeByAccount = map[string]string{"acc-1": "team"}

	m.LoadingMap = map[string]bool{"acc-1": true}
	loadingView := m.renderCompactView()
	if strings.Contains(loadingView, "▸") {
		t.Fatalf("did not expect legacy marker for loading subscribed account")
	}

	m.LoadingMap = map[string]bool{}
	noQuotaView := m.renderCompactView()
	if strings.Contains(noQuotaView, "▸") {
		t.Fatalf("did not expect legacy marker for no-quota subscribed account")
	}
}
