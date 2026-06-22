package ui

import (
	"errors"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"

	"github.com/deLiseLINO/codex-quota/internal/api"
)

func TestActionMenuShowsRateLimitResetOnlyWhenAvailable(t *testing.T) {
	m := testModelForHotkeys(1)
	out := ansi.Strip(m.renderActionMenuModal())
	if strings.Contains(out, "Use rate-limit reset") {
		t.Fatalf("did not expect reset action without available credits:\n%s", out)
	}

	available := int64(2)
	m.UsageData["managed:1"] = api.UsageData{AvailableRateLimitResetCredits: &available}
	m.Data = m.UsageData["managed:1"]
	out = ansi.Strip(m.renderActionMenuModal())
	if !strings.Contains(out, "Use rate-limit reset") {
		t.Fatalf("expected reset action with available credits:\n%s", out)
	}
}

func TestRateLimitResetFlowConsumesAndRefreshes(t *testing.T) {
	withFetchHooks(t)
	originalID := newRateLimitResetID
	newRateLimitResetID = func() string { return "stable-reset-id" }
	t.Cleanup(func() { newRateLimitResetID = originalID })

	consumeCalls := []string{}
	consumeResetCredit = func(accessToken, accountID, redeemRequestID string) (api.RateLimitResetResult, error) {
		consumeCalls = append(consumeCalls, redeemRequestID)
		return api.RateLimitResetResult{Outcome: api.RateLimitResetOutcomeReset, WindowsReset: 2}, nil
	}

	m := testModelForHotkeys(1)
	available := int64(1)
	m.UsageData["managed:1"] = api.UsageData{AvailableRateLimitResetCredits: &available}
	m.Data = m.UsageData["managed:1"]

	updated, cmd := m.beginRateLimitResetFlow()
	got := updated.(Model)
	if cmd != nil || !got.RateLimitResetVisible || got.RateLimitResetStage != rateLimitResetConfirm {
		t.Fatalf("expected confirmation modal, got visible=%v stage=%q cmd=%v", got.RateLimitResetVisible, got.RateLimitResetStage, cmd)
	}

	got.RateLimitResetCursor = 0
	updated, cmd = got.Update(tea.KeyMsg{Type: tea.KeyEnter})
	got = updated.(Model)
	if cmd == nil || got.RateLimitResetStage != rateLimitResetRunning {
		t.Fatalf("expected consume command and running state, got stage=%q cmd=%v", got.RateLimitResetStage, cmd)
	}

	msg := cmd().(RateLimitResetConsumedMsg)
	updated, refreshCmd := got.Update(msg)
	got = updated.(Model)
	if refreshCmd == nil {
		t.Fatalf("expected quota refresh command after reset")
	}
	if got.RateLimitResetStage != rateLimitResetMessage || !strings.Contains(got.RateLimitResetMessage, "Usage reset") {
		t.Fatalf("expected success message, got stage=%q message=%q", got.RateLimitResetStage, got.RateLimitResetMessage)
	}
	if len(consumeCalls) != 1 || consumeCalls[0] != "stable-reset-id" {
		t.Fatalf("consume calls = %v, want stable id", consumeCalls)
	}
}

func TestRateLimitResetRetryReusesRedeemRequestID(t *testing.T) {
	withFetchHooks(t)
	originalID := newRateLimitResetID
	newRateLimitResetID = func() string { return "stable-retry-id" }
	t.Cleanup(func() { newRateLimitResetID = originalID })

	consumeCalls := []string{}
	consumeResetCredit = func(accessToken, accountID, redeemRequestID string) (api.RateLimitResetResult, error) {
		consumeCalls = append(consumeCalls, redeemRequestID)
		if len(consumeCalls) == 1 {
			return api.RateLimitResetResult{}, errors.New("temporary failure")
		}
		return api.RateLimitResetResult{Outcome: api.RateLimitResetOutcomeNoCredit}, nil
	}

	m := testModelForHotkeys(1)
	available := int64(1)
	m.UsageData["managed:1"] = api.UsageData{AvailableRateLimitResetCredits: &available}
	m.Data = m.UsageData["managed:1"]
	updated, _ := m.beginRateLimitResetFlow()
	got := updated.(Model)
	got.RateLimitResetCursor = 0

	updated, cmd := got.Update(tea.KeyMsg{Type: tea.KeyEnter})
	got = updated.(Model)
	updated, _ = got.Update(cmd().(RateLimitResetConsumedMsg))
	got = updated.(Model)
	if got.RateLimitResetStage != rateLimitResetRetry {
		t.Fatalf("expected retry state, got %q", got.RateLimitResetStage)
	}

	got.RateLimitResetCursor = 0
	updated, cmd = got.Update(tea.KeyMsg{Type: tea.KeyEnter})
	got = updated.(Model)
	if cmd == nil || got.RateLimitResetStage != rateLimitResetRunning {
		t.Fatalf("expected retry consume command, got stage=%q cmd=%v", got.RateLimitResetStage, cmd)
	}

	if len(consumeCalls) != 1 || consumeCalls[0] != "stable-retry-id" {
		t.Fatalf("first consume calls = %v", consumeCalls)
	}
	_ = cmd().(RateLimitResetConsumedMsg)
	if len(consumeCalls) != 2 || consumeCalls[0] != consumeCalls[1] {
		t.Fatalf("consume calls should reuse id, got %v", consumeCalls)
	}
}
