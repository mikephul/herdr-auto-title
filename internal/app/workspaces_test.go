package app

import (
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"testing"

	"github.com/kryptamine/herdr-auto-title/internal/herdr"
)

func renameWorkspaceConfig() Config {
	cfg := testConfig()
	cfg.RenameWorkspaces = true

	return cfg
}

func wantWorkspaceNames(t *testing.T, h *harness, want ...string) {
	t.Helper()

	calls := h.client.WorkspaceRenames()
	got := make([]string, 0, len(calls))

	for _, call := range calls {
		if call.WorkspaceID != theWorkspace {
			t.Errorf("unexpected workspace id: %s", call.WorkspaceID)
		}

		got = append(got, call.Label)
	}

	if !slices.Equal(got, want) {
		t.Errorf("workspace names = %q, want %q", got, want)
	}
}

func TestWorkspaceRenamingFollowsTheActiveTab(t *testing.T) {
	t.Parallel()

	for _, active := range []string{"wE:t1", "", "wE:t9"} {
		t.Run(active, func(t *testing.T) {
			t.Parallel()
			h := startWorkspaces(
				t,
				renameWorkspaceConfig(),
				[]herdr.WorkspaceInfo{
					{WorkspaceID: theWorkspace, Label: "dashboard", ActiveTabID: active},
				},
				[]herdr.TabInfo{
					{TabID: "wE:t1", WorkspaceID: theWorkspace, Label: "1"},
					{TabID: "wE:t2", WorkspaceID: theWorkspace, Label: "2"},
				},
				[]herdr.PaneInfo{
					{
						PaneID:                "wE:p1",
						TabID:                 "wE:t1",
						CWD:                   dashboard,
						TerminalTitleStripped: "Fix login",
					},
					{
						PaneID:                "wE:p2",
						TabID:                 "wE:t2",
						CWD:                   api,
						TerminalTitleStripped: "Fix logout",
					},
				},
			)
			h.polls(3)
			wantWorkspaceNames(t, h, "dashboard › Fix login")
			h.client.SetWorkspaces(herdr.WorkspaceInfo{
				WorkspaceID: theWorkspace, Label: "dashboard › Fix login", ActiveTabID: "wE:t2",
			})
			h.polls(2)
			wantWorkspaceNames(t, h, "dashboard › Fix login", "api › Fix logout")
		})
	}
}

func TestWorkspaceRenamingPreservesManualNames(t *testing.T) {
	t.Parallel()
	h := oneTabWorkspace(t, renameWorkspaceConfig(), "My project",
		herdr.PaneInfo{TerminalTitleStripped: "Fix login"})
	h.polls(2)
	wantWorkspaceNames(t, h)

	other := oneTabWorkspace(t, renameWorkspaceConfig(), "dashboard",
		herdr.PaneInfo{TerminalTitleStripped: "Fix login"})
	other.poll()
	other.client.SetWorkspaces(herdr.WorkspaceInfo{
		WorkspaceID: theWorkspace, Label: "My project", ActiveTabID: "wE:t1",
	})
	other.polls(2)
	wantWorkspaceNames(t, other, "dashboard › Fix login")

	other.client.SetWorkspaces(herdr.WorkspaceInfo{
		WorkspaceID: theWorkspace, Label: "", ActiveTabID: "wE:t1",
	})
	other.poll()
	wantWorkspaceNames(t, other, "dashboard › Fix login", "dashboard › Fix login")
}

func TestWorkspaceRenamingSurvivesARestart(t *testing.T) {
	t.Parallel()

	cfg := renameWorkspaceConfig()
	cfg.ManualPath = filepath.Join(t.TempDir(), "manual.json")
	h := oneTabWorkspace(t, cfg, "dashboard", herdr.PaneInfo{TerminalTitleStripped: "Fix login"})
	h.poll()
	h.client.SetPane(herdr.PaneInfo{
		PaneID: "wE:p1", TabID: "wE:t1", CWD: dashboard, Revision: 2,
		TerminalTitleStripped: "Fix logout",
	})
	h.app = newTestApp(t, cfg)
	h.polls(2)
	wantWorkspaceNames(t, h, "dashboard › Fix login", "dashboard › Fix logout")

	h.client.SetWorkspaces(herdr.WorkspaceInfo{
		WorkspaceID: theWorkspace, Label: "Mine", ActiveTabID: "wE:t1",
	})
	h.poll()
	h.app = newTestApp(t, cfg)
	h.polls(2)
	wantWorkspaceNames(t, h, "dashboard › Fix login", "dashboard › Fix logout")
}

func TestWorkspaceRenameFailuresAreRetried(t *testing.T) {
	t.Parallel()

	for _, err := range []error{
		errors.New("connection refused"),
		&herdr.APIError{Code: herdr.CodeWorkspaceNotFound},
		fmt.Errorf("%w: timed out", herdr.ErrUnanswered),
	} {
		t.Run(err.Error(), func(t *testing.T) {
			t.Parallel()
			h := oneTabWorkspace(t, renameWorkspaceConfig(), "dashboard",
				herdr.PaneInfo{TerminalTitleStripped: "Fix login"})
			h.client.SetWorkspaceRenameError(err)
			h.poll()
			h.client.SetWorkspaceRenameError(nil)
			h.polls(2)
			wantWorkspaceNames(t, h, "dashboard › Fix login")
		})
	}
}

func TestWorkspaceNamesAreOptionalAndBoundOnlyWhenConfigured(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name    string
		enabled bool
		width   int
		want    []string
	}{
		{name: "disabled"},
		{name: "unbounded", enabled: true, want: []string{"dashboard › Fix login"}},
		{name: "bounded", enabled: true, width: 9},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			cfg := topicConfig()
			cfg.RenameWorkspaces = test.enabled
			cfg.WorkspaceMaxLength = test.width
			h := oneTabWorkspace(
				t,
				cfg,
				"dashboard",
				herdr.PaneInfo{TerminalTitleStripped: "Fix login"},
			)
			h.polls(3)
			wantWorkspaceNames(t, h, test.want...)
			wantTopics(t, h, "Fix login")
		})
	}

	empty := startWorkspaces(t, renameWorkspaceConfig(),
		[]herdr.WorkspaceInfo{{WorkspaceID: theWorkspace, Label: "dashboard"}}, nil, nil)
	empty.polls(2)
	wantWorkspaceNames(t, empty)
}

func TestAWorkspaceDefaultCanFollowADifferentShell(t *testing.T) {
	t.Parallel()
	h := startWorkspaces(t, renameWorkspaceConfig(),
		[]herdr.WorkspaceInfo{{WorkspaceID: theWorkspace, Label: "api", ActiveTabID: "wE:t1"}},
		[]herdr.TabInfo{{TabID: "wE:t1", WorkspaceID: theWorkspace, Label: "1"}},
		[]herdr.PaneInfo{
			{
				PaneID:                "wE:p1",
				TabID:                 "wE:t1",
				CWD:                   dashboard,
				Focused:               true,
				TerminalTitleStripped: "Fix login",
			},
			{PaneID: "wE:p2", TabID: "wE:t1", CWD: api},
		})
	h.polls(2)
	wantWorkspaceNames(t, h, "dashboard › Fix login")
}

func TestAWorkspaceAlreadyWearingItsGeneratedNameSurvivesARestart(t *testing.T) {
	t.Parallel()

	cfg := renameWorkspaceConfig()
	cfg.ManualPath = filepath.Join(t.TempDir(), "manual.json")
	h := oneTabWorkspace(
		t,
		cfg,
		"dashboard › Fix login",
		herdr.PaneInfo{TerminalTitleStripped: "Fix login"},
	)
	h.poll()
	h.client.SetPane(herdr.PaneInfo{
		PaneID: "wE:p1", TabID: "wE:t1", CWD: dashboard, Revision: 2,
		TerminalTitleStripped: "Fix logout",
	})
	h.app = newTestApp(t, cfg)
	h.polls(2)
	wantWorkspaceNames(t, h, "dashboard › Fix logout")
}

func TestAClosedWorkspaceDoesNotLeaveAManualLockOnItsID(t *testing.T) {
	t.Parallel()
	h := oneTabWorkspace(
		t,
		renameWorkspaceConfig(),
		"Mine",
		herdr.PaneInfo{TerminalTitleStripped: "Fix login"},
	)
	h.poll()
	h.client.SetWorkspaces()
	h.poll()
	h.client.SetWorkspaces(herdr.WorkspaceInfo{
		WorkspaceID: theWorkspace, Label: "dashboard", ActiveTabID: "wE:t1",
	})
	h.polls(2)
	wantWorkspaceNames(t, h, "dashboard › Fix login")
}

func TestWorkspaceNamesFollowTheForegroundProcess(t *testing.T) {
	t.Parallel()
	h := oneTabWorkspace(t, renameWorkspaceConfig(), "dashboard", herdr.PaneInfo{})
	h.client.SetProcesses("wE:p1", herdr.PaneProcessInfoProcess{
		Name: "npm", Argv: []string{"npm", "run", "build"}, CWD: dashboard,
	})
	h.polls(2)
	wantWorkspaceNames(t, h, "dashboard › npm")
}
