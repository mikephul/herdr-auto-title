package pr

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/kryptamine/herdr-auto-title/internal/git"
)

func TestLookupCachesByRepositoryAndBranch(t *testing.T) {
	t.Parallel()

	reader := New()
	calls := 0
	reader.query = func(_ context.Context, branch string, hint int, dir string) (int, error) {
		calls++

		if branch != "feat/login" || hint != 0 || dir != "/repo/worktree" {
			t.Errorf("query = %q, hint %d in %q", branch, hint, dir)
		}

		return 663, nil
	}

	checkout := git.Checkout{CommonDir: "/repo/.git", Branch: "feat/login"}
	if got := reader.Lookup(context.Background(), checkout, 0, "/repo/worktree"); got != 663 {
		t.Fatalf("number = %d, want 663", got)
	}

	if got, fresh := reader.Cached(checkout, 0); got != 663 || !fresh {
		t.Errorf("cache = (%d, %v), want (663, true)", got, fresh)
	}

	if calls != 1 {
		t.Errorf("queries = %d, want 1", calls)
	}

	other := git.Checkout{CommonDir: "/repo/.git", Branch: "feat/other"}
	if _, fresh := reader.Cached(other, 0); fresh {
		t.Error("another branch reused the cached PR")
	}
}

func TestFailedRefreshKeepsTheLastPRNumber(t *testing.T) {
	t.Parallel()

	reader := New()
	checkout := git.Checkout{CommonDir: "/repo/.git", Branch: "feat/login"}
	reader.query = func(context.Context, string, int, string) (int, error) {
		return 663, nil
	}
	reader.Lookup(context.Background(), checkout, 0, "/repo")

	k, _ := lookupKey(checkout, 0)

	reader.cache[k] = entry{number: 663, expires: time.Now().Add(-time.Second)}
	if got, fresh := reader.Cached(checkout, 0); got != 663 || fresh {
		t.Errorf("expired cache = (%d, %v), want (663, false)", got, fresh)
	}

	reader.query = func(context.Context, string, int, string) (int, error) {
		return 0, errors.New("offline")
	}
	if got := reader.Lookup(context.Background(), checkout, 0, "/repo"); got != 663 {
		t.Errorf("number after failure = %d, want 663", got)
	}
}

func TestMentionedPRUsesItsOwnCacheKey(t *testing.T) {
	t.Parallel()

	if got := Mention("Resolve PR 649 review comments | trade"); got != 649 {
		t.Fatalf("mention = %d, want 649", got)
	}

	for _, activity := range []string{"PR", "PR 0", "APR 649", "PR 6490x"} {
		if got := Mention(activity); got != 0 {
			t.Errorf("mention in %q = %d, want none", activity, got)
		}
	}

	reader := New()
	reader.query = func(_ context.Context, branch string, hint int, _ string) (int, error) {
		if branch != "main" || hint != 649 {
			t.Errorf("lookup = %q, hint %d", branch, hint)
		}

		return 649, nil
	}

	checkout := git.Checkout{CommonDir: "/repo/.git", Branch: "main"}
	if got := reader.Lookup(context.Background(), checkout, 649, "/repo"); got != 649 {
		t.Errorf("number = %d, want 649", got)
	}

	if _, fresh := reader.Cached(checkout, 0); fresh {
		t.Error("the main branch reused the mentioned PR")
	}
}
