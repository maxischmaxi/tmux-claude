// Package tui renders the sidebar listing all claude instances.
package tui

import (
	"fmt"
	"os"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/fsnotify/fsnotify"

	"github.com/maxischmaxi/tmux-claude/internal/source"
	"github.com/maxischmaxi/tmux-claude/internal/tmuxctl"
)

// ansi colors so the sidebar follows the terminal theme
var (
	cRed     = lipgloss.Color("1")
	cGreen   = lipgloss.Color("2")
	cYellow  = lipgloss.Color("3")
	cBlue    = lipgloss.Color("4")
	cMagenta = lipgloss.Color("5")
	cCyan    = lipgloss.Color("6")
	cGray    = lipgloss.Color("8")

	sTitle   = lipgloss.NewStyle().Bold(true).Foreground(cCyan)
	sGroup   = lipgloss.NewStyle().Foreground(cGray)
	sDim     = lipgloss.NewStyle().Foreground(cGray)
	sName    = lipgloss.NewStyle().Bold(true)
	sCurrent = lipgloss.NewStyle().Reverse(true)
)

func kindStyle(k source.Kind) (string, lipgloss.Style) {
	s := lipgloss.NewStyle()
	switch k {
	case source.KindBlocked:
		return "!", s.Foreground(cRed).Bold(true)
	case source.KindDone:
		return "●", s.Foreground(cGreen).Bold(true)
	case source.KindEdit, source.KindExec, source.KindThink:
		return "◆", s.Foreground(cYellow)
	case source.KindRead:
		return "◆", s.Foreground(cBlue)
	case source.KindAgent:
		return "◆", s.Foreground(cMagenta)
	case source.KindIdle:
		return "○", s.Foreground(cGray)
	}
	return "?", s.Foreground(cGray)
}

type (
	tickMsg    time.Time
	changedMsg struct{}
)

type model struct {
	col       *source.Collector
	items     []source.Instance
	cursor    int
	width     int
	height    int
	lineItem  []int // rendered line -> item index, -1 for none
	selfPane  string
	changes   chan struct{}
	lastFocus string
}

func Run() error {
	m := &model{
		col:      source.NewCollector(true),
		selfPane: os.Getenv("TMUX_PANE"),
		changes:  make(chan struct{}, 1),
	}
	m.watch()
	m.refresh()
	p := tea.NewProgram(m, tea.WithAltScreen(), tea.WithMouseCellMotion())
	_, err := p.Run()
	return err
}

// watch signals on any change in the sessions or state directories.
func (m *model) watch() {
	w, err := fsnotify.NewWatcher()
	if err != nil {
		return
	}
	for _, d := range []string{source.SessionsDir(), source.StateDir()} {
		os.MkdirAll(d, 0o755)
		w.Add(d)
	}
	go func() {
		for {
			select {
			case _, ok := <-w.Events:
				if !ok {
					return
				}
				select {
				case m.changes <- struct{}{}:
				default:
				}
			case _, ok := <-w.Errors:
				if !ok {
					return
				}
			}
		}
	}()
}

func (m *model) waitChange() tea.Cmd {
	return func() tea.Msg {
		<-m.changes
		// coalesce bursts of hook writes
		time.Sleep(80 * time.Millisecond)
		return changedMsg{}
	}
}

func tick() tea.Cmd {
	return tea.Tick(2*time.Second, func(t time.Time) tea.Msg { return tickMsg(t) })
}

func (m *model) Init() tea.Cmd { return tea.Batch(tick(), m.waitChange()) }

func (m *model) refresh() {
	var sel string
	if m.cursor >= 0 && m.cursor < len(m.items) {
		sel = m.items[m.cursor].ID
	}
	m.items = m.col.Collect()
	m.cursor = 0
	for i, it := range m.items {
		if it.ID == sel {
			m.cursor = i
		}
	}
	// keep the cursor on the instance the user is looking at
	if f := m.focused(); f != "" && f != m.lastFocus {
		m.lastFocus = f
		for i, it := range m.items {
			if it.ID == f {
				m.cursor = i
			}
		}
	}
}

func (m *model) focused() string {
	for _, it := range m.items {
		if it.Pane != nil && it.Pane.Focused() {
			return it.ID
		}
	}
	return ""
}

func (m *model) jump(i int) {
	if i < 0 || i >= len(m.items) || m.items[i].Pane == nil {
		return
	}
	m.cursor = i
	m.col.MarkSeen(m.items[i].ID)
	tmuxctl.Jump(m.items[i].Pane.ID)
	m.refresh()
}

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
	case tickMsg:
		m.refresh()
		return m, tick()
	case changedMsg:
		m.refresh()
		return m, m.waitChange()
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			tmuxctl.Close("")
			return m, tea.Quit
		case "j", "down":
			if m.cursor < len(m.items)-1 {
				m.cursor++
			}
		case "k", "up":
			if m.cursor > 0 {
				m.cursor--
			}
		case "g", "home":
			m.cursor = 0
		case "G", "end":
			m.cursor = len(m.items) - 1
		case "enter", "l", " ":
			m.jump(m.cursor)
		case "r":
			m.refresh()
		default:
			if s := msg.String(); len(s) == 1 && s[0] >= '1' && s[0] <= '9' {
				m.jump(int(s[0] - '1'))
			}
		}
	case tea.MouseMsg:
		switch {
		case msg.Action == tea.MouseActionPress && msg.Button == tea.MouseButtonLeft:
			if msg.Y >= 0 && msg.Y < len(m.lineItem) && m.lineItem[msg.Y] >= 0 {
				m.jump(m.lineItem[msg.Y])
			}
		case msg.Button == tea.MouseButtonWheelDown && m.cursor < len(m.items)-1:
			m.cursor++
		case msg.Button == tea.MouseButtonWheelUp && m.cursor > 0:
			m.cursor--
		}
	}
	return m, nil
}

func ago(ms int64) string {
	if ms <= 0 {
		return ""
	}
	d := time.Since(time.UnixMilli(ms))
	switch {
	case d < time.Minute:
		return fmt.Sprintf("%ds", int(d.Seconds()))
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	case d < 48*time.Hour:
		return fmt.Sprintf("%dh", int(d.Hours()))
	}
	return fmt.Sprintf("%dd", int(d.Hours()/24))
}

// row renders left and right text into exactly w cells.
func row(w int, left, right string) string {
	lw, rw := lipgloss.Width(left), lipgloss.Width(right)
	if lw+rw+1 > w {
		left = truncateStyled(left, w-rw-1)
		lw = lipgloss.Width(left)
	}
	gap := w - lw - rw
	if gap < 0 {
		gap = 0
	}
	return left + strings.Repeat(" ", gap) + right
}

func truncateStyled(s string, w int) string {
	if w <= 0 {
		return ""
	}
	return lipgloss.NewStyle().MaxWidth(w).Render(s)
}

func (m *model) View() string {
	w := m.width
	if w <= 0 {
		w = 34
	}
	inner := w - 1
	var lines []string
	m.lineItem = m.lineItem[:0]
	push := func(s string, item int) {
		lines = append(lines, " "+s)
		m.lineItem = append(m.lineItem, item)
	}

	attn := 0
	for _, it := range m.items {
		if it.Status.Attention() {
			attn++
		}
	}
	right := sDim.Render(fmt.Sprint(len(m.items)))
	if attn > 0 {
		right = lipgloss.NewStyle().Foreground(cGreen).Bold(true).Render(fmt.Sprintf("%d new", attn)) + sDim.Render(" / ") + right
	}
	push(row(inner, sTitle.Render("claude"), right), -1)
	push("", -1)

	if len(m.items) == 0 {
		push(sDim.Render("no claude sessions"), -1)
	}

	group := "\x00"
	for i, it := range m.items {
		g := "no tmux"
		if it.Pane != nil {
			g = it.Pane.Session
		}
		if g != group {
			if group != "\x00" {
				push("", -1)
			}
			group = g
			label := "─ " + g + " "
			push(sGroup.Render(label+strings.Repeat("─", max(0, inner-lipgloss.Width(label)))), -1)
		}

		icon, st := kindStyle(it.Status.Kind)
		loc := ""
		if it.Pane != nil {
			loc = it.Pane.WindowIndex + "." + it.Pane.PaneIndex
		}
		num := " "
		if i < 9 {
			num = fmt.Sprint(i + 1)
		}
		name := sName.Render(it.Name)
		l1 := row(inner, sDim.Render(num)+" "+st.Render(icon)+" "+name, sDim.Render(loc))

		status := st.Render(it.Status.Label)
		if it.Status.Detail != "" {
			status += sDim.Render(" " + it.Status.Detail)
		}
		l2 := row(inner, "    "+status, sDim.Render(ago(it.Status.Since)))

		if i == m.cursor {
			l1 = sCurrent.Render(l1)
		}
		push(l1, i)
		push(l2, i)
		if i == m.cursor && it.Prompt != "" {
			push(sDim.Render(truncateStyled("    › "+it.Prompt, inner)), i)
		}
	}

	if m.height > 0 && len(lines) > m.height {
		// keep the cursor visible
		start := 0
		for li, idx := range m.lineItem {
			if idx == m.cursor {
				start = li
				break
			}
		}
		if start+m.height > len(lines) {
			start = len(lines) - m.height
		}
		if start < 0 {
			start = 0
		}
		if start < 2 {
			start = 0
		}
		lines = lines[start : start+m.height]
		m.lineItem = m.lineItem[start : start+m.height]
	}
	return strings.Join(lines, "\n")
}
