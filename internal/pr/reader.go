// Package pr finds an open GitHub pull request for a checked-out branch.
package pr

import (
	"context"
	"encoding/json"
	"os/exec"
	"regexp"
	"strconv"
	"time"

	"github.com/kryptamine/herdr-auto-title/internal/git"
)

const (
	positiveTTL = 5 * time.Minute
	missingTTL  = time.Minute
	failureTTL  = 30 * time.Second
	queryLimit  = 2 * time.Second
)

type key struct {
	repo   string
	branch string
	hint   int
}

type entry struct {
	number  int
	expires time.Time
}

// Reader keeps GitHub lookups out of the half-second poll path until due.
type Reader struct {
	cache map[key]entry
	query func(context.Context, string, int, string) (int, error)
}

func New() *Reader {
	return &Reader{cache: make(map[key]entry), query: githubLookup}
}

var prMention = regexp.MustCompile(`(?i)\bpr\s*#?([1-9][0-9]{0,8})\b`)

// Mention reads an explicitly named PR from an agent's activity, if any.
func Mention(activity string) int {
	match := prMention.FindStringSubmatch(activity)
	if len(match) < 2 {
		return 0
	}

	number, _ := strconv.Atoi(match[1])

	return number
}

func lookupKey(checkout git.Checkout, hint int) (key, bool) {
	return key{repo: checkout.CommonDir, branch: checkout.Branch, hint: hint},
		checkout.CommonDir != "" && (checkout.Branch != "" || hint > 0)
}

// Cached returns the last answer and whether it is fresh. An expired answer
// remains usable while another checkout takes this poll's GitHub lookup.
func (r *Reader) Cached(checkout git.Checkout, hint int) (int, bool) {
	k, valid := lookupKey(checkout, hint)
	if !valid {
		return 0, true
	}

	got, found := r.cache[k]
	if !found {
		return 0, false
	}

	return got.number, time.Now().Before(got.expires)
}

// Lookup asks GitHub about one branch. A failed lookup leaves the title alone
// and is retried soon; a PR is refreshed less often.
func (r *Reader) Lookup(ctx context.Context, checkout git.Checkout, hint int, dir string) int {
	k, valid := lookupKey(checkout, hint)
	if !valid || dir == "" {
		return 0
	}

	queryCtx, cancel := context.WithTimeout(ctx, queryLimit)
	defer cancel()

	number, err := r.query(queryCtx, checkout.Branch, hint, dir)
	if err != nil {
		return r.retryLater(k)
	}

	ttl := missingTTL
	if number > 0 {
		ttl = positiveTTL
	}

	r.cache[k] = entry{number: number, expires: time.Now().Add(ttl)}

	return number
}

func githubLookup(ctx context.Context, branch string, hint int, dir string) (int, error) {
	if hint > 0 {
		return githubPR(ctx, hint, dir)
	}

	//nolint:gosec // Branch is passed as one gh argument, never through a shell.
	cmd := exec.CommandContext(
		ctx,
		"gh",
		"pr",
		"list",
		"--head",
		branch,
		"--state",
		"open",
		"--json",
		"number",
		"--limit",
		"1",
	)
	cmd.Dir = dir

	raw, err := cmd.Output()
	if err != nil {
		return 0, err
	}

	var prs []struct {
		Number int `json:"number"`
	}
	if err := json.Unmarshal(raw, &prs); err != nil {
		return 0, err
	}

	if len(prs) > 0 && prs[0].Number > 0 {
		return prs[0].Number, nil
	}

	return 0, nil
}

func githubPR(ctx context.Context, number int, dir string) (int, error) {
	//nolint:gosec // The checked numeric PR ID is an argument, never shell text.
	cmd := exec.CommandContext(ctx, "gh", "pr", "view",
		strconv.Itoa(number), "--json", "number,state")
	cmd.Dir = dir

	raw, err := cmd.Output()
	if err != nil {
		return 0, err
	}

	var found struct {
		Number int    `json:"number"`
		State  string `json:"state"`
	}
	if err := json.Unmarshal(raw, &found); err != nil {
		return 0, err
	}

	if found.Number == number && found.State == "OPEN" {
		return number, nil
	}

	return 0, nil
}

func (r *Reader) retryLater(k key) int {
	got := r.cache[k]
	got.expires = time.Now().Add(failureTTL)
	r.cache[k] = got

	return got.number
}
