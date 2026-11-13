package source

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// HookState is written by `tmux-claude hook` for every claude hook event.
type HookState struct {
	SessionID    string `json:"session_id"`
	PID          int    `json:"pid,omitempty"`
	ProcStart    string `json:"proc_start,omitempty"`
	Pane         string `json:"tmux_pane,omitempty"`
	Cwd          string `json:"cwd,omitempty"`
	Event        string `json:"event"`
	Tool         string `json:"tool,omitempty"`
	ToolSummary  string `json:"tool_summary,omitempty"`
	Notification string `json:"notification,omitempty"`
	Message      string `json:"message,omitempty"`
	Prompt       string `json:"prompt,omitempty"`
	TS           int64  `json:"ts"`
	DoneAt       int64  `json:"done_at,omitempty"`
}

func StatePath(dir, sessionID string) string {
	return filepath.Join(dir, sessionID+".json")
}

func ReadHookState(path string) (HookState, bool) {
	var h HookState
	b, err := os.ReadFile(path)
	if err != nil || json.Unmarshal(b, &h) != nil || h.SessionID == "" {
		return h, false
	}
	return h, true
}

// ReadHookStates returns states keyed by session id; dead ones are removed.
func ReadHookStates(dir string) map[string]HookState {
	files, _ := filepath.Glob(filepath.Join(dir, "*.json"))
	out := make(map[string]HookState, len(files))
	for _, f := range files {
		h, ok := ReadHookState(f)
		if !ok {
			continue
		}
		if h.PID != 0 && !Alive(h.PID, h.ProcStart) {
			os.Remove(f)
			continue
		}
		out[h.SessionID] = h
	}
	return out
}

// WriteHookState writes atomically via tmp file + rename.
func WriteHookState(dir string, h HookState) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	b, err := json.Marshal(h)
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".tmp-*")
	if err != nil {
		return err
	}
	if _, err := tmp.Write(b); err != nil {
		tmp.Close()
		os.Remove(tmp.Name())
		return err
	}
	tmp.Close()
	return os.Rename(tmp.Name(), StatePath(dir, h.SessionID))
}
