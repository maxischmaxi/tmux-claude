// Package tmuxctl manages the global sidebar pane via the tmux cli.
package tmuxctl

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

const (
	optPane  = "@claude_sidebar_pane"
	optWidth = "@claude-sidebar-width"
	hookIdx  = "[4711]"
)

var followHooks = []string{"session-window-changed", "client-session-changed", "after-select-window"}

func tmux(args ...string) (string, error) {
	out, err := exec.Command("tmux", args...).Output()
	return strings.TrimRight(string(out), "\n"), err
}

func Width() int {
	w, _ := tmux("show", "-gqv", optWidth)
	if n, err := strconv.Atoi(w); err == nil && n >= 16 {
		return n
	}
	return 34
}

// SidebarPane returns the sidebar pane id if it still exists.
func SidebarPane() string {
	p, _ := tmux("show", "-gqv", optPane)
	if p == "" {
		return ""
	}
	if id, err := tmux("display", "-p", "-t", p, "#{pane_id}"); err != nil || id != p {
		return ""
	}
	return p
}

// Toggle opens the sidebar in window (e.g. "@3") or closes it.
func Toggle(bin, window string) error {
	if p := SidebarPane(); p != "" {
		return Close(p)
	}
	return Open(bin, window)
}

func Open(bin, window string) error {
	args := []string{"split-window", "-hbf", "-d", "-l", strconv.Itoa(Width()), "-P", "-F", "#{pane_id}"}
	if window != "" {
		args = append(args, "-t", window)
	}
	args = append(args, "exec '"+bin+"' sidebar")
	p, err := tmux(args...)
	if err != nil {
		return fmt.Errorf("split-window: %w", err)
	}
	tmux("set", "-p", "-t", p, "@claude_sidebar", "1")
	tmux("set", "-p", "-t", p, "remain-on-exit", "off")
	tmux("select-pane", "-t", p, "-T", "claude")
	tmux("set", "-g", optPane, p)
	for _, h := range followHooks {
		tmux("set-hook", "-g", h+hookIdx, "run-shell -b \"'"+bin+"' follow '#{window_id}'\"")
	}
	return nil
}

func Close(p string) error {
	unhook()
	if p != "" {
		tmux("kill-pane", "-t", p)
	}
	return nil
}

func unhook() {
	tmux("set", "-gu", optPane)
	for _, h := range followHooks {
		tmux("set-hook", "-gu", h+hookIdx)
	}
}

// Follow moves the sidebar into window if a client is looking at it.
func Follow(window string) error {
	p := SidebarPane()
	if p == "" {
		unhook()
		return nil
	}
	if window == "" {
		return nil
	}
	info, err := tmux("display", "-p", "-t", window, "#{window_id}\t#{session_attached}\t#{window_active}")
	if err != nil {
		return nil
	}
	f := strings.Split(info, "\t")
	if len(f) < 3 || f[1] == "0" || f[2] != "1" {
		return nil
	}
	cur, _ := tmux("display", "-p", "-t", p, "#{window_id}")
	if cur == f[0] {
		return nil
	}
	// the sidebar may be the only pane left in its window; then just move it
	_, err = tmux("join-pane", "-hbf", "-d", "-l", strconv.Itoa(Width()), "-s", p, "-t", f[0])
	return err
}

// Jump focuses the pane, switching session and window as needed.
func Jump(pane string) error {
	_, err := tmux("switch-client", "-t", pane, ";", "select-window", "-t", pane, ";", "select-pane", "-t", pane)
	return err
}
