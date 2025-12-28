package tui

import (
	"testing"
	"time"

	"syschecker/internal/collector"
	"syschecker/internal/engine"
	"syschecker/ui/tui/state"

	tea "github.com/charmbracelet/bubbletea"
)

// MockStatsProvider for testing
type MockStatsProvider struct{}

func (m MockStatsProvider) GetRawMetrics() (*collector.RawStats, error) {
	return &collector.RawStats{}, nil
}

func TestMenuNavigation(t *testing.T) {
	provider := MockStatsProvider{}
	model := InitialModel(provider, engine.DefaultConfig())

	// Initial state
	if model.menuCursor != 0 {
		t.Errorf("Expected initial menu cursor 0, got %d", model.menuCursor)
	}
	if model.state.CurrentPage != state.PageMenu {
		t.Errorf("Expected initial page PageMenu, got %v", model.state.CurrentPage)
	}

	// Test Down Navigation
	cmd := tea.KeyMsg{Type: tea.KeyDown, Runes: []rune{}, Alt: false}
	updatedModel, _ := model.Update(cmd)
	m := updatedModel.(*MainModel)

	if m.menuCursor != 1 {
		t.Errorf("Expected menu cursor 1 after Down key, got %d", m.menuCursor)
	}

	// Test Up Navigation
	cmd = tea.KeyMsg{Type: tea.KeyUp, Runes: []rune{}, Alt: false}
	updatedModel, _ = m.Update(cmd)
	m = updatedModel.(*MainModel)

	if m.menuCursor != 0 {
		t.Errorf("Expected menu cursor 0 after Up key, got %d", m.menuCursor)
	}
}

func TestMenuAnimationLogic(t *testing.T) {
	provider := MockStatsProvider{}
	model := InitialModel(provider, engine.DefaultConfig())

	// Move cursor to 1
	model.menuCursor = 1

	// Initial animation cursor should be 0
	if model.animCursor != 0 {
		t.Errorf("Expected initial animCursor 0, got %f", model.animCursor)
	}

	// Simulate a few animation frames
	// The spring physics should move animCursor towards menuCursor (1.0)

	// Frame 1
	animateMsg := AnimateMsg(time.Now())
	updatedModel, _ := model.Update(animateMsg)
	m := updatedModel.(*MainModel)

	if m.animCursor <= 0 {
		t.Errorf("Expected animCursor to increase after animation frame, got %f", m.animCursor)
	}
	if m.animCursor >= 1.0 {
		t.Errorf("Expected animCursor to not reach target immediately, got %f", m.animCursor)
	}

	// Frame 2
	updatedModel, _ = m.Update(animateMsg)
	m = updatedModel.(*MainModel)
	prevCursor := m.animCursor

	// Frame 3
	updatedModel, _ = m.Update(animateMsg)
	m = updatedModel.(*MainModel)

	if m.animCursor <= prevCursor {
		t.Errorf("Expected animCursor to continue increasing, got %f (prev %f)", m.animCursor, prevCursor)
	}
}

func TestPageTransition(t *testing.T) {
	provider := MockStatsProvider{}
	model := InitialModel(provider, engine.DefaultConfig())

	// Select first item (Console)
	model.menuCursor = 0
	cmd := tea.KeyMsg{Type: tea.KeyEnter, Runes: []rune{}, Alt: false}
	updatedModel, _ := model.Update(cmd)
	m := updatedModel.(*MainModel)

	if m.state.CurrentPage != state.PageConsole {
		t.Errorf("Expected page to change to PageConsole, got %v", m.state.CurrentPage)
	}

	// Go Back
	cmd = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'b'}, Alt: false}
	updatedModel, _ = m.Update(cmd)
	m = updatedModel.(*MainModel)

	if m.state.CurrentPage != state.PageMenu {
		t.Errorf("Expected page to change back to PageMenu, got %v", m.state.CurrentPage)
	}
}
