package app

import (
	"context"
	"path/filepath"

	"github.com/kryptamine/herdr-auto-title/internal/herdr"
	"github.com/kryptamine/herdr-auto-title/internal/reads"
	"github.com/kryptamine/herdr-auto-title/internal/state"
)

func (a *App) nameWorkspaces(
	ctx context.Context,
	client herdr.Client,
	poll *reads.Poll,
	snapshot herdr.Snapshot,
	tabs []state.TabState,
) {
	first := firstTabs(snapshot.Tabs)

	contexts := make(map[string]*state.PaneState, len(tabs))
	for _, tab := range tabs {
		contexts[tab.ID] = tab.Context
	}

	for _, workspace := range snapshot.Workspaces {
		if ctx.Err() != nil {
			return
		}

		if a.manual.Workspaces.Locked(workspace.WorkspaceID) {
			continue
		}

		active := workspace.ActiveTabID
		if _, live := contexts[active]; !live {
			active = first[workspace.WorkspaceID]
		}

		pane := contexts[active]
		if pane == nil {
			continue
		}

		poll.Fill(ctx, pane)
		decision := a.workspaces.Resolve(pane)
		a.apply(ctx, client, workspaceLabels, a.manual.Workspaces, state.Sighting{
			ID: workspace.WorkspaceID, Current: workspace.Label, Desired: decision.Name,
			Default: workspaceDefault(workspace, snapshot),
		}, decision)
	}
}

// workspaceDefault uses shell directories, which Herdr's automatic label follows.
// The active foreground process may run elsewhere and cannot identify that label.
func workspaceDefault(workspace herdr.WorkspaceInfo, snapshot herdr.Snapshot) string {
	tabs := make(map[string]bool, len(snapshot.Tabs))
	for _, tab := range snapshot.Tabs {
		if tab.WorkspaceID == workspace.WorkspaceID {
			tabs[tab.TabID] = true
		}
	}

	for _, pane := range snapshot.Panes {
		if tabs[pane.TabID] && pane.CWD != "" &&
			filepath.Base(filepath.Clean(pane.CWD)) == workspace.Label {
			return workspace.Label
		}
	}

	return ""
}
