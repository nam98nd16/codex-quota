package ui

import (
	"strings"

	"github.com/deLiseLINO/codex-quota/internal/config"
)

func (m Model) activeTargetSourcesForAccount(account *config.Account) map[config.Source]bool {
	if account == nil || len(m.ActiveSourcesByIdentity) == 0 {
		return nil
	}

	seen := make(map[config.Source]bool, 2)
	appendLabels := func(labels []string) {
		for _, label := range labels {
			source, ok := sourceFromLabel(label)
			if !ok {
				continue
			}
			if source == config.SourceCodex || source == config.SourceOpenCode {
				seen[source] = true
			}
		}
	}

	for _, key := range config.ActiveIdentityKeys(account) {
		appendLabels(m.ActiveSourcesByIdentity[key])
	}
	if len(seen) == 0 {
		return nil
	}
	return seen
}

func (m Model) activeSourceBadgesForAccount(account *config.Account) string {
	sources := m.activeTargetSourcesForAccount(account)
	if len(sources) == 0 {
		return ""
	}

	parts := make([]string, 0, 2)
	if sources[config.SourceCodex] {
		parts = append(parts, "C")
	}
	if sources[config.SourceOpenCode] {
		parts = append(parts, "O")
	}
	return strings.Join(parts, "•")
}

func (m Model) hasSubscription(account *config.Account) bool {
	if account == nil || account.Key == "" {
		return false
	}
	return m.isPaidByKnownPlan(account.Key)
}

func (m Model) renderActiveSourceBadges(account *config.Account, isRowActive bool) string {
	raw := m.activeSourceBadgesForAccount(account)
	if raw == "" {
		return ""
	}

	cStyle := SourceCodexBadgeMutedStyle
	oStyle := SourceOpenCodeBadgeMutedStyle
	if isRowActive {
		cStyle = SourceCodexBadgeActiveStyle
		oStyle = SourceOpenCodeBadgeActiveStyle
	}

	var b strings.Builder
	b.WriteString(SourceBadgeBracketStyle.Render("["))
	for _, r := range raw {
		switch r {
		case 'C':
			b.WriteString(cStyle.Render("C"))
		case 'O':
			b.WriteString(oStyle.Render("O"))
		case '•':
			b.WriteString(SourceBadgeSeparatorStyle.Render("•"))
		default:
			b.WriteRune(r)
		}
	}
	b.WriteString(SourceBadgeBracketStyle.Render("]"))
	return b.String()
}

func (m Model) activeSourceBadgesDisplayWidth(account *config.Account) int {
	raw := m.activeSourceBadgesForAccount(account)
	if raw == "" {
		return 0
	}
	return len([]rune(raw)) + 2
}

func (m Model) renderAccountRefreshIndicator(account *config.Account, isRowActive bool) string {
	if account == nil || !m.BackgroundLoadingMap[account.Key] {
		return ""
	}
	style := RefreshIndicatorMutedStyle
	if isRowActive {
		style = RefreshIndicatorActiveStyle
	}
	return style.Render("↻")
}

func (m Model) accountRefreshIndicatorDisplayWidth(account *config.Account) int {
	if account == nil || !m.BackgroundLoadingMap[account.Key] {
		return 0
	}
	return 1
}
