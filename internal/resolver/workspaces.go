package resolver

import "github.com/kryptamine/herdr-auto-title/internal/state"

// Workspaces uses the tab's sources but keeps the project and omits numbering.
// Zero width leaves fitting to Herdr rather than imposing the tab's width.
type Workspaces struct {
	chain  *Deterministic
	maxLen int
}

func NewWorkspaces(opts Options, maxLen int) *Workspaces {
	return &Workspaces{chain: Default(opts), maxLen: maxLen}
}

func (w *Workspaces) Resolve(pane *state.PaneState) Decision {
	found := w.chain.collect(pane)

	return Decision{
		Name:       Format(withoutRepetition(found.parts), w.maxLen),
		Confidence: found.confidence,
		Reason:     found.reason,
	}
}
