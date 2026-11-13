package source

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"time"
)

type Instance struct {
	ID     string // claude session id, or pane id for fallback entries
	Name   string
	Cwd    string
	PID    int
	Pane   *Pane
	Status Status
	Prompt string
}

// Collector merges all sources and remembers transitions between calls.
type Collector struct {
	SessionsDir string
	StateDir    string
	SeenFile    string
	TrackSeen   bool // mark focused instances as seen and persist it

	prev map[string]string
	done map[string]int64
	seen map[string]int64
}

func NewCollector(trackSeen bool) *Collector {
	return &Collector{
		SessionsDir: SessionsDir(),
		StateDir:    StateDir(),
		SeenFile:    SeenFile(),
		TrackSeen:   trackSeen,
		prev:        map[string]string{},
		done:        map[string]int64{},
		seen:        map[string]int64{},
	}
}

func (c *Collector) Collect() []Instance {
	now := time.Now().UnixMilli()
	sessions := ReadSessions(c.SessionsDir)
	hooks := ReadHookStates(c.StateDir)
	panes := ListPanes()
	c.loadSeen()

	var out []Instance
	usedPanes := map[string]bool{}
	known := map[string]bool{}

	add := func(id string, cs *ClaudeSession, hs *HookState) {
		inst := Instance{ID: id}
		paneID := ""
		if cs != nil {
			inst.Name, inst.Cwd, inst.PID, paneID = cs.Name, cs.Cwd, cs.PID, cs.PaneID()
		}
		if hs != nil {
			if inst.Cwd == "" {
				inst.Cwd = hs.Cwd
			}
			if inst.PID == 0 {
				inst.PID = hs.PID
			}
			if paneID == "" {
				paneID = hs.Pane
			}
			inst.Prompt = hs.Prompt
		}
		if p, ok := panes[paneID]; ok {
			inst.Pane = &p
			usedPanes[paneID] = true
		}
		if inst.Name == "" {
			inst.Name = filepath.Base(inst.Cwd)
		}

		st, _ := coarse(cs, hs)
		if prev := c.prev[id]; (prev == "busy" || prev == "waiting") && st == "idle" {
			c.done[id] = now
		}
		c.prev[id] = st
		inst.Status = Derive(cs, hs, c.done[id], c.seen[id])
		// only acknowledge unread results, so the seen file rarely changes
		if c.TrackSeen && inst.Status.Kind == KindDone && inst.Pane != nil && inst.Pane.Focused() {
			c.markSeen(id, now)
			inst.Status = Derive(cs, hs, c.done[id], c.seen[id])
		}
		out = append(out, inst)
		known[id] = true
	}

	for i := range sessions {
		cs := &sessions[i]
		var hs *HookState
		if h, ok := hooks[cs.SessionID]; ok {
			hs = &h
		}
		add(cs.SessionID, cs, hs)
	}
	for id, h := range hooks {
		if known[id] || h.PID == 0 {
			continue
		}
		h := h
		add(id, nil, &h)
	}
	// claude processes neither registered nor hooked
	for id, p := range panes {
		if usedPanes[id] || p.Command != "claude" {
			continue
		}
		p := p
		out = append(out, Instance{ID: id, Name: p.WindowName, Pane: &p,
			Status: Status{Kind: KindUnknown, Label: "unknown"}})
	}

	for id := range c.prev {
		if !known[id] {
			delete(c.prev, id)
			delete(c.done, id)
		}
	}
	if c.TrackSeen {
		c.saveSeen(known)
	}
	sortInstances(out)
	return out
}

// MarkSeen records that the user looked at an instance (e.g. jumped to it).
func (c *Collector) MarkSeen(id string) {
	c.markSeen(id, time.Now().UnixMilli())
}

func (c *Collector) markSeen(id string, now int64) {
	c.seen[id] = now
}

func (c *Collector) loadSeen() {
	b, err := os.ReadFile(c.SeenFile)
	if err != nil {
		return
	}
	var m map[string]int64
	if json.Unmarshal(b, &m) != nil {
		return
	}
	for k, v := range m {
		if v > c.seen[k] {
			c.seen[k] = v
		}
	}
}

func (c *Collector) saveSeen(known map[string]bool) {
	for k := range c.seen {
		if !known[k] {
			delete(c.seen, k)
		}
	}
	b, _ := json.Marshal(c.seen)
	if old, err := os.ReadFile(c.SeenFile); err == nil && string(old) == string(b) {
		return
	}
	os.MkdirAll(filepath.Dir(c.SeenFile), 0o755)
	tmp := c.SeenFile + ".tmp"
	if os.WriteFile(tmp, b, 0o644) == nil {
		os.Rename(tmp, c.SeenFile)
	}
}

func sortInstances(xs []Instance) {
	num := func(s string) int { n, _ := strconv.Atoi(s); return n }
	sort.SliceStable(xs, func(i, j int) bool {
		a, b := xs[i].Pane, xs[j].Pane
		if (a == nil) != (b == nil) {
			return a != nil
		}
		if a == nil {
			return xs[i].Name < xs[j].Name
		}
		if a.Session != b.Session {
			return a.Session < b.Session
		}
		if a.WindowIndex != b.WindowIndex {
			return num(a.WindowIndex) < num(b.WindowIndex)
		}
		return num(a.PaneIndex) < num(b.PaneIndex)
	})
}
