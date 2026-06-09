package opencodeplugin

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInstallStatusAndUninstall(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("CQ_OPENCODE_PLUGIN_DIR", dir)
	t.Setenv("CQ_CONFIG_HOME", t.TempDir())

	status, err := CheckStatus()
	if err != nil {
		t.Fatalf("CheckStatus() error = %v", err)
	}
	if status.Installed {
		t.Fatalf("Installed = true, want false")
	}

	path, err := Install()
	if err != nil {
		t.Fatalf("Install() error = %v", err)
	}
	if path != filepath.Join(dir, pluginFileName) {
		t.Fatalf("path = %q, want %q", path, filepath.Join(dir, pluginFileName))
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if !strings.Contains(string(data), "CodexQuotaPlugin") || !strings.Contains(string(data), "quota-signal") {
		t.Fatalf("installed plugin source missing expected content")
	}

	status, err = CheckStatus()
	if err != nil {
		t.Fatalf("CheckStatus() after install error = %v", err)
	}
	if !status.Installed {
		t.Fatalf("Installed = false, want true")
	}

	removedPath, removed, err := Uninstall()
	if err != nil {
		t.Fatalf("Uninstall() error = %v", err)
	}
	if removedPath != path || !removed {
		t.Fatalf("Uninstall() = %q, %v; want %q, true", removedPath, removed, path)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("plugin file still exists after uninstall: %v", err)
	}
}

func TestPluginClassifierAcceptsUsageLimit429WithoutAPIErrorName(t *testing.T) {
	if strings.Contains(pluginSource, `signal.error_name === "APIError" && signal.status_code === 429`) {
		t.Fatalf("plugin classifier still requires APIError name for 429 quota signals")
	}
	if !strings.Contains(pluginSource, `signal.status_code === 429`) || !strings.Contains(pluginSource, `"usage limit"`) {
		t.Fatalf("plugin classifier is missing 429 usage-limit detection")
	}
}
