package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/deLiseLINO/codex-quota/internal/config"
	"github.com/deLiseLINO/codex-quota/internal/ui"
)

func main() {
	loadResult, err := config.LoadAllAccountsWithSources()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load accounts: %v\n", err)
		os.Exit(1)
	}

	uiState, uiStateErr := config.LoadUIState()
	if uiStateErr != nil {
		fmt.Fprintf(os.Stderr, "failed to load ui state: %v\n", uiStateErr)
	}

	p := tea.NewProgram(
		ui.InitialModelWithUIState(
			loadResult.Accounts,
			loadResult.SourcesByAccountID,
			loadResult.ActiveSourcesByIdentity,
			uiState,
		),
		tea.WithAltScreen(),
	)
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
