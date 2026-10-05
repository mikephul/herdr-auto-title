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
	reader.query = func(_ context.Context, branch, dir string) (int, error) {
		calls++

		if branch != "feat/login" || dir != "/repo/worktree" {
			t.Errorf("query = %q in %q", branch, dir)
		}

		return 663, nil
	}

	checkout := git.Checkout{CommonDir: "/repo/.git", Branch: "feat/login"}
	if got := reader.Lookup(context.Background(), checkout, "/repo/worktree"); got != 663 {
		t.Fatalf("number = %d, want 663", got)
	}

	if got, fresh := reader.Cached(checkout); got != 663 || !fresh {
		t.Errorf("cache = (%d, %v), want (663, true)", got, fresh)
	}

	if calls != 1 {
		t.Errorf("queries = %d, want 1", calls)
	}

	other := git.Checkout{CommonDir: "/repo/.git", Branch: "feat/other"}
	if _, fresh := reader.Cached(other); fresh {
		t.Error("another branch reused the cached PR")
	}
}

func TestFailedRefreshKeepsTheLastPRNumber(t *testing.T) {
	t.Parallel()

	reader := New()
	checkout := git.Checkout{CommonDir: "/repo/.git", Branch: "feat/login"}
	reader.query = func(context.Context, string, string) (int, error) {
		return 663, nil
	}
	reader.Lookup(context.Background(), checkout, "/repo")

	k, _ := lookupKey(checkout)

	reader.cache[k] = entry{number: 663, expires: time.Now().Add(-time.Second)}
	if got, fresh := reader.Cached(checkout); got != 663 || fresh {
		t.Errorf("expired cache = (%d, %v), want (663, false)", got, fresh)
	}

	reader.query = func(context.Context, string, string) (int, error) {
		return 0, errors.New("offline")
	}
	if got := reader.Lookup(context.Background(), checkout, "/repo"); got != 663 {
		t.Errorf("number after failure = %d, want 663", got)
	}
}
