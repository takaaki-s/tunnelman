package ui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/takaaki-s/tunnelman/internal/daemon"
)

func testModel(tunnels []daemon.TunnelInfo) Model {
	m := NewModel(nil)
	m.tunnels = tunnels
	m.width = 120
	m.height = 40
	return m
}

func TestUpdate_CursorMovement(t *testing.T) {
	tunnels := []daemon.TunnelInfo{
		{ID: "a", Name: "alpha", Status: "stopped"},
		{ID: "b", Name: "beta", Status: "running"},
		{ID: "c", Name: "gamma", Status: "stopped"},
	}
	m := testModel(tunnels)

	// Move down
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	nm := next.(Model)
	if nm.cursor != 1 {
		t.Errorf("expected cursor=1, got %d", nm.cursor)
	}

	// Move up
	next, _ = nm.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("k")})
	nm = next.(Model)
	if nm.cursor != 0 {
		t.Errorf("expected cursor=0, got %d", nm.cursor)
	}
}

func TestUpdate_CursorBounds(t *testing.T) {
	tunnels := []daemon.TunnelInfo{
		{ID: "a", Name: "alpha", Status: "stopped"},
	}
	m := testModel(tunnels)

	// Can't go below 0
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("k")})
	nm := next.(Model)
	if nm.cursor != 0 {
		t.Errorf("expected cursor=0, got %d", nm.cursor)
	}

	// Can't go past last
	next, _ = nm.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	nm = next.(Model)
	if nm.cursor != 0 {
		t.Errorf("expected cursor=0, got %d", nm.cursor)
	}
}

func TestUpdate_EmptyTunnels_NocrashOnActions(t *testing.T) {
	m := testModel(nil)
	keys := []string{"s", "S", "e", "d"}
	for _, k := range keys {
		next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(k)})
		nm := next.(Model)
		if nm.state == viewDelete {
			t.Errorf("key=%q: viewDelete should not activate on empty list", k)
		}
	}
}

func TestUpdate_DeleteTransition(t *testing.T) {
	tunnels := []daemon.TunnelInfo{{ID: "a", Name: "alpha", Status: "stopped"}}
	m := testModel(tunnels)

	// Press 'd' -> viewDelete
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("d")})
	nm := next.(Model)
	if nm.state != viewDelete {
		t.Fatalf("expected viewDelete, got %d", nm.state)
	}

	// Press 'n' -> back to viewList
	next, _ = nm.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("n")})
	nm = next.(Model)
	if nm.state != viewList {
		t.Errorf("expected viewList after 'n', got %d", nm.state)
	}
	if len(nm.tunnels) != 1 {
		t.Errorf("tunnel should not be removed on cancel")
	}
}

func TestUpdate_HelpToggle(t *testing.T) {
	m := testModel(nil)

	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("?")})
	nm := next.(Model)
	if nm.state != viewHelp {
		t.Errorf("expected viewHelp, got %d", nm.state)
	}

	// ? key closes help
	next, _ = nm.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("?")})
	nm = next.(Model)
	if nm.state != viewList {
		t.Errorf("expected viewList after closing help with '?', got %d", nm.state)
	}

	// Any other key also closes help (by design)
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("?")})
	nm = next.(Model)
	next, _ = nm.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	nm = next.(Model)
	if nm.state != viewList {
		t.Errorf("expected viewList after closing help with any key, got %d", nm.state)
	}
}

func TestUpdate_FormTransitions(t *testing.T) {
	m := testModel(nil)

	// Open add form
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("a")})
	nm := next.(Model)
	if nm.state != viewForm {
		t.Fatalf("expected viewForm, got %d", nm.state)
	}

	// Esc closes form
	next, _ = nm.Update(tea.KeyMsg{Type: tea.KeyEsc})
	nm = next.(Model)
	if nm.state != viewList {
		t.Errorf("expected viewList after Esc, got %d", nm.state)
	}
}

func TestUpdate_ProfileTab(t *testing.T) {
	m := testModel(nil)
	m.profiles = []string{"All", "dev", "prod"}

	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	nm := next.(Model)
	if nm.selectedProfile != 1 {
		t.Errorf("expected selectedProfile=1, got %d", nm.selectedProfile)
	}

	// Wrap around
	nm.selectedProfile = 2
	next, _ = nm.Update(tea.KeyMsg{Type: tea.KeyTab})
	nm = next.(Model)
	if nm.selectedProfile != 0 {
		t.Errorf("expected selectedProfile=0 (wrap), got %d", nm.selectedProfile)
	}
}

func TestUpdate_TunnelsLoaded(t *testing.T) {
	m := testModel(nil)
	tunnels := []daemon.TunnelInfo{{ID: "x", Name: "x", Status: "running"}}

	next, _ := m.Update(tunnelsLoadedMsg(tunnels))
	nm := next.(Model)
	if len(nm.tunnels) != 1 {
		t.Errorf("expected 1 tunnel, got %d", len(nm.tunnels))
	}
}

func TestSelectedProfileName(t *testing.T) {
	m := testModel(nil)
	m.profiles = []string{"All", "dev", "prod"}

	m.selectedProfile = 0
	if name := m.selectedProfileName(); name != "" {
		t.Errorf("expected empty for All, got %q", name)
	}

	m.selectedProfile = 1
	if name := m.selectedProfileName(); name != "dev" {
		t.Errorf("expected 'dev', got %q", name)
	}
}
