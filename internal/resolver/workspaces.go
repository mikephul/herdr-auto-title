package resolver

import "github.com/kryptamine/herdr-auto-title/internal/state"

// Workspaces uses the tab's sources but keeps the project and omits numbering.
// Zero width leaves fitting to Herdr rather than imposing the tab's width.
type Workspaces struct {
	chain     *Deterministic
	maxLen    int
	PRNumbers bool
}

func NewWorkspaces(opts Options, maxLen int) *Workspaces {
	return &Workspaces{chain: Default(opts), maxLen: maxLen}
}

func (w *Workspaces) Resolve(pane *state.PaneState) Decision {
	found := w.chain.collect(pane)

	parts := withoutRepetition(found.parts)
	if w.PRNumbers {
		activity := Parts{Agent: parts.Agent, Activity: parts.Activity}
		if activity.Activity != "" {
			parts = activity
		}
	}

	name := Format(parts, w.maxLen)
	if w.PRNumbers && pane.PRNumber > 0 {
		name = withPrefix(prPrefix(pane.PRNumber), name, w.maxLen)
	}

	return Decision{
		Name:       name,
		Confidence: found.confidence,
		Reason:     found.reason,
	}
}
