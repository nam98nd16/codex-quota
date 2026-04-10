package ui

import (
	"strings"

	"github.com/deLiseLINO/codex-quota/internal/config"
)

func (m Model) displayAccountLabel(account *config.Account) string {
	label := baseAccountLabel(account)
	if account == nil || strings.TrimSpace(account.AccountID) == "" {
		return label
	}

	normalized := strings.ToLower(strings.TrimSpace(label))
	if normalized == "" {
		return label
	}

	duplicateCount := 0
	for _, candidate := range m.Accounts {
		if candidate == nil {
			continue
		}
		if strings.ToLower(strings.TrimSpace(baseAccountLabel(candidate))) == normalized {
			duplicateCount++
		}
	}
	if duplicateCount < 2 {
		return label
	}

	return label + " (" + shortAccountIDForDisplay(account.AccountID) + ")"
}

func baseAccountLabel(account *config.Account) string {
	if account == nil {
		return ""
	}
	label := strings.TrimSpace(account.Label)
	if label != "" {
		return label
	}
	return account.SourceLabel()
}

func shortAccountIDForDisplay(accountID string) string {
	trimmed := strings.TrimSpace(accountID)
	if trimmed == "" {
		return ""
	}
	if len(trimmed) <= 12 {
		return trimmed
	}
	return trimmed[:6] + "..." + trimmed[len(trimmed)-4:]
}
