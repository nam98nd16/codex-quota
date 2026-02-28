package ui

import (
	"testing"
	"time"

	"github.com/deLiseLINO/codex-quota/internal/api"
	"github.com/deLiseLINO/codex-quota/internal/config"
)

func TestDataMsgStartsCompactBarAnimation(t *testing.T) {
	account := &config.Account{Key: "acc-1", Label: "user@example.com", AccountID: "acc-1", Source: config.SourceManaged}
	m := InitialModel([]*config.Account{account}, map[string][]string{}, map[string][]string{}, true)
	m.LoadingMap = map[string]bool{"acc-1": true}
	m.UsageData = map[string]api.UsageData{}

	updated, _ := m.Update(DataMsg{
		AccountKey: "acc-1",
		Data: api.UsageData{
			Windows: []api.QuotaWindow{{Label: "Weekly usage limit", WindowSec: 604800, LeftPercent: 40}},
		},
	})
	got := updated.(Model)

	anim, ok := got.compactBarAnimations["acc-1"]
	if !ok {
		t.Fatalf("expected compact bar animation to start")
	}
	if anim.From != 0 {
		t.Fatalf("expected animation to start from zero, got %.3f", anim.From)
	}
	if anim.To != 0.4 {
		t.Fatalf("expected animation target 0.4, got %.3f", anim.To)
	}
	if !got.animationTicking {
		t.Fatalf("expected animation ticker to be active")
	}
}

func TestCompactBarAnimationAdvancesAndCompletes(t *testing.T) {
	account := &config.Account{Key: "acc-1", Label: "user@example.com", AccountID: "acc-1", Source: config.SourceManaged}
	m := InitialModel([]*config.Account{account}, map[string][]string{}, map[string][]string{}, true)
	m.LoadingMap = map[string]bool{"acc-1": true}

	updated, _ := m.Update(DataMsg{
		AccountKey: "acc-1",
		Data: api.UsageData{
			Windows: []api.QuotaWindow{{Label: "Weekly usage limit", WindowSec: 604800, LeftPercent: 80}},
		},
	})
	got := updated.(Model)

	anim, ok := got.compactBarAnimations["acc-1"]
	if !ok {
		t.Fatalf("expected compact bar animation to exist")
	}

	halfNow := anim.StartedAt.Add(anim.Duration / 2)
	midUpdated, _ := got.Update(AnimationFrameMsg{Now: halfNow})
	mid := midUpdated.(Model)
	midAnim, ok := mid.compactBarAnimations["acc-1"]
	if !ok {
		t.Fatalf("expected animation to still be active at half duration")
	}
	if midAnim.Current <= 0 || midAnim.Current >= 0.8 {
		t.Fatalf("expected intermediate animated value between 0 and 0.8, got %.3f", midAnim.Current)
	}

	endNow := anim.StartedAt.Add(anim.Duration + time.Millisecond)
	finalUpdated, _ := mid.Update(AnimationFrameMsg{Now: endNow})
	final := finalUpdated.(Model)
	if _, ok := final.compactBarAnimations["acc-1"]; ok {
		t.Fatalf("expected animation to finish and be removed")
	}
	if final.animationTicking {
		t.Fatalf("expected animation ticker to stop after completion")
	}
}
