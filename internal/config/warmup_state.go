package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type WarmupState struct {
	Entries map[string]WarmupEntry `json:"entries"`
}

type WarmupEntry struct {
	ResetAt  time.Time `json:"reset_at"`
	WarmedAt time.Time `json:"warmed_at"`
}

func LoadWarmupState() (WarmupState, error) {
	path, err := warmupStatePath()
	if err != nil {
		return WarmupState{}, err
	}

	root, err := readJSONMap(path)
	if err != nil {
		if os.IsNotExist(err) {
			return WarmupState{Entries: map[string]WarmupEntry{}}, nil
		}
		return WarmupState{}, fmt.Errorf("failed to read %s: %w", path, err)
	}

	state := WarmupState{Entries: map[string]WarmupEntry{}}
	entries := asMap(root["entries"])
	for key, raw := range entries {
		key = strings.TrimSpace(key)
		entryMap := asMap(raw)
		if key == "" || entryMap == nil {
			continue
		}
		entry, err := parseWarmupEntry(path, entryMap)
		if err != nil {
			return WarmupState{}, err
		}
		if !entry.ResetAt.IsZero() {
			state.Entries[key] = entry
		}
	}

	return state, nil
}

func SaveWarmupState(state WarmupState) error {
	path, err := warmupStatePath()
	if err != nil {
		return err
	}

	entries := map[string]any{}
	for key, entry := range state.Entries {
		key = strings.TrimSpace(key)
		if key == "" || entry.ResetAt.IsZero() {
			continue
		}
		value := map[string]any{
			"reset_at":  entry.ResetAt.UTC().Format(time.RFC3339),
			"warmed_at": "",
		}
		if !entry.WarmedAt.IsZero() {
			value["warmed_at"] = entry.WarmedAt.UTC().Format(time.RFC3339)
		}
		entries[key] = value
	}

	root := map[string]any{"entries": entries}
	if err := writeJSONMap(path, root); err != nil {
		return fmt.Errorf("failed to write %s: %w", path, err)
	}

	return nil
}

func WarmupStateKey(account *Account) string {
	if key := AccountStableKey(account); key != "" {
		return key
	}
	if account == nil {
		return ""
	}
	return strings.TrimSpace(account.Key)
}

func parseWarmupEntry(path string, values map[string]any) (WarmupEntry, error) {
	entry := WarmupEntry{}
	if resetAt := strings.TrimSpace(asString(values["reset_at"])); resetAt != "" {
		parsed, err := time.Parse(time.RFC3339, resetAt)
		if err != nil {
			return WarmupEntry{}, fmt.Errorf("failed to parse %s: %w", path, err)
		}
		entry.ResetAt = parsed
	}
	if warmedAt := strings.TrimSpace(asString(values["warmed_at"])); warmedAt != "" {
		parsed, err := time.Parse(time.RFC3339, warmedAt)
		if err != nil {
			return WarmupEntry{}, fmt.Errorf("failed to parse %s: %w", path, err)
		}
		entry.WarmedAt = parsed
	}
	return entry, nil
}

func warmupStatePath() (string, error) {
	dir, err := codexQuotaConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "warmup_state.json"), nil
}
