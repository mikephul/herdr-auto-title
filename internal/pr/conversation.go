package pr

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

const (
	maxRead       = 2 << 20
	findRetry     = 10 * time.Second
	userRole      = "user"
	assistantRole = "assistant"
)

var sessionUUID = regexp.MustCompile(`^[0-9a-fA-F]{8}(-[0-9a-fA-F]{4}){3}-[0-9a-fA-F]{12}$`)

// Session identifies one agent conversation, independently of its checkout.
type Session struct {
	Agent string
	ID    string
}

type conversation struct {
	path       string
	offset     int64
	number     int
	skipping   bool
	searchedAt time.Time
}

// Conversations remembers the last PR mentioned in each session's messages.
type Conversations struct {
	home       string
	claudeDirs []string
	sessions   map[Session]*conversation
}

func NewConversations(home string, claudeDirs []string) *Conversations {
	return &Conversations{
		home:       home,
		claudeDirs: claudeDirs,
		sessions:   make(map[Session]*conversation),
	}
}

func (r *Conversations) Retain(live []Session) {
	kept := make(map[Session]*conversation, len(live))
	for _, key := range live {
		if session, ok := r.sessions[key]; ok {
			kept[key] = session
		}
	}

	r.sessions = kept
}

// Mention returns the latest explicit PR in this chat, including earlier turns.
func (r *Conversations) Mention(key Session, dir string) int {
	if !sessionUUID.MatchString(key.ID) {
		return 0
	}

	session, known := r.sessions[key]
	if !known {
		session = &conversation{}
		r.sessions[key] = session
	}

	if session.path == "" {
		if !session.searchedAt.IsZero() && time.Since(session.searchedAt) < findRetry {
			return session.number
		}

		session.searchedAt = time.Now()

		session.path = r.locate(key, dir)
		if session.path == "" {
			return session.number
		}
	}

	session.read(key.Agent)

	return session.number
}

func (r *Conversations) locate(key Session, dir string) string {
	var candidates []string

	switch key.Agent {
	case "claude":
		for _, root := range r.claudeDirs {
			projects := filepath.Join(root, "projects")
			if dir != "" {
				candidates = append(candidates, filepath.Join(projects, slug(dir), key.ID+".jsonl"))
			}

			matches, _ := filepath.Glob(filepath.Join(projects, "*", key.ID+".jsonl"))
			candidates = append(candidates, matches...)
		}
	case "codex":
		matches, _ := filepath.Glob(
			filepath.Join(r.home, ".codex", "sessions", "*", "*", "*", "*"+key.ID+".jsonl"),
		)
		candidates = append(candidates, matches...)
	case "droid":
		root := filepath.Join(r.home, ".factory", "sessions")
		if dir != "" {
			candidates = append(candidates, filepath.Join(root, slug(dir), key.ID+".jsonl"))
		}

		matches, _ := filepath.Glob(filepath.Join(root, "*", key.ID+".jsonl"))
		candidates = append(candidates, matches...)
	}

	for _, path := range candidates {
		if info, err := os.Stat(path); err == nil && info.Mode().IsRegular() {
			return path
		}
	}

	return ""
}

func slug(dir string) string {
	var out strings.Builder

	for _, char := range dir {
		if char >= 'a' && char <= 'z' || char >= 'A' && char <= 'Z' || char >= '0' && char <= '9' {
			out.WriteRune(char)
		} else {
			out.WriteByte('-')
		}
	}

	return out.String()
}

func (s *conversation) read(agent string) {
	info, err := os.Stat(s.path)
	if err != nil {
		s.path, s.offset = "", 0
		return
	}

	if info.Size() < s.offset {
		s.offset, s.number, s.skipping = 0, 0, false
	}

	if info.Size() == s.offset {
		return
	}

	file, err := os.Open(s.path)
	if err != nil {
		return
	}
	defer func() { _ = file.Close() }()

	if _, err := file.Seek(s.offset, io.SeekStart); err != nil {
		return
	}

	content, err := io.ReadAll(io.LimitReader(file, maxRead))
	if err != nil {
		return
	}

	end := bytes.LastIndexByte(content, '\n')
	if end < 0 {
		if len(content) == maxRead {
			s.offset += int64(len(content))
			s.skipping = true
		}

		return
	}

	start := 0
	if s.skipping {
		start = bytes.IndexByte(content, '\n') + 1
		s.skipping = false
	}

	for line := range bytes.SplitSeq(content[start:end], []byte{'\n'}) {
		if number := mentionedInChat(agent, line); number > 0 {
			s.number = number
		}
	}

	s.offset += int64(end + 1)
}

type chatLine struct {
	Type   string `json:"type"`
	Origin *struct {
		Kind string `json:"kind"`
	} `json:"origin"`
	Message struct {
		Role    string          `json:"role"`
		Content json.RawMessage `json:"content"`
	} `json:"message"`
	Payload struct {
		Type    string          `json:"type"`
		Role    string          `json:"role"`
		Content json.RawMessage `json:"content"`
	} `json:"payload"`
}

func mentionedInChat(agent string, line []byte) int {
	var read chatLine
	if json.Unmarshal(line, &read) != nil {
		return 0
	}

	switch agent {
	case "claude":
		if read.Type == userRole && (read.Origin == nil || read.Origin.Kind == "human") {
			return mentionContent(read.Message.Content, "text")
		}

		if read.Type == assistantRole {
			return mentionContent(read.Message.Content, "text")
		}
	case "codex":
		if read.Type == "response_item" && read.Payload.Type == "message" {
			return mentionedByRole(
				read.Payload.Role,
				read.Payload.Content,
				"input_text",
				"output_text",
			)
		}
	case "droid":
		if read.Type == "message" {
			return mentionedByRole(read.Message.Role, read.Message.Content, "text", "text")
		}
	}

	return 0
}

func mentionedByRole(role string, content json.RawMessage, userKind, assistantKind string) int {
	switch role {
	case userRole:
		return mentionContent(content, userKind)
	case assistantRole:
		return mentionContent(content, assistantKind)
	default:
		return 0
	}
}

func mentionContent(raw json.RawMessage, kind string) int {
	var plain string
	if json.Unmarshal(raw, &plain) == nil {
		return Mention(plain)
	}

	var blocks []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	if json.Unmarshal(raw, &blocks) != nil {
		return 0
	}

	number := 0

	for _, block := range blocks {
		if block.Type == kind {
			if found := Mention(block.Text); found > 0 {
				number = found
			}
		}
	}

	return number
}
