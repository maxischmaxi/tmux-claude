package main

import (
	"fmt"
	"os"
	"path/filepath"
	"text/tabwriter"

	"github.com/maxischmaxi/tmux-claude/internal/hook"
	"github.com/maxischmaxi/tmux-claude/internal/source"
	"github.com/maxischmaxi/tmux-claude/internal/tmuxctl"
	"github.com/maxischmaxi/tmux-claude/internal/tui"
)

const usage = `usage: tmux-claude <command>

  sidebar              run the sidebar tui in the current pane
  toggle [window]      open or close the global sidebar
  follow <window>      move the sidebar into window (used by tmux hooks)
  list                 print all claude instances with their state
  status               short summary for the tmux status line
  hook                 claude hook handler (reads event json from stdin)
  install-hooks        add hooks to ~/.claude/settings.json (--print: only show)
  uninstall-hooks      remove them again
`

func self() string {
	p, err := os.Executable()
	if err != nil {
		return "tmux-claude"
	}
	if r, err := filepath.EvalSymlinks(p); err == nil {
		return r
	}
	return p
}

func main() {
	if len(os.Args) < 2 {
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
	}
	arg := func(i int) string {
		if len(os.Args) > i {
			return os.Args[i]
		}
		return ""
	}
	var err error
	switch os.Args[1] {
	case "sidebar":
		err = tui.Run()
	case "toggle":
		err = tmuxctl.Toggle(self(), arg(2))
	case "follow":
		err = tmuxctl.Follow(arg(2))
	case "list":
		list()
	case "status":
		status()
	case "hook":
		hook.Run(os.Stdin)
	case "install-hooks":
		if arg(2) == "--print" {
			fmt.Println(hook.Snippet(self()))
			return
		}
		err = install(false)
	case "uninstall-hooks":
		err = install(true)
	default:
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "tmux-claude:", err)
		os.Exit(1)
	}
}

func install(remove bool) error {
	path := hook.SettingsPath()
	changed, err := hook.Install(path, self(), remove)
	if err != nil {
		return err
	}
	switch {
	case !changed:
		fmt.Println("nothing to do:", path)
	case remove:
		fmt.Println("hooks removed from", path)
	default:
		fmt.Println("hooks installed in", path, "(backup written next to it)")
	}
	return nil
}

func list() {
	tw := tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
	fmt.Fprintln(tw, "NAME\tPANE\tLOCATION\tSTATE\tDETAIL\tPID")
	for _, it := range source.NewCollector(false).Collect() {
		pane, loc := "-", "-"
		if it.Pane != nil {
			pane = it.Pane.ID
			loc = fmt.Sprintf("%s:%s.%s", it.Pane.Session, it.Pane.WindowIndex, it.Pane.PaneIndex)
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%d\n", it.Name, pane, loc, it.Status.Label, it.Status.Detail, it.PID)
	}
	tw.Flush()
}

func status() {
	var busy, blocked, done int
	for _, it := range source.NewCollector(false).Collect() {
		switch {
		case it.Status.Kind == source.KindBlocked:
			blocked++
		case it.Status.Kind == source.KindDone:
			done++
		case it.Status.Busy():
			busy++
		}
	}
	out := ""
	if blocked > 0 {
		out += fmt.Sprintf("#[fg=red,bold]! %d ", blocked)
	}
	if done > 0 {
		out += fmt.Sprintf("#[fg=green,bold]● %d ", done)
	}
	if busy > 0 {
		out += fmt.Sprintf("#[fg=yellow]◆ %d ", busy)
	}
	if out != "" {
		out += "#[default]"
	}
	fmt.Print(out)
}
