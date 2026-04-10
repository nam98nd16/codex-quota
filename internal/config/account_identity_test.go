package config

import (
	"encoding/base64"
	"encoding/json"
	"testing"
	"time"
)

func TestParseAccessToken_UsesNestedAuthObjectChatGPTAccountID(t *testing.T) {
	uuid := "98609d8a-85fb-4ff8-aee2-9344e68fbe3f"
	exp := time.Now().Add(2 * time.Hour).Unix()

	token := makeTestJWT(t, map[string]any{
		"cid":   "client-123",
		"email": "user@example.com",
		"sub":   "auth0|sub-id",
		"exp":   exp,
		"https://api.openai.com/auth": map[string]any{
			"chatgpt_account_id": uuid,
		},
	})

	claims := ParseAccessToken(token)
	if claims.AccountID != uuid {
		t.Fatalf("expected account id %q, got %q", uuid, claims.AccountID)
	}
	if claims.Email != "user@example.com" {
		t.Fatalf("expected email %q, got %q", "user@example.com", claims.Email)
	}
	if claims.ClientID != "client-123" {
		t.Fatalf("expected client id %q, got %q", "client-123", claims.ClientID)
	}
	if claims.ExpiresAt.IsZero() {
		t.Fatalf("expected non-zero expiration")
	}
}

func TestParseAccessToken_UsesClientIDClaim(t *testing.T) {
	uuid := "98609d8a-85fb-4ff8-aee2-9344e68fbe3f"
	exp := time.Now().Add(2 * time.Hour).Unix()

	token := makeTestJWT(t, map[string]any{
		"client_id": "app_EMoamEEZ73f0CkXaXp7hrann",
		"email":     "user@example.com",
		"sub":       "auth0|sub-id",
		"exp":       exp,
		"https://api.openai.com/auth": map[string]any{
			"chatgpt_account_id": uuid,
		},
	})

	claims := ParseAccessToken(token)
	if claims.ClientID != "app_EMoamEEZ73f0CkXaXp7hrann" {
		t.Fatalf("expected client id %q, got %q", "app_EMoamEEZ73f0CkXaXp7hrann", claims.ClientID)
	}
}

func TestParseAccessToken_UsesNestedProfileEmail(t *testing.T) {
	uuid := "98609d8a-85fb-4ff8-aee2-9344e68fbe3f"
	exp := time.Now().Add(2 * time.Hour).Unix()

	token := makeTestJWT(t, map[string]any{
		"client_id": "app_EMoamEEZ73f0CkXaXp7hrann",
		"sub":       "auth0|sub-id",
		"exp":       exp,
		"https://api.openai.com/auth": map[string]any{
			"chatgpt_account_id": uuid,
			"chatgpt_user_id":    "user-123",
		},
		"https://api.openai.com/profile": map[string]any{
			"email": "ryan@pinescore.com",
		},
	})

	claims := ParseAccessToken(token)
	if claims.Email != "ryan@pinescore.com" {
		t.Fatalf("expected nested profile email %q, got %q", "ryan@pinescore.com", claims.Email)
	}
	if claims.UserID != "user-123" {
		t.Fatalf("expected nested user id %q, got %q", "user-123", claims.UserID)
	}
}

func TestExactActiveIdentityKeys_PrefersUserEmailAndTokenOverSharedAccountID(t *testing.T) {
	account := &Account{
		UserID:       "user-123",
		Email:        "User@example.com",
		AccountID:    "shared-account-id",
		AccessToken:  "access-token",
		RefreshToken: "refresh-token",
	}

	got := ExactActiveIdentityKeys(account)
	want := []string{
		"user-account:user-123|shared-account-id",
		"email-account:user@example.com|shared-account-id",
	}

	if len(got) != len(want) {
		t.Fatalf("expected %d identity keys, got %d: %v", len(want), len(got), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("expected key %d to be %q, got %q", i, want[i], got[i])
		}
	}
}

func TestExactActiveIdentityKeys_FallsBackToAccountID(t *testing.T) {
	account := &Account{AccountID: "shared-account-id"}

	got := ExactActiveIdentityKeys(account)
	if len(got) != 1 || got[0] != "account:shared-account-id" {
		t.Fatalf("expected account fallback key, got %v", got)
	}
}

func TestDedupeAccounts_MergesByAccountIDWhenEmailMissingOnOneSide(t *testing.T) {
	accountID := "98609d8a-85fb-4ff8-aee2-9344e68fbe3f"
	now := time.Now().UTC()

	codex := &Account{
		UserID:      "user-1",
		AccountID:   accountID,
		Email:       "",
		AccessToken: "older-token",
		ExpiresAt:   now,
		Source:      SourceCodex,
		Writable:    true,
		FilePath:    "/tmp/codex-auth.json",
	}
	managed := &Account{
		UserID:       "user-1",
		AccountID:    accountID,
		Email:        "user@example.com",
		Label:        "user@example.com",
		AccessToken:  "newer-token",
		RefreshToken: "refresh-token",
		ExpiresAt:    now.Add(1 * time.Hour),
		Source:       SourceManaged,
		Writable:     true,
		FilePath:     "/tmp/accounts.json",
	}

	deduped := dedupeAccounts([]*Account{codex, managed})
	if len(deduped) != 1 {
		t.Fatalf("expected 1 account after dedupe, got %d", len(deduped))
	}

	got := deduped[0]
	if got.AccountID != accountID {
		t.Fatalf("expected account id %q, got %q", accountID, got.AccountID)
	}
	if got.Email != "user@example.com" {
		t.Fatalf("expected merged email, got %q", got.Email)
	}
	if got.Label != "user@example.com" {
		t.Fatalf("expected merged label, got %q", got.Label)
	}
	if got.Source != SourceManaged {
		t.Fatalf("expected managed source to win priority, got %q", got.Source)
	}
}

func TestFinalizeAccount_ReplacesTechnicalLabelWithEmail(t *testing.T) {
	accountID := "98609d8a-85fb-4ff8-aee2-9344e68fbe3f"

	tests := []struct {
		name  string
		label string
	}{
		{name: "auth0 prefix", label: "auth0|SZltqUcP3OMO8k3f18EOszJr"},
		{name: "short account id", label: shortAccountID(accountID)},
		{name: "n/a", label: "n/a"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			account := &Account{
				Label:     tc.label,
				Email:     "user@example.com",
				AccountID: accountID,
				Source:    SourceManaged,
			}

			finalizeAccount(account)
			if account.Label != "user@example.com" {
				t.Fatalf("expected label replaced with email, got %q", account.Label)
			}
		})
	}
}

func TestMigrateManagedAccounts_CanonicalizesIDAndNormalizesLabel(t *testing.T) {
	uuid := "98609d8a-85fb-4ff8-aee2-9344e68fbe3f"
	token := makeTestJWT(t, map[string]any{
		"sub": "auth0|SZltqUcP3OMO8k3f18EOszJr",
		"https://api.openai.com/auth": map[string]any{
			"chatgpt_account_id": uuid,
		},
		"email": "new@example.com",
		"cid":   "client-xyz",
		"exp":   time.Now().Add(30 * time.Minute).Unix(),
	})

	input := []managedAccount{
		{
			Label:        "auth0|SZltqUcP3OMO8k3f18EOszJr",
			Email:        "",
			AccountID:    "auth0|SZltqUcP3OMO8k3f18EOszJr",
			AccessToken:  token,
			RefreshToken: "refresh-token",
		},
	}

	output, changed := migrateManagedAccounts(input)
	if !changed {
		t.Fatalf("expected changed=true from migration")
	}
	if len(output) != 1 {
		t.Fatalf("expected 1 output account, got %d", len(output))
	}

	got := output[0]
	if got.AccountID != uuid {
		t.Fatalf("expected canonical uuid account id %q, got %q", uuid, got.AccountID)
	}
	if got.Email != "new@example.com" {
		t.Fatalf("expected email from token, got %q", got.Email)
	}
	if got.Label != "new@example.com" {
		t.Fatalf("expected label normalized to email, got %q", got.Label)
	}
	if got.ClientID != "client-xyz" {
		t.Fatalf("expected client id from token, got %q", got.ClientID)
	}
	if got.ExpiresAt == 0 {
		t.Fatalf("expected expires_at to be backfilled")
	}
}

func TestMigrateManagedAccounts_ReplacesStaleEmailAndLabelFromToken(t *testing.T) {
	uuid := "98609d8a-85fb-4ff8-aee2-9344e68fbe3f"
	token := makeTestJWT(t, map[string]any{
		"sub": "auth0|SZltqUcP3OMO8k3f18EOszJr",
		"https://api.openai.com/auth": map[string]any{
			"chatgpt_account_id": uuid,
		},
		"https://api.openai.com/profile": map[string]any{
			"email": "ryan@pinescore.com",
		},
		"client_id": "app_EMoamEEZ73f0CkXaXp7hrann",
		"exp":       time.Now().Add(30 * time.Minute).Unix(),
	})

	input := []managedAccount{
		{
			Label:        "pinescore@outlook.com",
			Email:        "pinescore@outlook.com",
			AccountID:    uuid,
			AccessToken:  token,
			RefreshToken: "refresh-token",
		},
	}

	output, changed := migrateManagedAccounts(input)
	if !changed {
		t.Fatalf("expected changed=true from migration")
	}
	if len(output) != 1 {
		t.Fatalf("expected 1 output account, got %d", len(output))
	}

	got := output[0]
	if got.Email != "ryan@pinescore.com" {
		t.Fatalf("expected migrated email %q, got %q", "ryan@pinescore.com", got.Email)
	}
	if got.Label != "ryan@pinescore.com" {
		t.Fatalf("expected migrated label %q, got %q", "ryan@pinescore.com", got.Label)
	}
}

func TestMergeManagedAccount_ReplacesStaleEmailAndMatchingLabel(t *testing.T) {
	existing := managedAccount{
		Label:        "pinescore@outlook.com",
		Email:        "pinescore@outlook.com",
		UserID:       "user-old",
		AccountID:    "98609d8a-85fb-4ff8-aee2-9344e68fbe3f",
		AccessToken:  "older-token",
		RefreshToken: "old-refresh",
		ExpiresAt:    time.Now().UTC().Add(1 * time.Hour).UnixMilli(),
		ClientID:     "old-client",
	}
	incoming := managedAccount{
		Email:        "ryan@pinescore.com",
		AccountID:    existing.AccountID,
		AccessToken:  "newer-token",
		RefreshToken: "new-refresh",
		ExpiresAt:    time.Now().UTC().Add(2 * time.Hour).UnixMilli(),
		ClientID:     "new-client",
	}

	merged := mergeManagedAccount(existing, incoming)
	if merged.Email != "ryan@pinescore.com" {
		t.Fatalf("expected merged email %q, got %q", "ryan@pinescore.com", merged.Email)
	}
	if merged.Label != "ryan@pinescore.com" {
		t.Fatalf("expected merged label %q, got %q", "ryan@pinescore.com", merged.Label)
	}
	if merged.AccessToken != "newer-token" {
		t.Fatalf("expected newer access token, got %q", merged.AccessToken)
	}
}

func TestDedupeAccounts_KeepsDistinctUsersWhenAccountIDMatches(t *testing.T) {
	accountID := "98609d8a-85fb-4ff8-aee2-9344e68fbe3f"
	deduped := dedupeAccounts([]*Account{
		{
			UserID:    "user-1",
			Email:     "one@example.com",
			AccountID: accountID,
			Source:    SourceManaged,
			Writable:  true,
		},
		{
			UserID:    "user-2",
			Email:     "two@example.com",
			AccountID: accountID,
			Source:    SourceCodex,
			Writable:  true,
		},
	})

	if len(deduped) != 2 {
		t.Fatalf("expected 2 accounts after dedupe, got %d", len(deduped))
	}
}

func TestDedupeAccounts_KeepsSameUserWhenAccountIDDiffers(t *testing.T) {
	deduped := dedupeAccounts([]*Account{
		{
			UserID:    "user-1",
			Email:     "one@example.com",
			AccountID: "account-a",
			Source:    SourceManaged,
			Writable:  true,
		},
		{
			UserID:    "user-1",
			Email:     "one@example.com",
			AccountID: "account-b",
			Source:    SourceCodex,
			Writable:  true,
		},
	})

	if len(deduped) != 2 {
		t.Fatalf("expected 2 accounts after dedupe, got %d", len(deduped))
	}
}

func TestUpsertManagedAccount_AppendsDistinctUserWithSameAccountID(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("CQ_CONFIG_HOME", tmp)

	accountID := "98609d8a-85fb-4ff8-aee2-9344e68fbe3f"
	firstToken := makeTestJWT(t, map[string]any{
		"client_id": "app_EMoamEEZ73f0CkXaXp7hrann",
		"exp":       time.Now().Add(1 * time.Hour).Unix(),
		"https://api.openai.com/auth": map[string]any{
			"chatgpt_account_id": accountID,
			"chatgpt_user_id":    "user-1",
		},
		"https://api.openai.com/profile": map[string]any{
			"email": "one@example.com",
		},
	})
	secondToken := makeTestJWT(t, map[string]any{
		"client_id": "app_EMoamEEZ73f0CkXaXp7hrann",
		"exp":       time.Now().Add(2 * time.Hour).Unix(),
		"https://api.openai.com/auth": map[string]any{
			"chatgpt_account_id": accountID,
			"chatgpt_user_id":    "user-2",
		},
		"https://api.openai.com/profile": map[string]any{
			"email": "two@example.com",
		},
	})

	if err := UpsertManagedAccount(&Account{Email: "one@example.com", AccountID: accountID, AccessToken: firstToken}); err != nil {
		t.Fatalf("upsert first account: %v", err)
	}
	if err := UpsertManagedAccount(&Account{Email: "two@example.com", AccountID: accountID, AccessToken: secondToken}); err != nil {
		t.Fatalf("upsert second account: %v", err)
	}

	accounts, err := LoadManagedAccounts()
	if err != nil {
		t.Fatalf("load managed accounts: %v", err)
	}
	if len(accounts) != 2 {
		t.Fatalf("expected 2 managed accounts, got %d", len(accounts))
	}
}

func TestUpsertManagedAccount_AppendsSameUserWithDifferentAccountID(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("CQ_CONFIG_HOME", tmp)

	firstToken := makeTestJWT(t, map[string]any{
		"client_id": "app_EMoamEEZ73f0CkXaXp7hrann",
		"exp":       time.Now().Add(1 * time.Hour).Unix(),
		"https://api.openai.com/auth": map[string]any{
			"chatgpt_account_id": "account-a",
			"chatgpt_user_id":    "user-1",
		},
		"https://api.openai.com/profile": map[string]any{
			"email": "one@example.com",
		},
	})
	secondToken := makeTestJWT(t, map[string]any{
		"client_id": "app_EMoamEEZ73f0CkXaXp7hrann",
		"exp":       time.Now().Add(2 * time.Hour).Unix(),
		"https://api.openai.com/auth": map[string]any{
			"chatgpt_account_id": "account-b",
			"chatgpt_user_id":    "user-1",
		},
		"https://api.openai.com/profile": map[string]any{
			"email": "one@example.com",
		},
	})

	if err := UpsertManagedAccount(&Account{Email: "one@example.com", AccountID: "account-a", AccessToken: firstToken}); err != nil {
		t.Fatalf("upsert first account: %v", err)
	}
	if err := UpsertManagedAccount(&Account{Email: "one@example.com", AccountID: "account-b", AccessToken: secondToken}); err != nil {
		t.Fatalf("upsert second account: %v", err)
	}

	accounts, err := LoadManagedAccounts()
	if err != nil {
		t.Fatalf("load managed accounts: %v", err)
	}
	if len(accounts) != 2 {
		t.Fatalf("expected 2 managed accounts, got %d", len(accounts))
	}
	if AccountStableKey(accounts[0]) == AccountStableKey(accounts[1]) {
		t.Fatalf("expected distinct stable keys for different account ids, got %q", AccountStableKey(accounts[0]))
	}
}

func makeTestJWT(t *testing.T, claims map[string]any) string {
	t.Helper()

	headerBytes, err := json.Marshal(map[string]any{
		"alg": "none",
		"typ": "JWT",
	})
	if err != nil {
		t.Fatalf("marshal header: %v", err)
	}
	payloadBytes, err := json.Marshal(claims)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}

	return base64.RawURLEncoding.EncodeToString(headerBytes) + "." +
		base64.RawURLEncoding.EncodeToString(payloadBytes) + ".sig"
}
