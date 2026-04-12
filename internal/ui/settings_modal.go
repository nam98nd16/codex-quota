package ui

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/deLiseLINO/codex-quota/internal/config"
)

const settingsTimeStepMinutes = 30

var autoRefreshMinuteOptions = []int{1, 2, 5, 10, 15, 20, 30, 45, 60, 90, 120}

type settingsField int

const (
	settingsFieldAutoRefresh settingsField = iota
	settingsFieldAutoSwitchExhausted
	settingsFieldPeakStart
	settingsFieldPeakEnd
	settingsFieldPeakInterval
	settingsFieldOffPeakInterval
	settingsFieldUpdates
	settingsFieldCount
)

func (m *Model) openSettingsOverlay() {
	m.resetHelpState()
	m.resetActionMenuState()
	m.resetDeleteState()
	m.resetApplyState()
	m.ShowInfo = false
	m.Notice = ""
	m.Err = nil
	m.SettingsVisible = true
	m.SettingsDraft = config.NormalizeSettings(m.Settings)
	m.SettingsCursor = 0
}

func (m *Model) resetSettingsState() {
	m.SettingsVisible = false
	m.SettingsCursor = 0
	m.SettingsDraft = config.Settings{}
}

func (m *Model) moveSettingsCursor(delta int) {
	total := int(settingsFieldCount)
	if total == 0 {
		m.SettingsCursor = 0
		return
	}
	m.SettingsCursor = (m.SettingsCursor + delta + total) % total
}

func (m *Model) adjustCurrentSettingsField(delta int, toggle bool) {
	draft := config.NormalizeSettings(m.SettingsDraft)
	switch settingsField(m.SettingsCursor) {
	case settingsFieldAutoRefresh:
		if toggle || delta != 0 {
			draft.AutoRefreshEnabled = !draft.AutoRefreshEnabled
		}
	case settingsFieldAutoSwitchExhausted:
		if toggle || delta != 0 {
			draft.AutoSwitchExhausted = !draft.AutoSwitchExhausted
		}
	case settingsFieldPeakStart:
		stepClockValue(&draft.AutoRefreshPeakStart, delta)
	case settingsFieldPeakEnd:
		stepClockValue(&draft.AutoRefreshPeakEnd, delta)
	case settingsFieldPeakInterval:
		draft.AutoRefreshPeakMinutes = stepMinuteOption(draft.AutoRefreshPeakMinutes, delta)
	case settingsFieldOffPeakInterval:
		draft.AutoRefreshOffPeakMinutes = stepMinuteOption(draft.AutoRefreshOffPeakMinutes, delta)
	case settingsFieldUpdates:
		if toggle || delta != 0 {
			draft.CheckForUpdateOnStartup = !draft.CheckForUpdateOnStartup
		}
	}
	m.SettingsDraft = config.NormalizeSettings(draft)
}

func (m Model) handleSettingsOverlay(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "q", "ctrl+c":
		return m, tea.Quit
	case "esc":
		m.resetSettingsState()
		return m, nil
	case "up", "k":
		m.moveSettingsCursor(-1)
		return m, nil
	case "down", "j":
		m.moveSettingsCursor(1)
		return m, nil
	case "left", "h":
		m.adjustCurrentSettingsField(-1, false)
		return m, nil
	case "right", "l":
		m.adjustCurrentSettingsField(1, false)
		return m, nil
	case "space":
		m.adjustCurrentSettingsField(1, true)
		return m, nil
	case "enter":
		settings := config.NormalizeSettings(m.SettingsDraft)
		m.Settings = settings
		if !settings.AutoRefreshEnabled {
			m.clearAutoRefreshPending()
		}
		m.resetSettingsState()
		return m, tea.Batch(SaveSettingsCmd(settings), m.nextAutoRefreshCmd(time.Now()))
	default:
		return m, nil
	}
}

func (m Model) renderSettingsModal() string {
	draft := config.NormalizeSettings(m.SettingsDraft)
	items := []struct {
		label string
		value string
	}{
		{label: "Auto refresh", value: onOffText(draft.AutoRefreshEnabled)},
		{label: "Auto switch exhausted", value: onOffText(draft.AutoSwitchExhausted)},
		{label: "Peak start", value: draft.AutoRefreshPeakStart},
		{label: "Peak end", value: draft.AutoRefreshPeakEnd},
		{label: "Peak interval", value: formatMinuteValue(draft.AutoRefreshPeakMinutes)},
		{label: "Off-peak interval", value: formatMinuteValue(draft.AutoRefreshOffPeakMinutes)},
		{label: "Startup update check", value: onOffText(draft.CheckForUpdateOnStartup)},
	}

	labelWidth := 0
	for _, item := range items {
		if len(item.label) > labelWidth {
			labelWidth = len(item.label)
		}
	}

	lines := []string{
		ActionMenuTitleStyle.Render("Settings"),
		InfoValueStyle.Render("Timezone for auto refresh: GMT+7 (fixed)"),
		"",
		HelpSectionStyle.Render("Auto refresh schedule"),
	}
	for index, item := range items {
		cursor := " "
		style := ActionMenuItemStyle
		if index == m.SettingsCursor {
			cursor = ">"
			style = ActionMenuSelectedStyle
		}
		line := fmt.Sprintf("%s %d. %-*s %s", cursor, index+1, labelWidth, item.label, item.value)
		lines = append(lines, style.Render(line))
	}
	lines = append(lines, "")
	lines = append(lines, InfoValueStyle.Render("Peak window uses the peak interval; all other times use off-peak."))
	lines = append(lines, InfoValueStyle.Render("Auto switch exhausted watches the active account and switches/applies when quota is spent."))
	lines = append(lines, ActionMenuHintStyle.Render("[↑/↓] Move   [←/→] Adjust   [space] Toggle   [enter] Save   [esc] Cancel"))

	return InfoBoxStyle.Copy().Width(actionMenuModalWidth(lines) + 6).Render(strings.Join(lines, "\n"))
}

func stepClockValue(target *string, delta int) {
	if target == nil || delta == 0 {
		return
	}
	minutes, ok := parseClockMinutesUI(*target)
	if !ok {
		minutes = 0
	}
	minutes = (minutes + delta*settingsTimeStepMinutes) % (24 * 60)
	if minutes < 0 {
		minutes += 24 * 60
	}
	*target = fmt.Sprintf("%02d:%02d", minutes/60, minutes%60)
}

func stepMinuteOption(current, delta int) int {
	if delta == 0 {
		return current
	}
	current = config.NormalizeSettings(config.Settings{AutoRefreshPeakMinutes: current}).AutoRefreshPeakMinutes
	index := 0
	for i, option := range autoRefreshMinuteOptions {
		if option >= current {
			index = i
			break
		}
		index = i
	}
	index = (index + delta + len(autoRefreshMinuteOptions)) % len(autoRefreshMinuteOptions)
	return autoRefreshMinuteOptions[index]
}

func formatMinuteValue(minutes int) string {
	return fmt.Sprintf("%dm", minutes)
}

func onOffText(value bool) string {
	if value {
		return "on"
	}
	return "off"
}
