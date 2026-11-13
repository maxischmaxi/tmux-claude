package source

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDerive(t *testing.T) {
	cs := func(st string, ts int64) *ClaudeSession { return &ClaudeSession{Status: st, StatusUpdatedAt: ts} }
	hs := func(ev, tool string, ts int64) *HookState {
		return &HookState{Event: ev, Tool: tool, TS: ts, ToolSummary: "x"}
	}
	cases := []struct {
		name         string
		cs           *ClaudeSession
		hs           *HookState
		done, seen   int64
		want         Kind
		wantLabelSub string
	}{
		{"busy without hooks", cs("busy", 10), nil, 0, 0, KindThink, "thinking"},
		{"edit", cs("busy", 10), hs("PreToolUse", "Edit", 20), 0, 0, KindEdit, "editing"},
		{"bash", cs("busy", 10), hs("PostToolUse", "Bash", 20), 0, 0, KindExec, "running"},
		{"grep", cs("busy", 10), hs("PreToolUse", "Grep", 20), 0, 0, KindRead, "reading"},
		{"agent", cs("busy", 10), hs("PreToolUse", "Agent", 20), 0, 0, KindAgent, "subagents"},
		{"waiting from session file", cs("waiting", 10), nil, 0, 0, KindBlocked, "approval"},
		{"permission hook newer", cs("busy", 10), &HookState{Event: "Notification", Notification: "permission_prompt", TS: 20}, 0, 0, KindBlocked, "approval"},
		{"stale hook ignored", cs("idle", 30), hs("PreToolUse", "Edit", 20), 0, 0, KindIdle, "awaiting"},
		{"stop hook newer than busy", cs("busy", 10), &HookState{Event: "Stop", TS: 20, DoneAt: 20}, 0, 0, KindDone, "done"},
		{"done but seen", cs("idle", 30), &HookState{Event: "Stop", TS: 20, DoneAt: 20}, 0, 25, KindIdle, "awaiting"},
		{"observed transition", cs("idle", 30), nil, 30, 0, KindDone, "done"},
		{"late hook does not revive idle", cs("idle", 10), hs("PostToolUse", "Bash", 20), 0, 0, KindIdle, "awaiting"},
		{"interrupt without stop", cs("idle", 10), hs("UserPromptSubmit", "", 20), 0, 0, KindIdle, "awaiting"},
		{"hook only", nil, hs("PreToolUse", "Write", 5), 0, 0, KindEdit, "editing"},
		{"nothing", nil, nil, 0, 0, KindUnknown, "unknown"},
	}
	for _, c := range cases {
		got := Derive(c.cs, c.hs, c.done, c.seen)
		if got.Kind != c.want {
			t.Errorf("%s: kind %v, want %v (%+v)", c.name, got.Kind, c.want, got)
		}
		if c.wantLabelSub != "" && !strings.Contains(got.Label, c.wantLabelSub) {
			t.Errorf("%s: label %q, want %q", c.name, got.Label, c.wantLabelSub)
		}
	}
}

func TestPaneID(t *testing.T) {
	if got := (ClaudeSession{Tmux: "tmux-claude:@12.%14"}).PaneID(); got != "%14" {
		t.Fatalf("got %q", got)
	}
	if got := (ClaudeSession{}).PaneID(); got != "" {
		t.Fatalf("got %q", got)
	}
}

func TestSummarizeTool(t *testing.T) {
	if s := SummarizeTool("Edit", map[string]any{"file_path": "/a/b/c.go"}); s != "c.go" {
		t.Errorf("edit: %q", s)
	}
	if s := SummarizeTool("Bash", map[string]any{"command": "ls\n  -la"}); s != "ls -la" {
		t.Errorf("bash: %q", s)
	}
	if s := SummarizeTool("WebFetch", map[string]any{"url": "https://example.com/x"}); s != "example.com" {
		t.Errorf("webfetch: %q", s)
	}
}

func writeStat(t *testing.T, root string, pid, comm, ppid, start string) {
	t.Helper()
	d := filepath.Join(root, pid)
	os.MkdirAll(d, 0o755)
	// fields 3..22 after "(comm)"
	stat := pid + " (" + comm + ") S " + ppid + " 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 " + start + " 0 0\n"
	if err := os.WriteFile(filepath.Join(d, "stat"), []byte(stat), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestProc(t *testing.T) {
	root := t.TempDir()
	procRoot = root
	os.MkdirAll(filepath.Join(root, "self"), 0o755)
	defer func() { procRoot = "/proc" }()

	writeStat(t, root, "100", "claude", "50", "777")
	writeStat(t, root, "200", "my hook (x)", "100", "888")

	if !Alive(100, "777") || Alive(100, "778") || Alive(999, "") {
		t.Fatal("alive check wrong")
	}
	if pid, start := FindClaudeAncestor(200); pid != 100 || start != "777" {
		t.Fatalf("ancestor %d %s", pid, start)
	}
}

func TestParsePanes(t *testing.T) {
	p := parsePanes("%1\t10\tmain\t@1\t0\tzsh\t0\tzsh\t1\t1\t1\t\ttitle\n%2\t11\twork\t@2\t3\tcc\t1\tclaude\t0\t1\t0\t1\t✳ x\ty\n")
	if len(p) != 2 || !p["%1"].Focused() || p["%2"].Focused() || !p["%2"].IsSidebar || p["%2"].Title != "✳ x\ty" {
		t.Fatalf("%+v", p)
	}
}
