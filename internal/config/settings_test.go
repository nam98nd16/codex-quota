package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadSettingsMissingFileReturnsDefaults(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("CQ_CONFIG_HOME", dir)

	settings, err := LoadSettings()
	if err != nil {
		t.Fatalf("LoadSettings() error = %v", err)
	}
	if !settings.CheckForUpdateOnStartup {
		t.Fatalf("CheckForUpdateOnStartup = false, want true")
	}
	if !settings.AutoRefreshEnabled {
		t.Fatalf("AutoRefreshEnabled = false, want true")
	}
	if settings.AutoSwitchExhausted {
		t.Fatalf("AutoSwitchExhausted = true, want false")
	}
	if settings.AutoRefreshPeakStart != "08:30" || settings.AutoRefreshPeakEnd != "22:30" {
		t.Fatalf("unexpected default peak range: %q-%q", settings.AutoRefreshPeakStart, settings.AutoRefreshPeakEnd)
	}
	if settings.AutoRefreshPeakMinutes != 5 || settings.AutoRefreshOffPeakMinutes != 30 {
		t.Fatalf("unexpected default refresh minutes: peak=%d offpeak=%d", settings.AutoRefreshPeakMinutes, settings.AutoRefreshOffPeakMinutes)
	}
}

func TestSaveAndLoadSettings(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("CQ_CONFIG_HOME", dir)

	initial := Settings{
		CheckForUpdateOnStartup:   false,
		AutoRefreshEnabled:        true,
		AutoSwitchExhausted:       true,
		AutoRefreshPeakStart:      "09:00",
		AutoRefreshPeakEnd:        "21:00",
		AutoRefreshPeakMinutes:    10,
		AutoRefreshOffPeakMinutes: 45,
	}
	if err := SaveSettings(initial); err != nil {
		t.Fatalf("SaveSettings() error = %v", err)
	}

	loaded, err := LoadSettings()
	if err != nil {
		t.Fatalf("LoadSettings() error = %v", err)
	}
	if loaded.CheckForUpdateOnStartup != initial.CheckForUpdateOnStartup {
		t.Fatalf("CheckForUpdateOnStartup = %v, want %v", loaded.CheckForUpdateOnStartup, initial.CheckForUpdateOnStartup)
	}
	if loaded.AutoRefreshEnabled != initial.AutoRefreshEnabled {
		t.Fatalf("AutoRefreshEnabled = %v, want %v", loaded.AutoRefreshEnabled, initial.AutoRefreshEnabled)
	}
	if loaded.AutoSwitchExhausted != initial.AutoSwitchExhausted {
		t.Fatalf("AutoSwitchExhausted = %v, want %v", loaded.AutoSwitchExhausted, initial.AutoSwitchExhausted)
	}
	if loaded.AutoRefreshPeakStart != initial.AutoRefreshPeakStart || loaded.AutoRefreshPeakEnd != initial.AutoRefreshPeakEnd {
		t.Fatalf("unexpected peak range after load: %q-%q", loaded.AutoRefreshPeakStart, loaded.AutoRefreshPeakEnd)
	}
	if loaded.AutoRefreshPeakMinutes != initial.AutoRefreshPeakMinutes || loaded.AutoRefreshOffPeakMinutes != initial.AutoRefreshOffPeakMinutes {
		t.Fatalf("unexpected refresh minutes after load: peak=%d offpeak=%d", loaded.AutoRefreshPeakMinutes, loaded.AutoRefreshOffPeakMinutes)
	}
}

func TestLoadSettingsNormalizesInvalidAutoRefreshValues(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("CQ_CONFIG_HOME", dir)

	path := filepath.Join(dir, "codex-quota", "settings.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	content := `{"auto_refresh_enabled":true,"auto_refresh_peak_start":"99:99","auto_refresh_peak_end":"bad","auto_refresh_peak_minutes":0,"auto_refresh_off_peak_minutes":-1}`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write settings json: %v", err)
	}

	loaded, err := LoadSettings()
	if err != nil {
		t.Fatalf("LoadSettings() error = %v", err)
	}
	if loaded.AutoRefreshPeakStart != "08:30" || loaded.AutoRefreshPeakEnd != "22:30" {
		t.Fatalf("expected invalid clock values to normalize, got %q-%q", loaded.AutoRefreshPeakStart, loaded.AutoRefreshPeakEnd)
	}
	if loaded.AutoRefreshPeakMinutes != 5 || loaded.AutoRefreshOffPeakMinutes != 30 {
		t.Fatalf("expected invalid minute values to normalize, got peak=%d offpeak=%d", loaded.AutoRefreshPeakMinutes, loaded.AutoRefreshOffPeakMinutes)
	}
}

func TestLoadSettingsInvalidJSONReturnsError(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("CQ_CONFIG_HOME", dir)

	path := filepath.Join(dir, "codex-quota", "settings.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(path, []byte("{"), 0o600); err != nil {
		t.Fatalf("write invalid json: %v", err)
	}

	if _, err := LoadSettings(); err == nil {
		t.Fatalf("LoadSettings() error = nil, want non-nil")
	}
}
