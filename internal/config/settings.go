package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type Settings struct {
	CheckForUpdateOnStartup   bool   `json:"check_for_update_on_startup"`
	AutoRefreshEnabled        bool   `json:"auto_refresh_enabled"`
	AutoSwitchExhausted       bool   `json:"auto_switch_exhausted"`
	AutoRefreshPeakStart      string `json:"auto_refresh_peak_start"`
	AutoRefreshPeakEnd        string `json:"auto_refresh_peak_end"`
	AutoRefreshPeakMinutes    int    `json:"auto_refresh_peak_minutes"`
	AutoRefreshOffPeakMinutes int    `json:"auto_refresh_off_peak_minutes"`
}

func DefaultSettings() Settings {
	return NormalizeSettings(Settings{
		CheckForUpdateOnStartup:   true,
		AutoRefreshEnabled:        true,
		AutoSwitchExhausted:       false,
		AutoRefreshPeakStart:      "08:30",
		AutoRefreshPeakEnd:        "22:30",
		AutoRefreshPeakMinutes:    5,
		AutoRefreshOffPeakMinutes: 30,
	})
}

func NormalizeSettings(settings Settings) Settings {
	defaults := Settings{
		CheckForUpdateOnStartup:   settings.CheckForUpdateOnStartup,
		AutoRefreshEnabled:        settings.AutoRefreshEnabled,
		AutoSwitchExhausted:       settings.AutoSwitchExhausted,
		AutoRefreshPeakStart:      normalizeClockValue(settings.AutoRefreshPeakStart, "08:30"),
		AutoRefreshPeakEnd:        normalizeClockValue(settings.AutoRefreshPeakEnd, "22:30"),
		AutoRefreshPeakMinutes:    normalizePositiveMinutes(settings.AutoRefreshPeakMinutes, 5),
		AutoRefreshOffPeakMinutes: normalizePositiveMinutes(settings.AutoRefreshOffPeakMinutes, 30),
	}
	return defaults
}

func LoadSettings() (Settings, error) {
	path, err := settingsPath()
	if err != nil {
		return DefaultSettings(), err
	}

	root, err := readJSONMap(path)
	if err != nil {
		if os.IsNotExist(err) {
			return DefaultSettings(), nil
		}
		return DefaultSettings(), fmt.Errorf("failed to read %s: %w", path, err)
	}

	settings := DefaultSettings()
	if check, ok := root["check_for_update_on_startup"].(bool); ok {
		settings.CheckForUpdateOnStartup = check
	}
	if enabled, ok := root["auto_refresh_enabled"].(bool); ok {
		settings.AutoRefreshEnabled = enabled
	}
	if enabled, ok := root["auto_switch_exhausted"].(bool); ok {
		settings.AutoSwitchExhausted = enabled
	}
	if start, ok := root["auto_refresh_peak_start"].(string); ok {
		settings.AutoRefreshPeakStart = start
	}
	if end, ok := root["auto_refresh_peak_end"].(string); ok {
		settings.AutoRefreshPeakEnd = end
	}
	if peakMinutes, ok := intJSONValue(root["auto_refresh_peak_minutes"]); ok {
		settings.AutoRefreshPeakMinutes = peakMinutes
	}
	if offPeakMinutes, ok := intJSONValue(root["auto_refresh_off_peak_minutes"]); ok {
		settings.AutoRefreshOffPeakMinutes = offPeakMinutes
	}

	return NormalizeSettings(settings), nil
}

func SaveSettings(settings Settings) error {
	settings = NormalizeSettings(settings)

	path, err := settingsPath()
	if err != nil {
		return err
	}

	root := map[string]any{
		"check_for_update_on_startup":   settings.CheckForUpdateOnStartup,
		"auto_refresh_enabled":          settings.AutoRefreshEnabled,
		"auto_switch_exhausted":         settings.AutoSwitchExhausted,
		"auto_refresh_peak_start":       settings.AutoRefreshPeakStart,
		"auto_refresh_peak_end":         settings.AutoRefreshPeakEnd,
		"auto_refresh_peak_minutes":     settings.AutoRefreshPeakMinutes,
		"auto_refresh_off_peak_minutes": settings.AutoRefreshOffPeakMinutes,
	}
	if err := writeJSONMap(path, root); err != nil {
		return fmt.Errorf("failed to write %s: %w", path, err)
	}

	return nil
}

func settingsPath() (string, error) {
	dir, err := codexQuotaConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "settings.json"), nil
}

func normalizeClockValue(value, fallback string) string {
	trimmed := strings.TrimSpace(value)
	if _, ok := parseClockMinutes(trimmed); ok {
		return trimmed
	}
	return fallback
}

func normalizePositiveMinutes(value, fallback int) int {
	if value > 0 {
		return value
	}
	return fallback
}

func parseClockMinutes(value string) (int, bool) {
	parts := strings.Split(strings.TrimSpace(value), ":")
	if len(parts) != 2 {
		return 0, false
	}
	hour, err := strconv.Atoi(parts[0])
	if err != nil || hour < 0 || hour > 23 {
		return 0, false
	}
	minute, err := strconv.Atoi(parts[1])
	if err != nil || minute < 0 || minute > 59 {
		return 0, false
	}
	return hour*60 + minute, true
}

func intJSONValue(value any) (int, bool) {
	switch typed := value.(type) {
	case float64:
		return int(typed), true
	case int:
		return typed, true
	default:
		return 0, false
	}
}
