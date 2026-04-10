package config

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestLoadAllAccountsWithSources_PopulatesActiveSourcesByIdentity(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("CQ_CONFIG_HOME", filepath.Join(tmp, "cfg"))
	t.Setenv("CODEX_HOME", filepath.Join(tmp, "codex"))
	t.Setenv("OPENCODE_AUTH_PATH", filepath.Join(tmp, "opencode", "auth.json"))

	codexPath := filepath.Join(tmp, "codex", "auth.json")
	if err := os.MkdirAll(filepath.Dir(codexPath), 0o700); err != nil {
		t.Fatalf("mkdir codex dir: %v", err)
	}
	if err := os.WriteFile(codexPath, []byte(`{"tokens":{"access_token":"tok-codex","account_id":"acc-123"}}`), 0o600); err != nil {
		t.Fatalf("write codex auth: %v", err)
	}

	openCodePath := filepath.Join(tmp, "opencode", "auth.json")
	if err := os.MkdirAll(filepath.Dir(openCodePath), 0o700); err != nil {
		t.Fatalf("mkdir opencode dir: %v", err)
	}
	if err := os.WriteFile(openCodePath, []byte(`{"openai":{"access":"tok-open","accountId":"acc-123","email":"User@Example.com"}}`), 0o600); err != nil {
		t.Fatalf("write opencode auth: %v", err)
	}

	result, err := LoadAllAccountsWithSources()
	if err != nil {
		t.Fatalf("load accounts: %v", err)
	}

	gotByCodexAccount := result.ActiveSourcesByIdentity["account:acc-123"]
	wantByCodexAccount := []string{"codex"}
	if !reflect.DeepEqual(gotByCodexAccount, wantByCodexAccount) {
		t.Fatalf("active sources by codex account mismatch: got %v, want %v", gotByCodexAccount, wantByCodexAccount)
	}

	gotByEmailAccount := result.ActiveSourcesByIdentity["email-account:user@example.com|acc-123"]
	wantByEmailAccount := []string{"opencode"}
	if !reflect.DeepEqual(gotByEmailAccount, wantByEmailAccount) {
		t.Fatalf("active sources by email+account mismatch: got %v, want %v", gotByEmailAccount, wantByEmailAccount)
	}
}
