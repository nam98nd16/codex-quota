package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type managedStore struct {
	Accounts []managedAccount `json:"accounts"`
}

type managedAccount struct {
	Label        string `json:"label,omitempty"`
	Email        string `json:"email,omitempty"`
	UserID       string `json:"user_id,omitempty"`
	AccountID    string `json:"account_id"`
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresAt    int64  `json:"expires_at_ms,omitempty"`
	ClientID     string `json:"client_id,omitempty"`
}

func LoadManagedAccounts() ([]*Account, error) {
	path, err := managedAccountsPath()
	if err != nil {
		return nil, err
	}

	root, err := readJSONMap(path)
	if err != nil {
		if os.IsNotExist(err) {
			return []*Account{}, nil
		}
		return nil, fmt.Errorf("failed to read %s: %w", path, err)
	}

	store := managedStore{}
	if rawAccounts, ok := root["accounts"]; ok {
		store.Accounts, err = decodeManagedAccounts(rawAccounts)
		if err != nil {
			return nil, fmt.Errorf("failed to decode accounts in %s: %w", path, err)
		}
	}

	if migrated, changed := migrateManagedAccounts(store.Accounts); changed {
		store.Accounts = migrated
		// Best-effort persistence for automatic migration; in-memory state continues even if write fails.
		_ = writeJSONMap(path, map[string]any{"accounts": store.Accounts})
	}

	accounts := make([]*Account, 0, len(store.Accounts))
	for _, item := range store.Accounts {
		if strings.TrimSpace(item.AccessToken) == "" {
			continue
		}
		account := &Account{
			Label:        strings.TrimSpace(item.Label),
			Email:        strings.TrimSpace(item.Email),
			UserID:       strings.TrimSpace(item.UserID),
			AccountID:    strings.TrimSpace(item.AccountID),
			AccessToken:  strings.TrimSpace(item.AccessToken),
			RefreshToken: strings.TrimSpace(item.RefreshToken),
			ClientID:     strings.TrimSpace(item.ClientID),
			Source:       SourceManaged,
			FilePath:     path,
			Writable:     true,
		}
		if item.ExpiresAt > 0 {
			account.ExpiresAt = time.UnixMilli(item.ExpiresAt)
		}

		claims := ParseAccessToken(account.AccessToken)
		account.AccountID = CanonicalAccountID(account.AccountID, claims.AccountID)
		if account.UserID == "" {
			account.UserID = normalizeUserID(claims.UserID)
		}
		if account.ClientID == "" {
			account.ClientID = claims.ClientID
		}
		if account.Email == "" {
			account.Email = claims.Email
		}
		if account.ExpiresAt.IsZero() {
			account.ExpiresAt = claims.ExpiresAt
		}

		accounts = append(accounts, account)
	}

	sort.Slice(accounts, func(i, j int) bool {
		leftLabel := strings.ToLower(accounts[i].Label)
		rightLabel := strings.ToLower(accounts[j].Label)
		if leftLabel != rightLabel {
			return leftLabel < rightLabel
		}
		leftAccountID := strings.TrimSpace(accounts[i].AccountID)
		rightAccountID := strings.TrimSpace(accounts[j].AccountID)
		if leftAccountID != rightAccountID {
			return leftAccountID < rightAccountID
		}
		return strings.TrimSpace(accounts[i].UserID) < strings.TrimSpace(accounts[j].UserID)
	})

	return accounts, nil
}

func UpsertManagedAccount(account *Account) error {
	if account == nil {
		return fmt.Errorf("account is nil")
	}
	if strings.TrimSpace(account.AccessToken) == "" {
		return fmt.Errorf("access token is empty")
	}
	claims := ParseAccessToken(account.AccessToken)
	account.AccountID = CanonicalAccountID(account.AccountID, claims.AccountID)
	if account.Email == "" {
		account.Email = claims.Email
	}
	if account.UserID == "" {
		account.UserID = normalizeUserID(claims.UserID)
	}
	if account.ClientID == "" {
		account.ClientID = claims.ClientID
	}
	if account.ExpiresAt.IsZero() && !claims.ExpiresAt.IsZero() {
		account.ExpiresAt = claims.ExpiresAt
	}
	if strings.TrimSpace(account.AccountID) == "" {
		return fmt.Errorf("account_id is missing")
	}

	path, err := managedAccountsPath()
	if err != nil {
		return err
	}

	store := managedStore{}
	root, err := readJSONMap(path)
	if err == nil {
		if rawAccounts, ok := root["accounts"]; ok {
			store.Accounts, err = decodeManagedAccounts(rawAccounts)
			if err != nil {
				return fmt.Errorf("failed to decode accounts in %s: %w", path, err)
			}
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("failed to read %s: %w", path, err)
	}

	item := managedAccount{
		Label:        strings.TrimSpace(account.Label),
		Email:        strings.TrimSpace(account.Email),
		UserID:       strings.TrimSpace(account.UserID),
		AccountID:    strings.TrimSpace(account.AccountID),
		AccessToken:  strings.TrimSpace(account.AccessToken),
		RefreshToken: strings.TrimSpace(account.RefreshToken),
		ClientID:     strings.TrimSpace(account.ClientID),
	}
	if !account.ExpiresAt.IsZero() {
		item.ExpiresAt = account.ExpiresAt.UnixMilli()
	}

	updated := false
	for i := range store.Accounts {
		if managedAccountsMatchByIdentity(store.Accounts[i], item) {
			store.Accounts[i] = mergeManagedAccount(store.Accounts[i], item)
			updated = true
			break
		}
	}
	if !updated {
		store.Accounts = append(store.Accounts, item)
	}

	if err := writeJSONMap(path, map[string]any{"accounts": store.Accounts}); err != nil {
		return err
	}

	return nil
}

func mergeManagedAccount(existing, incoming managedAccount) managedAccount {
	merged := existing

	if strings.TrimSpace(merged.Label) == "" {
		merged.Label = incoming.Label
	}
	if strings.TrimSpace(merged.Email) == "" {
		merged.Email = incoming.Email
	} else if incomingEmail := strings.TrimSpace(incoming.Email); incomingEmail != "" && normalizeEmail(incomingEmail) != normalizeEmail(merged.Email) {
		previousEmail := merged.Email
		merged.Email = incomingEmail
		if shouldUpdateManagedLabelToEmail(merged, previousEmail) {
			merged.Label = incomingEmail
		}
	}
	if strings.TrimSpace(merged.UserID) == "" {
		merged.UserID = incoming.UserID
	}
	if strings.TrimSpace(merged.ClientID) == "" {
		merged.ClientID = incoming.ClientID
	}
	if strings.TrimSpace(merged.RefreshToken) == "" {
		merged.RefreshToken = incoming.RefreshToken
	}

	if merged.ExpiresAt == 0 {
		merged.ExpiresAt = incoming.ExpiresAt
	}

	if incoming.ExpiresAt > 0 && (merged.ExpiresAt == 0 || incoming.ExpiresAt > merged.ExpiresAt) {
		merged.AccessToken = incoming.AccessToken
		merged.ExpiresAt = incoming.ExpiresAt
		if strings.TrimSpace(incoming.RefreshToken) != "" {
			merged.RefreshToken = incoming.RefreshToken
		}
		if strings.TrimSpace(incoming.ClientID) != "" {
			merged.ClientID = incoming.ClientID
		}
	}

	if strings.TrimSpace(merged.AccessToken) == "" {
		merged.AccessToken = incoming.AccessToken
		if merged.ExpiresAt == 0 {
			merged.ExpiresAt = incoming.ExpiresAt
		}
	}

	return merged
}

func saveManagedAccount(account *Account) error {
	return UpsertManagedAccount(account)
}

func DeleteManagedAccount(accountID string) error {
	accountID = strings.TrimSpace(accountID)
	if accountID == "" {
		return fmt.Errorf("account_id is empty")
	}

	path, err := managedAccountsPath()
	if err != nil {
		return err
	}

	root, err := readJSONMap(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("failed to read %s: %w", path, err)
	}

	store := managedStore{}
	if rawAccounts, ok := root["accounts"]; ok {
		store.Accounts, err = decodeManagedAccounts(rawAccounts)
		if err != nil {
			return fmt.Errorf("failed to decode accounts in %s: %w", path, err)
		}
	}

	filtered := make([]managedAccount, 0, len(store.Accounts))
	for _, item := range store.Accounts {
		if strings.TrimSpace(item.AccountID) == accountID {
			continue
		}
		filtered = append(filtered, item)
	}

	if len(filtered) == len(store.Accounts) {
		return nil
	}

	root["accounts"] = filtered
	return writeJSONMap(path, root)
}

func DeleteManagedAccountByIdentity(account *Account) error {
	if account == nil {
		return fmt.Errorf("account is nil")
	}

	accountID := strings.TrimSpace(account.AccountID)
	email := normalizeEmail(account.Email)
	userID := normalizeUserID(account.UserID)
	if accountID == "" && email == "" && userID == "" {
		return fmt.Errorf("account identity is empty")
	}

	path, err := managedAccountsPath()
	if err != nil {
		return err
	}

	root, err := readJSONMap(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("failed to read %s: %w", path, err)
	}

	store := managedStore{}
	if rawAccounts, ok := root["accounts"]; ok {
		store.Accounts, err = decodeManagedAccounts(rawAccounts)
		if err != nil {
			return fmt.Errorf("failed to decode accounts in %s: %w", path, err)
		}
	}

	filtered := make([]managedAccount, 0, len(store.Accounts))
	removed := false
	for _, item := range store.Accounts {
		if managedAccountMatchesAccount(item, account) {
			removed = true
			continue
		}
		filtered = append(filtered, item)
	}

	if !removed {
		return nil
	}

	root["accounts"] = filtered
	return writeJSONMap(path, root)
}

func ApplyAccountToOpenCode(account *Account) (string, error) {
	if account == nil {
		return "", fmt.Errorf("account is nil")
	}
	paths := opencodeApplyPaths()
	if len(paths) == 0 {
		return "", fmt.Errorf("OpenCode auth path is unknown")
	}

	successPaths := make([]string, 0, len(paths))
	errorsList := make([]string, 0)

	for _, path := range paths {
		root, err := readJSONMap(path)
		if err != nil {
			if os.IsNotExist(err) {
				root = make(map[string]any)
			} else {
				errorsList = append(errorsList, fmt.Sprintf("%s: failed to read: %v", path, err))
				continue
			}
		}

		openai := asMap(root["openai"])
		if openai == nil {
			openai = make(map[string]any)
			root["openai"] = openai
		}

		openai["access"] = account.AccessToken
		if account.RefreshToken != "" {
			openai["refresh"] = account.RefreshToken
		}
		if account.AccountID != "" {
			openai["accountId"] = account.AccountID
		}
		if account.Email != "" {
			openai["email"] = account.Email
		}
		if !account.ExpiresAt.IsZero() {
			openai["expires"] = account.ExpiresAt.UnixMilli()
		}

		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			errorsList = append(errorsList, fmt.Sprintf("%s: failed to ensure directory: %v", path, err))
			continue
		}

		if err := writeJSONMap(path, root); err != nil {
			errorsList = append(errorsList, fmt.Sprintf("%s: failed to write: %v", path, err))
			continue
		}

		successPaths = append(successPaths, path)
	}

	if len(successPaths) == 0 {
		if len(errorsList) > 0 {
			return "", fmt.Errorf("apply to OpenCode failed: %s", strings.Join(errorsList, "; "))
		}
		return "", fmt.Errorf("apply to OpenCode failed: no writable auth path")
	}

	return successPaths[0], nil
}

func ApplyAccountToCodex(account *Account) (string, error) {
	if account == nil {
		return "", fmt.Errorf("account is nil")
	}
	path := codexAuthPath()
	if strings.TrimSpace(path) == "" {
		return "", fmt.Errorf("Codex auth path is unknown")
	}

	root, err := readJSONMap(path)
	if err != nil {
		if os.IsNotExist(err) {
			root = make(map[string]any)
		} else {
			return "", fmt.Errorf("failed to read %s: %w", path, err)
		}
	}

	tokens := asMap(root["tokens"])
	if tokens == nil {
		tokens = make(map[string]any)
		root["tokens"] = tokens
	}

	tokens["access_token"] = account.AccessToken
	if account.RefreshToken != "" {
		tokens["refresh_token"] = account.RefreshToken
	}
	if account.AccountID != "" {
		tokens["account_id"] = account.AccountID
	}
	root["last_refresh"] = time.Now().UTC().Format(time.RFC3339)

	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return "", fmt.Errorf("failed to ensure directory for %s: %w", path, err)
	}

	if err := writeJSONMap(path, root); err != nil {
		return "", err
	}

	return path, nil
}

func DeleteOpenCodeAuthAccount() error {
	paths := opencodeExistingPaths()
	if len(paths) == 0 {
		if len(opencodeAuthPaths()) == 0 {
			return fmt.Errorf("OpenCode auth path is unknown")
		}
		return nil
	}

	errorsList := make([]string, 0)
	for _, path := range paths {
		root, err := readJSONMap(path)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			errorsList = append(errorsList, fmt.Sprintf("%s: failed to read: %v", path, err))
			continue
		}

		openai := asMap(root["openai"])
		if openai == nil {
			continue
		}

		changed := false
		changed = deleteMapKey(openai, "access") || changed
		changed = deleteMapKey(openai, "refresh") || changed
		changed = deleteMapKey(openai, "accountId") || changed
		changed = deleteMapKey(openai, "email") || changed
		changed = deleteMapKey(openai, "expires") || changed
		if !changed {
			continue
		}

		if err := writeJSONMap(path, root); err != nil {
			errorsList = append(errorsList, fmt.Sprintf("%s: failed to write: %v", path, err))
			continue
		}
	}

	if len(errorsList) > 0 {
		return fmt.Errorf("delete from OpenCode failed: %s", strings.Join(errorsList, "; "))
	}

	return nil
}

func opencodeExistingPaths() []string {
	paths := opencodeAuthPaths()
	if len(paths) == 0 {
		return nil
	}

	existing := make([]string, 0, len(paths))
	for _, path := range paths {
		if path == "" {
			continue
		}
		if _, err := os.Stat(path); err == nil {
			existing = append(existing, path)
		}
	}
	return existing
}

func opencodeApplyPaths() []string {
	existing := opencodeExistingPaths()
	if len(existing) > 0 {
		return existing
	}

	allPaths := opencodeAuthPaths()
	if len(allPaths) > 0 {
		return []string{allPaths[0]}
	}

	path := opencodeAuthPath()
	if strings.TrimSpace(path) != "" {
		return []string{path}
	}
	return nil
}

func DeleteCodexAuthAccount() error {
	path := codexAuthPath()
	if strings.TrimSpace(path) == "" {
		return fmt.Errorf("Codex auth path is unknown")
	}

	root, err := readJSONMap(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("failed to read %s: %w", path, err)
	}

	tokens := asMap(root["tokens"])
	if tokens == nil {
		return nil
	}

	changed := false
	changed = deleteMapKey(tokens, "access_token") || changed
	changed = deleteMapKey(tokens, "refresh_token") || changed
	changed = deleteMapKey(tokens, "account_id") || changed
	if !changed {
		return nil
	}

	return writeJSONMap(path, root)
}

func deleteMapKey(values map[string]any, key string) bool {
	if values == nil {
		return false
	}
	if _, ok := values[key]; !ok {
		return false
	}
	delete(values, key)
	return true
}

func managedAccountsPath() (string, error) {
	dir, err := codexQuotaConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "accounts.json"), nil
}

func decodeManagedAccounts(raw any) ([]managedAccount, error) {
	data, err := json.Marshal(raw)
	if err != nil {
		return nil, err
	}

	accounts := make([]managedAccount, 0)
	if err := json.Unmarshal(data, &accounts); err != nil {
		return nil, err
	}

	return accounts, nil
}

func migrateManagedAccounts(input []managedAccount) ([]managedAccount, bool) {
	if len(input) == 0 {
		return input, false
	}

	byID := make(map[string]managedAccount, len(input))
	order := make([]string, 0, len(input))
	changed := false

	for _, item := range input {
		normalized := strings.TrimSpace(item.AccountID)
		accessToken := strings.TrimSpace(item.AccessToken)
		claims := ParseAccessToken(accessToken)
		canonicalID := CanonicalAccountID(normalized, claims.AccountID)
		if canonicalID != normalized {
			changed = true
		}
		item.AccountID = canonicalID
		if userID := normalizeUserID(claims.UserID); userID != "" && userID != strings.TrimSpace(item.UserID) {
			item.UserID = userID
			changed = true
		}

		previousEmail := item.Email
		if claims.Email != "" && normalizeEmail(item.Email) != normalizeEmail(claims.Email) {
			item.Email = claims.Email
			changed = true
		}
		if shouldUpdateManagedLabelToEmail(item, previousEmail) {
			item.Label = strings.TrimSpace(item.Email)
			changed = true
		}
		if item.ClientID == "" && claims.ClientID != "" {
			item.ClientID = claims.ClientID
			changed = true
		}
		if item.ExpiresAt == 0 && !claims.ExpiresAt.IsZero() {
			item.ExpiresAt = claims.ExpiresAt.UnixMilli()
			changed = true
		}

		key := ""
		identityKeys := managedAccountIdentityKeys(item)
		if len(identityKeys) > 0 {
			key = identityKeys[0]
		}
		if key == "" {
			key = fmt.Sprintf("__empty__:%d", len(order))
		}

		if existing, ok := byID[key]; ok {
			merged := mergeManagedAccount(existing, item)
			if merged != existing {
				changed = true
			}
			byID[key] = merged
			continue
		}

		byID[key] = item
		order = append(order, key)
	}

	output := make([]managedAccount, 0, len(order))
	for _, key := range order {
		if account, ok := byID[key]; ok {
			output = append(output, account)
		}
	}

	if len(output) != len(input) {
		changed = true
	}

	return output, changed
}

func shouldReplaceManagedLabelWithEmail(item managedAccount) bool {
	email := strings.TrimSpace(item.Email)
	if email == "" {
		return false
	}
	label := strings.TrimSpace(item.Label)
	if label == "" {
		return true
	}
	if strings.EqualFold(label, "n/a") {
		return true
	}
	if strings.HasPrefix(strings.ToLower(label), "auth0|") {
		return true
	}
	if accountID := strings.TrimSpace(item.AccountID); accountID != "" && label == shortAccountID(accountID) {
		return true
	}
	return false
}

func shouldUpdateManagedLabelToEmail(item managedAccount, previousEmail string) bool {
	if shouldReplaceManagedLabelWithEmail(item) {
		return true
	}
	label := normalizeEmail(strings.TrimSpace(item.Label))
	if label == "" {
		return false
	}
	return previousEmail != "" && label == normalizeEmail(previousEmail)
}

func managedAccountIdentityKeys(item managedAccount) []string {
	account := &Account{
		Email:        strings.TrimSpace(item.Email),
		UserID:       strings.TrimSpace(item.UserID),
		AccountID:    strings.TrimSpace(item.AccountID),
		AccessToken:  strings.TrimSpace(item.AccessToken),
		RefreshToken: strings.TrimSpace(item.RefreshToken),
	}
	claims := ParseAccessToken(account.AccessToken)
	account.AccountID = CanonicalAccountID(account.AccountID, claims.AccountID)
	if account.UserID == "" {
		account.UserID = normalizeUserID(claims.UserID)
	}
	if account.Email == "" {
		account.Email = claims.Email
	}
	return AccountIdentityKeys(account)
}

func managedAccountsMatchByIdentity(left, right managedAccount) bool {
	leftKeys := managedAccountIdentityKeys(left)
	rightKeys := managedAccountIdentityKeys(right)
	if len(leftKeys) == 0 || len(rightKeys) == 0 {
		return false
	}
	leftIndex := make(map[string]struct{}, len(leftKeys))
	for _, key := range leftKeys {
		leftIndex[key] = struct{}{}
	}
	for _, key := range rightKeys {
		if _, ok := leftIndex[key]; ok {
			return true
		}
	}
	return false
}

func managedAccountMatchesAccount(item managedAccount, account *Account) bool {
	itemKeys := managedAccountIdentityKeys(item)
	targetKeys := AccountIdentityKeys(account)
	if len(itemKeys) == 0 || len(targetKeys) == 0 {
		return false
	}
	itemIndex := make(map[string]struct{}, len(itemKeys))
	for _, key := range itemKeys {
		itemIndex[key] = struct{}{}
	}
	for _, key := range targetKeys {
		if _, ok := itemIndex[key]; ok {
			return true
		}
	}
	return false
}
