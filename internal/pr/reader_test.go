package pr

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/kryptamine/herdr-auto-title/internal/git"
)

func TestLookupRequiresAnExplicitMention(t *testing.T) {
	t.Parallel()

	reader := New()
	reader.query = func(context.Context, int, string) (int, error) {
		t.Fatal("queried GitHub without a chat mention")
		return 0, nil
	}

	checkout := git.Checkout{CommonDir: "/repo/.git", Branch: "feat/login"}
	if got := reader.Lookup(context.Background(), checkout, 0, "/repo"); got != 0 {
		t.Errorf("number = %d, want none", got)
	}

	if got, fresh := reader.Cached(checkout, 0); got != 0 || !fresh {
		t.Errorf("cache = (%d, %v), want (0, true)", got, fresh)
	}
}

func TestLookupCachesByRepositoryAndMention(t *testing.T) {
	t.Parallel()

	reader := New()
	calls := 0
	reader.query = func(_ context.Context, hint int, dir string) (int, error) {
		calls++

		if hint != 663 || dir != "/repo/worktree" {
			t.Errorf("query hint %d in %q", hint, dir)
		}

		return 663, nil
	}

	checkout := git.Checkout{CommonDir: "/repo/.git", Branch: "feat/login"}
	if got := reader.Lookup(context.Background(), checkout, 663, "/repo/worktree"); got != 663 {
		t.Fatalf("number = %d, want 663", got)
	}

	if got, fresh := reader.Cached(checkout, 663); got != 663 || !fresh {
		t.Errorf("cache = (%d, %v), want (663, true)", got, fresh)
	}

	if calls != 1 {
		t.Errorf("queries = %d, want 1", calls)
	}

	if _, fresh := reader.Cached(checkout, 664); fresh {
		t.Error("another mention reused the cached PR")
	}

	if _, fresh := reader.Cached(git.Checkout{CommonDir: "/other/.git"}, 663); fresh {
		t.Error("another repository reused the cached PR")
	}
}

func TestFailedRefreshKeepsTheLastPRNumber(t *testing.T) {
	t.Parallel()

	reader := New()
	checkout := git.Checkout{CommonDir: "/repo/.git"}
	reader.query = func(context.Context, int, string) (int, error) { return 663, nil }
	reader.Lookup(context.Background(), checkout, 663, "/repo")
	k, _ := lookupKey(checkout, 663)

	reader.cache[k] = entry{number: 663, expires: time.Now().Add(-time.Second)}
	if got, fresh := reader.Cached(checkout, 663); got != 663 || fresh {
		t.Errorf("expired cache = (%d, %v), want (663, false)", got, fresh)
	}

	reader.query = func(context.Context, int, string) (int, error) {
		return 0, errors.New("offline")
	}
	if got := reader.Lookup(context.Background(), checkout, 663, "/repo"); got != 663 {
		t.Errorf("number after failure = %d, want 663", got)
	}
}

func TestMentionRequiresHashAndKeepsLatest(t *testing.T) {
	t.Parallel()

	if got := Mention("Resolve PR #649 review comments, then PR #675"); got != 675 {
		t.Fatalf("mention = %d, want 675", got)
	}

	for _, activity := range []string{"PR", "PR 649", "PR #0", "APR #649", "PR #6490x"} {
		if got := Mention(activity); got != 0 {
			t.Errorf("mention in %q = %d, want none", activity, got)
		}
	}
}
