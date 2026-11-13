package source

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

// ClaudeSession mirrors ~/.claude/sessions/<pid>.json. The format is internal
// to claude code, so every field is optional.
type ClaudeSession struct {
	PID             int    `json:"pid"`
	SessionID       string `json:"sessionId"`
	Cwd             string `json:"cwd"`
	Kind            string `json:"kind"`
	Tmux            string `json:"tmux"`
	Name            string `json:"name"`
	Status          string `json:"status"`
	WaitingFor      string `json:"waitingFor"`
	ProcStart       string `json:"procStart"`
	StartedAt       int64  `json:"startedAt"`
	StatusUpdatedAt int64  `json:"statusUpdatedAt"`
}

// PaneID extracts "%14" from "session:@12.%14".
func (s ClaudeSession) PaneID() string {
	i := strings.LastIndexByte(s.Tmux, '%')
	if i < 0 {
		return ""
	}
	return s.Tmux[i:]
}

// ReadSessions returns all live sessions registered in dir.
func ReadSessions(dir string) []ClaudeSession {
	files, _ := filepath.Glob(filepath.Join(dir, "*.json"))
	var out []ClaudeSession
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		var s ClaudeSession
		if json.Unmarshal(b, &s) != nil || s.PID == 0 {
			continue
		}
		if s.Kind != "" && s.Kind != "interactive" {
			continue
		}
		if !Alive(s.PID, s.ProcStart) {
			continue
		}
		out = append(out, s)
	}
	return out
}
