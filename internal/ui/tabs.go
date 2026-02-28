package ui

import (
	"strings"
)

func (m Model) renderAccountTabs() string {
	accounts := m.Accounts
	activeIndex := m.ActiveAccountIx
	width := m.Width

	if len(accounts) == 0 {
		return ""
	}

	maxVisible := 3
	if width >= 180 {
		maxVisible = 5
	} else if width >= 130 {
		maxVisible = 4
	}

	start := 0
	end := len(accounts)
	if len(accounts) > maxVisible {
		half := maxVisible / 2
		start = activeIndex - half
		if start < 0 {
			start = 0
		}
		end = start + maxVisible
		if end > len(accounts) {
			end = len(accounts)
			start = end - maxVisible
		}
	}

	tabs := make([]string, 0, maxVisible+2)
	if start > 0 {
		tabs = append(tabs, TabInactiveStyle.Render("..."))
	}

	for i := start; i < end; i++ {
		account := accounts[i]
		label := account.Label
		if label == "" {
			label = account.SourceLabel()
		}
		subscribed := m.hasSubscription(account)
		badgesRaw := m.activeSourceBadgesForAccount(account)
		if badgesRaw != "" {
			limit := 28 - (m.activeSourceBadgesDisplayWidth(account) + 1)
			if limit < 4 {
				limit = 4
			}
			label = truncateLabel(label, limit)
		} else {
			label = truncateLabel(label, 28)
		}
		labelText := TabInactiveStyle.Render(label)
		switch {
		case subscribed && i == activeIndex:
			labelText = SubscribedLabelActiveStyle.Render(label)
		case subscribed:
			labelText = SubscribedLabelMutedStyle.Render(label)
		case i == activeIndex:
			labelText = TabActiveStyle.Render(label)
		}

		if badgesRaw != "" {
			badges := m.renderActiveSourceBadges(account, i == activeIndex)
			tabs = append(tabs, badges+" "+labelText)
			continue
		}
		tabs = append(tabs, labelText)
	}

	if end < len(accounts) {
		tabs = append(tabs, TabInactiveStyle.Render("..."))
	}

	return strings.Join(tabs, " • ")
}

func truncateLabel(value string, limit int) string {
	if limit <= 0 {
		return value
	}
	runes := []rune(value)
	if len(runes) <= limit {
		return value
	}
	if limit <= 1 {
		return string(runes[:limit])
	}
	return string(runes[:limit-1]) + "…"
}
