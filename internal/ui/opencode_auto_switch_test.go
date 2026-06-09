package ui

import (
	"testing"
	"time"

	"github.com/deLiseLINO/codex-quota/internal/api"
	"github.com/deLiseLINO/codex-quota/internal/config"
)

func TestAutoSwitchWithPluginInstalledDoesNotUseThreePercentFallback(t *testing.T) {
	m := testAutoSwitchModelWithReplacement()
	m.OpenCodePluginInstalled = true

	updated, _ := m.Update(DataMsg{
		AccountKey: "managed:1",
		Data:       lowFiveHourQuota(2),
		Background: true,
		FetchedAt:  time.Now(),
	})
	got := updated.(Model)

	if got.activeAccountKey() != "managed:1" {
		t.Fatalf("active account = %q, want managed:1", got.activeAccountKey())
	}
}

func TestAutoSwitchWithoutPluginUsesThreePercentFallback(t *testing.T) {
	m := testAutoSwitchModelWithReplacement()
	m.OpenCodePluginInstalled = false

	updated, cmd := m.Update(DataMsg{
		AccountKey: "managed:1",
		Data:       lowFiveHourQuota(2),
		Background: true,
		FetchedAt:  time.Now(),
	})
	got := updated.(Model)

	if got.activeAccountKey() != "managed:2" {
		t.Fatalf("active account = %q, want managed:2", got.activeAccountKey())
	}
	if cmd == nil {
		t.Fatalf("expected fallback switch command")
	}
}

func TestAutoSwitchWithPluginInstalledStillSwitchesOnOpenCodeEvent(t *testing.T) {
	m := testAutoSwitchModelWithReplacement()
	m.OpenCodePluginInstalled = true

	updated, cmd := m.Update(OpenCodeQuotaSignalMsg{ProviderID: "openai", StatusCode: 429, Message: "quota exhausted"})
	got := updated.(Model)

	if got.activeAccountKey() != "managed:2" {
		t.Fatalf("active account = %q, want managed:2", got.activeAccountKey())
	}
	if cmd == nil {
		t.Fatalf("expected event switch command")
	}
}

func TestLegacyOnlyUsesThreePercentFallbackEvenWhenPluginInstalled(t *testing.T) {
	m := testAutoSwitchModelWithReplacement()
	m.OpenCodePluginInstalled = true
	m.Settings.AutoSwitchTrigger = config.AutoSwitchTriggerLegacyOnly

	updated, cmd := m.Update(DataMsg{
		AccountKey: "managed:1",
		Data:       lowFiveHourQuota(2),
		Background: true,
		FetchedAt:  time.Now(),
	})
	got := updated.(Model)

	if got.activeAccountKey() != "managed:2" {
		t.Fatalf("active account = %q, want managed:2", got.activeAccountKey())
	}
	if cmd == nil {
		t.Fatalf("expected legacy fallback switch command")
	}
}

func TestManualSmartSwitchUsesThreePercentEvenWhenPluginInstalled(t *testing.T) {
	m := testAutoSwitchModelWithReplacement()
	m.OpenCodePluginInstalled = true
	m.PendingSmartSwitchManual = true
	m.PendingSmartSwitchKeys = map[string]bool{"managed:1": true}

	updated, cmd := m.Update(DataMsg{
		AccountKey: "managed:1",
		Data:       lowFiveHourQuota(2),
		FetchedAt:  time.Now(),
	})
	got := updated.(Model)

	if got.activeAccountKey() != "managed:2" {
		t.Fatalf("active account = %q, want managed:2", got.activeAccountKey())
	}
	if cmd == nil {
		t.Fatalf("expected manual smart switch command")
	}
}

func testAutoSwitchModelWithReplacement() Model {
	m := testModelForHotkeys(2)
	m.Loading = false
	m.ActiveAccountIx = 0
	m.Settings = config.DefaultSettings()
	m.Settings.AutoSwitchExhausted = true
	m.Settings.AutoSwitchTrigger = config.AutoSwitchTriggerEventFallback
	m.LoadingMap = map[string]bool{}
	m.ErrorsMap = map[string]error{}
	m.BackgroundLoadingMap = map[string]bool{}
	m.BackgroundErrorMap = map[string]bool{}
	m.PlanTypeByAccount = map[string]string{"managed:2": "team"}
	m.UsageData = map[string]api.UsageData{"managed:2": usableWeeklyQuota(64)}
	markAppliedSources(&m, map[config.Source]string{config.SourceCodex: "managed:1", config.SourceOpenCode: "managed:1"})
	return m
}
