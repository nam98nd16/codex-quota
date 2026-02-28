package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

func (m Model) View() string {
	var s strings.Builder

	s.WriteString(TitleStyle.Render("🚀 Codex Quota Monitor"))
	s.WriteString("\n")

	if len(m.Accounts) > 0 {
		if !m.CompactMode {
			s.WriteString(m.renderAccountTabs())
			s.WriteString("\n\n")
		} else {
			s.WriteString("\n")
		}
	}

	if m.CompactMode {
		s.WriteString(m.renderCompactView())
	} else {
		if m.Loading {
			s.WriteString(m.renderWindowsLoadingSkeleton())
		} else if account := m.activeAccount(); account != nil {
			s.WriteString(m.renderWindowsView())
		} else {
			s.WriteString("\n")
		}
	}

	s.WriteString(HelpStyle.Render("\n[r] refresh • [R] refresh all • [i] info • [n] add • [enter/o] apply • [x] del • [v] view • [↑↓←→] switch • [q] quit"))

	content := s.String()
	contentWidth := lipgloss.Width(content)
	contentHeight := lipgloss.Height(content)

	containerStyle := lipgloss.NewStyle().Padding(1, 2)
	if m.Width > contentWidth+4 && m.Height > contentHeight+2 {
		containerStyle = containerStyle.
			Width(m.Width).
			Height(m.Height).
			Align(lipgloss.Center, lipgloss.Center)
	}

	baseView := containerStyle.Render(content)

	if modal := m.currentOverlayModal(); modal != "" {
		return overlayCenter(baseView, modal, m.Width, m.Height)
	}

	return baseView
}
