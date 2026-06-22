package ui

import (
	"fmt"
	"strings"
)

func (m Model) renderRateLimitResetModal() string {
	lines := []string{WarningStyle.Render("Rate-limit reset")}
	if account := m.findAccountByKey(m.RateLimitResetAccountKey); account != nil {
		lines = append(lines, InfoValueStyle.Render("Account: "+truncateLabel(m.displayAccountLabel(account), 44)))
	}
	lines = append(lines, "")

	switch m.RateLimitResetStage {
	case rateLimitResetConfirm:
		available, _ := m.activeRateLimitResetCredits()
		lines = append(lines, InfoValueStyle.Render(fmt.Sprintf("Use one of %d available reset credits?", available)))
		lines = append(lines, "")
		lines = append(lines, m.renderRateLimitResetOption(0, "Use reset"))
		lines = append(lines, m.renderRateLimitResetOption(1, "Cancel"))
		lines = append(lines, "")
		lines = append(lines, ActionMenuHintStyle.Render("[↑/↓] Move   [enter] Select   [esc] Cancel"))
	case rateLimitResetRunning:
		lines = append(lines, InfoValueStyle.Render("Resetting usage..."))
		lines = append(lines, "")
		lines = append(lines, ActionMenuHintStyle.Render("Please wait"))
	case rateLimitResetRetry:
		message := strings.TrimSpace(m.RateLimitResetMessage)
		if message == "" {
			message = "Couldn't reset usage. Please try again."
		}
		lines = append(lines, InfoValueStyle.Render(message))
		lines = append(lines, "")
		lines = append(lines, m.renderRateLimitResetOption(0, "Try again"))
		lines = append(lines, m.renderRateLimitResetOption(1, "Close"))
		lines = append(lines, "")
		lines = append(lines, ActionMenuHintStyle.Render("[↑/↓] Move   [enter] Select   [esc] Cancel"))
	case rateLimitResetMessage:
		message := strings.TrimSpace(m.RateLimitResetMessage)
		if message == "" {
			message = "Rate-limit reset finished."
		}
		lines = append(lines, InfoValueStyle.Render(message))
		lines = append(lines, "")
		lines = append(lines, ActionMenuHintStyle.Render("[enter/esc] Close"))
	default:
		lines = append(lines, InfoValueStyle.Render("Rate-limit reset is unavailable."))
	}

	return InfoBoxStyle.Copy().Width(68).Render(strings.Join(lines, "\n"))
}

func (m Model) renderRateLimitResetOption(index int, label string) string {
	cursor := " "
	style := ActionMenuItemStyle
	if m.RateLimitResetCursor == index {
		cursor = ">"
		style = ActionMenuSelectedStyle
	}
	return style.Render(fmt.Sprintf("%s %d. %s", cursor, index+1, label))
}

func (m Model) rateLimitResetFooter() string {
	switch m.RateLimitResetStage {
	case rateLimitResetRunning:
		return "Rate-limit reset: please wait"
	case rateLimitResetMessage:
		return "Rate-limit reset: Enter/Esc Close"
	default:
		return "Rate-limit reset: ↑↓ Move • Enter Select • Esc Cancel"
	}
}
