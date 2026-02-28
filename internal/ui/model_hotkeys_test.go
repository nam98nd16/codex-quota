package ui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/deLiseLINO/codex-quota/internal/api"
	"github.com/deLiseLINO/codex-quota/internal/config"
)

func TestArrowNavigationWorksInBothModes(t *testing.T) {
	tests := []struct {
		name       string
		compact    bool
		keyType    tea.KeyType
		wantActive int
	}{
		{name: "normal down", compact: false, keyType: tea.KeyDown, wantActive: 1},
		{name: "normal right", compact: false, keyType: tea.KeyRight, wantActive: 1},
		{name: "normal up", compact: false, keyType: tea.KeyUp, wantActive: 2},
		{name: "normal left", compact: false, keyType: tea.KeyLeft, wantActive: 2},
		{name: "compact down", compact: true, keyType: tea.KeyDown, wantActive: 1},
		{name: "compact right", compact: true, keyType: tea.KeyRight, wantActive: 1},
		{name: "compact up", compact: true, keyType: tea.KeyUp, wantActive: 2},
		{name: "compact left", compact: true, keyType: tea.KeyLeft, wantActive: 2},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			m := testModelForHotkeys(3)
			m.CompactMode = tc.compact
			m.ActiveAccountIx = 0

			updated, _ := m.Update(tea.KeyMsg{Type: tc.keyType})
			got := updated.(Model).ActiveAccountIx
			if got != tc.wantActive {
				t.Fatalf("expected active index %d, got %d", tc.wantActive, got)
			}
		})
	}
}

func TestInitialModelUsesPersistedCompactMode(t *testing.T) {
	m := InitialModel([]*config.Account{}, map[string][]string{}, map[string][]string{}, true)
	if !m.CompactMode {
		t.Fatalf("expected compact mode to be initialized from persisted state")
	}
}

func TestEnterOpensApplyFlowOnMainScreen(t *testing.T) {
	m := testModelForHotkeys(2)
	m.ApplyTargetSelect = false
	m.ApplyConfirm = false

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	got := updated.(Model)

	if !got.ApplyTargetSelect {
		t.Fatalf("expected apply target selection to open on enter")
	}
	if got.ApplyConfirm {
		t.Fatalf("did not expect apply confirm to be set immediately")
	}
}

func TestEnterInApplySelectionKeepsModalSemantics(t *testing.T) {
	m := testModelForHotkeys(1)
	m.startApplyFlow()
	m.ApplyTargets = map[config.Source]bool{
		config.SourceCodex:    true,
		config.SourceOpenCode: false,
	}

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	got := updated.(Model)

	if got.ApplyTargetSelect {
		t.Fatalf("expected apply target selection to close on enter")
	}
	if !got.ApplyConfirm {
		t.Fatalf("expected apply confirm step to open on enter")
	}
}

func TestRefreshAllDoesNotSetNoticeModal(t *testing.T) {
	m := testModelForHotkeys(2)
	m.Notice = "old notice"
	m.UsageData["managed:1"] = api.UsageData{Allowed: true}
	m.ErrorsMap["managed:1"] = nil
	m.LoadingMap["managed:1"] = false

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'R'}})
	got := updated.(Model)

	if got.Notice != "" {
		t.Fatalf("expected no notice for refresh-all, got %q", got.Notice)
	}
	if len(got.UsageData) != 0 {
		t.Fatalf("expected usage cache reset, got %d entries", len(got.UsageData))
	}
	if len(got.ErrorsMap) != 0 {
		t.Fatalf("expected errors cache reset, got %d entries", len(got.ErrorsMap))
	}
	if len(got.LoadingMap) != 2 {
		t.Fatalf("expected two loading accounts scheduled, got %d entries", len(got.LoadingMap))
	}
	loadingCount := 0
	for _, isLoading := range got.LoadingMap {
		if isLoading {
			loadingCount++
		}
	}
	if loadingCount != 2 {
		t.Fatalf("expected exactly two loading markers, got %d", loadingCount)
	}
}

func TestRefreshAllLoadsInListOrderNotActivePriority(t *testing.T) {
	m := testModelForHotkeys(4)
	m.ActiveAccountIx = 3 // focus on the last account

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'R'}})
	got := updated.(Model)

	if !got.LoadingMap["managed:1"] || !got.LoadingMap["managed:2"] {
		t.Fatalf("expected first two accounts to be scheduled first, got loading map: %#v", got.LoadingMap)
	}
	if got.LoadingMap["managed:4"] {
		t.Fatalf("did not expect focused last account to be prioritized, got loading map: %#v", got.LoadingMap)
	}
}

func TestCompactArrowNavigationFollowsVisualOrderWithExhaustedBlock(t *testing.T) {
	m := testModelForHotkeys(4)
	m.CompactMode = true
	// visual order in compact should become: 1,3,2,4
	m.ExhaustedSticky["managed:2"] = true
	m.ExhaustedSticky["managed:4"] = true
	m.ActiveAccountIx = 0 // managed:1

	down, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	gotDown := down.(Model)
	if gotDown.ActiveAccountIx != 2 { // managed:3
		t.Fatalf("expected next compact item to be managed:3 (idx=2), got %d", gotDown.ActiveAccountIx)
	}

	down2, _ := gotDown.Update(tea.KeyMsg{Type: tea.KeyDown})
	gotDown2 := down2.(Model)
	if gotDown2.ActiveAccountIx != 1 { // managed:2 (first exhausted)
		t.Fatalf("expected next compact item to be managed:2 (idx=1), got %d", gotDown2.ActiveAccountIx)
	}

	up, _ := gotDown2.Update(tea.KeyMsg{Type: tea.KeyUp})
	gotUp := up.(Model)
	if gotUp.ActiveAccountIx != 2 { // back to managed:3
		t.Fatalf("expected previous compact item to be managed:3 (idx=2), got %d", gotUp.ActiveAccountIx)
	}
}

func testModelForHotkeys(count int) Model {
	accounts := make([]*config.Account, 0, count)
	for i := 0; i < count; i++ {
		accounts = append(accounts, &config.Account{
			Key:       "managed:" + string(rune('1'+i)),
			Label:     "user" + string(rune('1'+i)) + "@example.com",
			Email:     "user" + string(rune('1'+i)) + "@example.com",
			AccountID: "acc-" + string(rune('1'+i)),
			Source:    config.SourceManaged,
			Writable:  true,
		})
	}
	m := InitialModel(accounts, map[string][]string{}, map[string][]string{}, false)
	m.Loading = false
	return m
}
