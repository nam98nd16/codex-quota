package ui

import (
	"strings"

	"github.com/charmbracelet/x/ansi"

	"github.com/deLiseLINO/codex-quota/internal/update"
)

const (
	actionMenuApply      = "apply"
	actionMenuRefresh    = "refresh"
	actionMenuRefreshAll = "refresh_all"
	actionMenuWarm       = "warm"
	actionMenuWarmFree   = "warm_free"
	actionMenuWarmAll    = "warm_all"
	actionMenuInfo       = "info"
	actionMenuAdd        = "add"
	actionMenuView       = "view"
	actionMenuDelete     = "delete"
	actionMenuUpdate     = "update"
	actionMenuSettings   = "settings"
)

type actionMenuItem struct {
	ID       string
	Label    string
	Shortcut string
}

type actionMenuSection struct {
	Title string
	Items []actionMenuItem
}

func (m Model) actionMenuSections() []actionMenuSection {
	sections := []actionMenuSection{
		{
			Title: "Current account",
			Items: []actionMenuItem{
				{ID: actionMenuApply, Label: "Apply selected account", Shortcut: "o"},
				{ID: actionMenuRefresh, Label: "Refresh selected quota", Shortcut: "r"},
				{ID: actionMenuInfo, Label: "Account details", Shortcut: "i"},
				{ID: actionMenuDelete, Label: "Delete account", Shortcut: "x"},
			},
		},
		{
			Title: "Global actions",
			Items: []actionMenuItem{
				{ID: actionMenuRefreshAll, Label: "Refresh all", Shortcut: "R"},
				{ID: actionMenuWarm, Label: "Warm selected quota", Shortcut: ""},
				{ID: actionMenuWarmFree, Label: "Warm all free", Shortcut: ""},
				{ID: actionMenuWarmAll, Label: "Warm all", Shortcut: ""},
				{ID: actionMenuAdd, Label: "Add account", Shortcut: "n"},
				{ID: actionMenuView, Label: "Switch view", Shortcut: "v"},
				{ID: actionMenuSettings, Label: "Settings", Shortcut: "t"},
			},
		},
	}
	if strings.TrimSpace(m.UpdatePromptVersion) != "" && update.SupportsAutoUpdate(m.UpdatePromptMethod) {
		sections[1].Items = append(sections[1].Items, actionMenuItem{ID: actionMenuUpdate, Label: "Install update", Shortcut: "u"})
	}
	return sections
}

func (m Model) actionMenuItems() []actionMenuItem {
	sections := m.actionMenuSections()
	total := 0
	for _, section := range sections {
		total += len(section.Items)
	}
	items := make([]actionMenuItem, 0, total)
	for _, section := range sections {
		items = append(items, section.Items...)
	}
	return items
}

func actionMenuLabelWidth(sections []actionMenuSection) int {
	width := 0
	for _, section := range sections {
		for _, item := range section.Items {
			if w := ansi.StringWidth(item.Label); w > width {
				width = w
			}
		}
	}
	return width
}

func actionMenuModalWidth(lines []string) int {
	width := 56
	for _, line := range lines {
		if w := ansi.StringWidth(line) + 2; w > width {
			width = w
		}
	}
	return width
}

func sourceBadgeLegendText(badges string) string {
	parts := []string{}
	if strings.Contains(badges, "C") {
		parts = append(parts, "Codex")
	}
	if strings.Contains(badges, "O") {
		parts = append(parts, "OpenCode")
	}
	return strings.Join(parts, ", ")
}
