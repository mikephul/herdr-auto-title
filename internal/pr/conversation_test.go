package pr

import (
	"os"
	"path/filepath"
	"testing"
)

const (
	firstSession  = "11111111-1111-4111-8111-111111111111"
	secondSession = "22222222-2222-4222-8222-222222222222"
)

func TestConversationsKeepPRsPerSession(t *testing.T) {
	t.Parallel()

	home := t.TempDir()
	dir := "/repo/trade"
	root := filepath.Join(home, ".factory", "sessions", slug(dir))
	writeConversation(
		t,
		filepath.Join(root, firstSession+".jsonl"),
		`{"type":"message","message":{"role":"user","content":[{"type":"text","text":"Review PR #675"}]}}`+"\n",
	)
	writeConversation(
		t,
		filepath.Join(root, secondSession+".jsonl"),
		`{"type":"message","message":{"role":"user","content":[{"type":"text","text":"Work on the same checkout"}]}}`+"\n",
	)

	reader := NewConversations(home, nil)
	first := Session{Agent: "droid", ID: firstSession}
	second := Session{Agent: "droid", ID: secondSession}

	if got := reader.Mention(first, dir); got != 675 {
		t.Errorf("first session = %d, want 675", got)
	}

	if got := reader.Mention(second, dir); got != 0 {
		t.Errorf("second session = %d, want none", got)
	}

	appendConversation(
		t,
		filepath.Join(root, secondSession+".jsonl"),
		`{"type":"message","message":{"role":"assistant","content":[{"type":"text","text":"Opened PR #677"}]}}`+"\n",
	)

	if got := reader.Mention(second, dir); got != 677 {
		t.Errorf("second session after append = %d, want 677", got)
	}

	if got := reader.Mention(first, dir); got != 675 {
		t.Errorf("first session after other append = %d, want 675", got)
	}

	reader.Retain([]Session{second})

	if _, kept := reader.sessions[first]; kept {
		t.Error("closed session was retained")
	}
}

func TestConversationFormatsAndMessageBoundaries(t *testing.T) {
	t.Parallel()

	home := t.TempDir()
	dir := "/repo/trade"
	claudeRoot := filepath.Join(home, ".claude")
	writeConversation(t, filepath.Join(claudeRoot, "projects", slug(dir), firstSession+".jsonl"),
		`{"type":"user","origin":{"kind":"human"},"message":{"content":"Fix PR #649"}}`+"\n"+
			`{"type":"user","message":{"content":[{"type":"tool_result","content":"PR #999"}]}}`+"\n"+
			`{"type":"assistant","message":{"content":[{"type":"text","text":"Opened PR #650"}]}}`+"\n")
	writeConversation(
		t,
		filepath.Join(
			home,
			".codex",
			"sessions",
			"2026",
			"10",
			"05",
			"rollout-"+secondSession+".jsonl",
		),
		`{"type":"response_item","payload":{"type":"message","role":"user","content":[{"type":"input_text","text":"Review PR #675"}]}}`+"\n"+
			`{"type":"response_item","payload":{"type":"function_call_output","role":"tool","content":[{"type":"output_text","text":"PR #999"}]}}`+"\n"+
			`{"type":"response_item","payload":{"type":"message","role":"assistant","content":[{"type":"output_text","text":"Opened PR #676"}]}}`+"\n",
	)

	reader := NewConversations(home, []string{claudeRoot})
	if got := reader.Mention(Session{Agent: "claude", ID: firstSession}, dir); got != 650 {
		t.Errorf("Claude mention = %d, want 650", got)
	}

	if got := reader.Mention(Session{Agent: "codex", ID: secondSession}, dir); got != 676 {
		t.Errorf("Codex mention = %d, want 676", got)
	}

	if got := reader.Mention(Session{Agent: "codex", ID: "../escape"}, dir); got != 0 {
		t.Errorf("invalid session = %d, want none", got)
	}
}

func writeConversation(t *testing.T, path, content string) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func appendConversation(t *testing.T, path, content string) {
	t.Helper()

	file, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := file.WriteString(content); err != nil {
		_ = file.Close()

		t.Fatal(err)
	}

	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
}
