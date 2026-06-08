package auth

import (
	"testing"
	"time"

	"github.com/deLiseLINO/codex-quota/internal/config"
)

func TestTokenRefreshWindows(t *testing.T) {
	now := time.Now()

	nearRequired := &config.Account{ExpiresAt: now.Add(4 * time.Minute)}
	if !IsExpired(nearRequired) {
		t.Fatalf("expected token inside required refresh window to be expired")
	}

	beyondRequired := &config.Account{ExpiresAt: now.Add(10 * time.Minute)}
	if IsExpired(beyondRequired) {
		t.Fatalf("did not expect token outside required refresh window to be expired")
	}

	nearPreferred := &config.Account{ExpiresAt: now.Add(23 * time.Hour)}
	if !ShouldRefreshSoon(nearPreferred) {
		t.Fatalf("expected token inside preferred refresh window to refresh soon")
	}

	beyondPreferred := &config.Account{ExpiresAt: now.Add(25 * time.Hour)}
	if ShouldRefreshSoon(beyondPreferred) {
		t.Fatalf("did not expect token outside preferred refresh window to refresh soon")
	}
}
