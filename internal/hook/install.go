package hook

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

var Events = []string{
	"SessionStart", "UserPromptSubmit", "PreToolUse", "PostToolUse",
	"Notification", "Stop", "SubagentStop", "SessionEnd",
}

func SettingsPath() string {
	if d := os.Getenv("CLAUDE_CONFIG_DIR"); d != "" {
		return filepath.Join(d, "settings.json")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".claude", "settings.json")
}

func Command(bin string) string { return "'" + bin + "' hook" }

func entry(bin string) json.RawMessage {
	e := map[string]any{"hooks": []map[string]any{{
		"type": "command", "command": Command(bin), "timeout": 5,
	}}}
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.Encode(e)
	return bytes.TrimSpace(buf.Bytes())
}

// Snippet is the hooks block for manual installation.
func Snippet(bin string) string {
	var parts []string
	for _, ev := range Events {
		parts = append(parts, fmt.Sprintf("%q: [%s]", ev, entry(bin)))
	}
	var out bytes.Buffer
	json.Indent(&out, []byte(`{"hooks":{`+strings.Join(parts, ",")+`}}`), "", "  ")
	return out.String()
}

// Install adds our hooks to settings.json, keeping key order and other hooks.
// remove=true strips them instead. Returns whether the file changed.
func Install(path, bin string, remove bool) (bool, error) {
	orig, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		orig = []byte("{}")
	} else if err != nil {
		return false, err
	}
	root, err := parseObject(orig)
	if err != nil {
		return false, fmt.Errorf("%s: %w", path, err)
	}
	hooksRaw, _ := root.get("hooks")
	if hooksRaw == nil {
		hooksRaw = json.RawMessage("{}")
	}
	hooks, err := parseObject(hooksRaw)
	if err != nil {
		return false, fmt.Errorf("hooks: %w", err)
	}

	cmd, _ := json.Marshal(Command(bin))
	ours := func(r json.RawMessage) bool { return bytes.Contains(r, cmd) }
	changed := false
	for _, ev := range Events {
		var list []json.RawMessage
		if raw, ok := hooks.get(ev); ok {
			if err := json.Unmarshal(raw, &list); err != nil {
				return false, fmt.Errorf("hooks.%s: %w", ev, err)
			}
		}
		has := false
		var kept []json.RawMessage
		for _, r := range list {
			if ours(r) {
				has = true
				if remove {
					changed = true
					continue
				}
			}
			kept = append(kept, r)
		}
		if !remove && !has {
			kept = append(kept, entry(bin))
			changed = true
		}
		if len(kept) == 0 {
			hooks.del(ev)
			continue
		}
		hooks.set(ev, joinArray(kept))
	}
	if !changed {
		return false, nil
	}
	if len(hooks) == 0 {
		root.del("hooks")
	} else {
		root.set("hooks", hooks.encode())
	}
	var out bytes.Buffer
	if err := json.Indent(&out, root.encode(), "", "  "); err != nil {
		return false, err
	}
	out.WriteByte('\n')

	if len(orig) > 2 {
		backup := path + ".bak-tmux-claude-" + time.Now().Format("20060102-150405")
		if err := os.WriteFile(backup, orig, 0o600); err != nil {
			return false, err
		}
	}
	tmp := path + ".tmp-tmux-claude"
	if err := os.WriteFile(tmp, out.Bytes(), 0o600); err != nil {
		return false, err
	}
	return true, os.Rename(tmp, path)
}

type kv struct {
	k string
	v json.RawMessage
}

// object is a json object that keeps its key order.
type object []kv

func parseObject(b []byte) (object, error) {
	dec := json.NewDecoder(bytes.NewReader(b))
	t, err := dec.Token()
	if err != nil {
		return nil, err
	}
	if d, ok := t.(json.Delim); !ok || d != '{' {
		return nil, fmt.Errorf("not a json object")
	}
	var o object
	for dec.More() {
		t, err := dec.Token()
		if err != nil {
			return nil, err
		}
		var v json.RawMessage
		if err := dec.Decode(&v); err != nil {
			return nil, err
		}
		o = append(o, kv{t.(string), v})
	}
	return o, nil
}

func (o object) get(k string) (json.RawMessage, bool) {
	for _, e := range o {
		if e.k == k {
			return e.v, true
		}
	}
	return nil, false
}

func (o *object) set(k string, v json.RawMessage) {
	for i, e := range *o {
		if e.k == k {
			(*o)[i].v = v
			return
		}
	}
	*o = append(*o, kv{k, v})
}

func (o *object) del(k string) {
	for i, e := range *o {
		if e.k == k {
			*o = append((*o)[:i], (*o)[i+1:]...)
			return
		}
	}
}

func (o object) encode() json.RawMessage {
	var b bytes.Buffer
	b.WriteByte('{')
	for i, e := range o {
		if i > 0 {
			b.WriteByte(',')
		}
		k, _ := json.Marshal(e.k)
		b.Write(k)
		b.WriteByte(':')
		b.Write(e.v)
	}
	b.WriteByte('}')
	return b.Bytes()
}

func joinArray(xs []json.RawMessage) json.RawMessage {
	var b bytes.Buffer
	b.WriteByte('[')
	for i, x := range xs {
		if i > 0 {
			b.WriteByte(',')
		}
		b.Write(x)
	}
	b.WriteByte(']')
	return b.Bytes()
}
