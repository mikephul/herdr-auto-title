package resolver

import (
	"fmt"

	"github.com/kryptamine/herdr-auto-title/internal/state"
)

// PRTitles puts the active checkout's PR beside the tab position.
type PRTitles struct {
	inner    TitleResolver
	maxWidth int
}

func NewPRTitles(inner TitleResolver, maxWidth int) *PRTitles {
	if maxWidth <= 0 {
		maxWidth = DefaultMaxLength
	}

	return &PRTitles{inner: inner, maxWidth: maxWidth}
}

func (p *PRTitles) Resolve(tab state.TabState) Decision {
	decision := p.inner.Resolve(tab)
	if tab.Context != nil && tab.Context.PRNumber > 0 {
		decision.Name = withPrefix(prPrefix(tab.Context.PRNumber), decision.Name, p.maxWidth)
	}

	return decision
}

func prPrefix(number int) string { return fmt.Sprintf("[#%d] ", number) }
