package source

import (
	"os/exec"
	"strconv"
	"strings"
)

type Pane struct {
	ID          string
	PID         int
	Session     string
	WindowID    string
	WindowIndex string
	WindowName  string
	PaneIndex   string
	Command     string
	Title       string
	Attached    bool
	WindowAct   bool
	PaneAct     bool
	IsSidebar   bool
}

// Focused is true when the pane is what an attached client looks at.
func (p Pane) Focused() bool { return p.Attached && p.WindowAct && p.PaneAct }

const paneFormat = "#{pane_id}\t#{pane_pid}\t#{session_name}\t#{window_id}\t#{window_index}\t#{window_name}\t#{pane_index}\t#{pane_current_command}\t#{session_attached}\t#{window_active}\t#{pane_active}\t#{@claude_sidebar}\t#{pane_title}"

// ListPanes returns all tmux panes keyed by pane id (nil if tmux is not running).
func ListPanes() map[string]Pane {
	out, err := exec.Command("tmux", "list-panes", "-a", "-F", paneFormat).Output()
	if err != nil {
		return nil
	}
	return parsePanes(string(out))
}

func parsePanes(s string) map[string]Pane {
	panes := map[string]Pane{}
	for _, line := range strings.Split(strings.TrimRight(s, "\n"), "\n") {
		f := strings.SplitN(line, "\t", 13)
		if len(f) < 13 {
			continue
		}
		pid, _ := strconv.Atoi(f[1])
		att, _ := strconv.Atoi(f[8])
		panes[f[0]] = Pane{
			ID: f[0], PID: pid, Session: f[2], WindowID: f[3], WindowIndex: f[4],
			WindowName: f[5], PaneIndex: f[6], Command: f[7], Attached: att > 0,
			WindowAct: f[9] == "1", PaneAct: f[10] == "1", IsSidebar: f[11] == "1",
			Title: f[12],
		}
	}
	return panes
}
