package ui

import (
	"testing"

	"github.com/deLiseLINO/codex-quota/internal/config"
)

func TestDisplayAccountLabel_AppendsShortAccountIDForDuplicateLabels(t *testing.T) {
	accounts := []*config.Account{
		{Key: "a1", Label: "user@example.com", Email: "user@example.com", UserID: "user-1", AccountID: "12345678-aaaa-bbbb-cccc-1234567890ab", Source: config.SourceManaged},
		{Key: "a2", Label: "user@example.com", Email: "user@example.com", UserID: "user-1", AccountID: "abcdef12-1111-2222-3333-abcdef123456", Source: config.SourceManaged},
	}

	m := InitialModel(accounts, map[string][]string{}, map[string][]string{}, false)

	if got := m.displayAccountLabel(accounts[0]); got != "user@example.com (123456...90ab)" {
		t.Fatalf("unexpected duplicate label rendering: %q", got)
	}
	if got := m.displayAccountLabel(accounts[1]); got != "user@example.com (abcdef...3456)" {
		t.Fatalf("unexpected duplicate label rendering: %q", got)
	}
}

func TestDisplayAccountLabel_LeavesUniqueLabelsUnchanged(t *testing.T) {
	account := &config.Account{Key: "a1", Label: "user@example.com", Email: "user@example.com", AccountID: "12345678-aaaa-bbbb-cccc-1234567890ab", Source: config.SourceManaged}
	m := InitialModel([]*config.Account{account}, map[string][]string{}, map[string][]string{}, false)

	if got := m.displayAccountLabel(account); got != "user@example.com" {
		t.Fatalf("expected unique label unchanged, got %q", got)
	}
}
