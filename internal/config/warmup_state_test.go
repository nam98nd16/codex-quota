package config

import (
	"testing"
	"time"
)

func TestWarmupStateRoundTrip(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("CQ_CONFIG_HOME", tmp)

	resetAt := time.Now().Add(time.Hour).UTC().Truncate(time.Second)
	warmedAt := time.Now().UTC().Truncate(time.Second)
	initial := WarmupState{Entries: map[string]WarmupEntry{
		"account:one": {ResetAt: resetAt, WarmedAt: warmedAt},
	}}

	if err := SaveWarmupState(initial); err != nil {
		t.Fatalf("save warmup state: %v", err)
	}
	loaded, err := LoadWarmupState()
	if err != nil {
		t.Fatalf("load warmup state: %v", err)
	}

	entry, ok := loaded.Entries["account:one"]
	if !ok {
		t.Fatalf("missing warmup entry")
	}
	if !entry.ResetAt.Equal(resetAt) || !entry.WarmedAt.Equal(warmedAt) {
		t.Fatalf("entry = %+v, want reset=%s warmed=%s", entry, resetAt, warmedAt)
	}
}

func TestLoadWarmupStateMissingFileReturnsEmptyEntries(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("CQ_CONFIG_HOME", tmp)

	loaded, err := LoadWarmupState()
	if err != nil {
		t.Fatalf("load warmup state: %v", err)
	}
	if loaded.Entries == nil || len(loaded.Entries) != 0 {
		t.Fatalf("entries = %#v, want empty map", loaded.Entries)
	}
}
