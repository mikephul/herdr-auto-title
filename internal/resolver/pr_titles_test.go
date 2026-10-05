package resolver

import (
	"testing"

	"github.com/kryptamine/herdr-auto-title/internal/state"
)

func TestPRNumberFollowsTheTabPosition(t *testing.T) {
	t.Parallel()

	tab := state.TabState{
		Position: 2,
		Context:  &state.PaneState{PRNumber: 663},
	}
	inner := fixedResolver{decision: Decision{Name: "trade › Build checkout"}}

	titles := NewNumbered(NewPRTitles(inner, 50), 50)
	if got := titles.Resolve(tab).Name; got != "2 · [#663] trade › Build checkout" {
		t.Errorf("title = %q", got)
	}

	tab.Context.PRNumber = 0
	if got := titles.Resolve(tab).Name; got != "2 · trade › Build checkout" {
		t.Errorf("title without PR = %q", got)
	}
}

func TestPRWorkspaceShowsTheActivityWithoutTheDirectory(t *testing.T) {
	t.Parallel()

	workspaces := NewWorkspaces(Options{Home: "/home/you"}, 0)
	workspaces.PRNumbers = true

	pane := &state.PaneState{
		Dir:           "/home/you/trade",
		TerminalTitle: "Build checkout",
		PRNumber:      663,
	}
	if got := workspaces.Resolve(pane).Name; got != "[#663] Build checkout" {
		t.Errorf("workspace = %q", got)
	}

	pane.PRNumber = 0
	if got := workspaces.Resolve(pane).Name; got != "Build checkout" {
		t.Errorf("workspace without PR = %q", got)
	}
}
