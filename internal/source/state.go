package source

import (
	"net/url"
	"path/filepath"
	"strings"
)

type Kind int

const (
	KindUnknown Kind = iota
	KindIdle
	KindDone
	KindThink
	KindEdit
	KindExec
	KindRead
	KindAgent
	KindBlocked
)

type Status struct {
	Kind   Kind
	Label  string
	Detail string
	Since  int64 // unix ms
}

// Attention is true for states the user should look at.
func (s Status) Attention() bool { return s.Kind == KindBlocked || s.Kind == KindDone }

func (s Status) Busy() bool { return s.Kind >= KindThink && s.Kind != KindBlocked }

// coarse returns busy|idle|waiting plus the time it was set.
func coarse(cs *ClaudeSession, hs *HookState) (string, int64) {
	st, since := "", int64(0)
	if cs != nil && cs.Status != "" {
		st, since = cs.Status, cs.StatusUpdatedAt
	}
	if hs == nil || (st != "" && hs.TS <= since) {
		return st, since
	}
	hst := ""
	switch hs.Event {
	case "Notification":
		if isPermission(hs.Notification) {
			hst = "waiting"
		}
	case "Stop", "SessionStart":
		hst = "idle"
	case "UserPromptSubmit", "PreToolUse", "PostToolUse", "SubagentStop":
		hst = "busy"
	}
	// claude's own status wins; a newer hook may only report that a busy
	// session stopped or waits. Stop does not fire on interrupts, and late
	// events (subagents, background tasks) must not revive an idle session.
	switch {
	case hst == "":
	case st == "":
		st, since = hst, hs.TS
	case st == "busy" && (hst == "idle" || hst == "waiting"):
		st, since = hst, hs.TS
	}
	return st, since
}

func isPermission(n string) bool {
	return n == "permission_prompt" || n == "elicitation_dialog" || n == "agent_needs_input"
}

// Derive turns raw sources into a display status. doneAt/seenAt are unix ms.
func Derive(cs *ClaudeSession, hs *HookState, doneAt, seenAt int64) Status {
	st, since := coarse(cs, hs)
	switch st {
	case "waiting":
		d := ""
		if hs != nil && hs.Event == "Notification" {
			d = hs.Message
		}
		if d == "" && cs != nil {
			d = cs.WaitingFor
		}
		return Status{Kind: KindBlocked, Label: "needs approval", Detail: d, Since: since}
	case "busy":
		s := toolStatus(hs)
		s.Since = since
		if hs != nil && hs.TS > since && s.Kind != KindThink {
			s.Since = hs.TS
		}
		return s
	case "idle":
		if hs != nil && hs.DoneAt > doneAt {
			doneAt = hs.DoneAt
		}
		if doneAt > 0 && doneAt > seenAt {
			return Status{Kind: KindDone, Label: "done", Detail: "awaiting input", Since: doneAt}
		}
		if doneAt > since {
			since = doneAt
		}
		return Status{Kind: KindIdle, Label: "awaiting input", Since: since}
	}
	return Status{Kind: KindUnknown, Label: "unknown", Since: since}
}

func toolStatus(hs *HookState) Status {
	if hs == nil || hs.Tool == "" || (hs.Event != "PreToolUse" && hs.Event != "PostToolUse") {
		return Status{Kind: KindThink, Label: "thinking"}
	}
	d := hs.ToolSummary
	switch hs.Tool {
	case "Edit", "Write", "MultiEdit", "NotebookEdit":
		return Status{Kind: KindEdit, Label: "editing", Detail: d}
	case "Bash", "BashOutput", "KillShell", "Monitor":
		return Status{Kind: KindExec, Label: "running", Detail: d}
	case "Read", "Grep", "Glob", "LS", "LSP":
		return Status{Kind: KindRead, Label: "reading", Detail: d}
	case "WebFetch", "WebSearch":
		return Status{Kind: KindRead, Label: "researching", Detail: d}
	case "Agent", "Task", "Workflow":
		return Status{Kind: KindAgent, Label: "subagents", Detail: d}
	case "TodoWrite", "TaskCreate", "TaskUpdate":
		return Status{Kind: KindThink, Label: "planning", Detail: d}
	}
	name := hs.Tool
	if strings.HasPrefix(name, "mcp__") {
		parts := strings.Split(name, "__")
		name = parts[len(parts)-1]
	}
	return Status{Kind: KindThink, Label: "tool", Detail: name}
}

// SummarizeTool builds a short human description from a tool's input.
func SummarizeTool(tool string, in map[string]any) string {
	str := func(k string) string {
		v, _ := in[k].(string)
		return v
	}
	var s string
	switch tool {
	case "Edit", "Write", "MultiEdit", "Read", "NotebookEdit":
		s = filepath.Base(str("file_path"))
		if s == "." {
			s = filepath.Base(str("notebook_path"))
		}
	case "Bash":
		s = str("description")
		if s == "" {
			s = str("command")
		}
	case "Grep", "Glob":
		s = str("pattern")
	case "WebFetch":
		if u, err := url.Parse(str("url")); err == nil {
			s = u.Host
		}
	case "WebSearch":
		s = str("query")
	case "Agent", "Task":
		s = str("description")
	}
	s = strings.Join(strings.Fields(s), " ")
	return Truncate(s, 80)
}

func Truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	if n <= 1 {
		return string(r[:n])
	}
	return string(r[:n-1]) + "…"
}
