// Package hook handles claude code hook events and records them as state files.
package hook

import (
	"encoding/json"
	"io"
	"os"
	"strings"
	"time"

	"github.com/maxischmaxi/tmux-claude/internal/source"
)

type input struct {
	SessionID        string         `json:"session_id"`
	Cwd              string         `json:"cwd"`
	Event            string         `json:"hook_event_name"`
	ToolName         string         `json:"tool_name"`
	ToolInput        map[string]any `json:"tool_input"`
	Prompt           string         `json:"prompt"`
	NotificationType string         `json:"notification_type"`
	Message          string         `json:"message"`
}

// Run never fails loudly: a hook must not disturb claude.
func Run(stdin io.Reader) {
	b, err := io.ReadAll(io.LimitReader(stdin, 4<<20))
	if err != nil {
		return
	}
	var in input
	if json.Unmarshal(b, &in) != nil || in.SessionID == "" || strings.ContainsAny(in.SessionID, "/\\") {
		return
	}
	dir := source.StateDir()
	path := source.StatePath(dir, in.SessionID)

	if in.Event == "SessionEnd" {
		os.Remove(path)
		return
	}

	h, _ := source.ReadHookState(path)
	h.SessionID = in.SessionID
	if h.PID == 0 || !source.Alive(h.PID, h.ProcStart) {
		h.PID, h.ProcStart = source.FindClaudeAncestor(os.Getppid())
	}
	if p := os.Getenv("TMUX_PANE"); p != "" {
		h.Pane = p
	}
	if in.Cwd != "" {
		h.Cwd = in.Cwd
	}
	now := time.Now().UnixMilli()

	switch in.Event {
	case "SessionStart":
		h.Tool, h.ToolSummary, h.Notification, h.Message = "", "", "", ""
	case "UserPromptSubmit":
		h.Tool, h.ToolSummary, h.Notification, h.Message = "", "", "", ""
		h.Prompt = source.Truncate(strings.Join(strings.Fields(in.Prompt), " "), 200)
	case "PreToolUse":
		h.Tool = in.ToolName
		h.ToolSummary = source.SummarizeTool(in.ToolName, in.ToolInput)
		h.Notification, h.Message = "", ""
	case "PostToolUse", "SubagentStop":
		// keep the last tool so the label does not flicker between calls
	case "Notification":
		// idle reminders carry no new state
		if in.NotificationType == "idle_prompt" {
			return
		}
		h.Notification = in.NotificationType
		h.Message = source.Truncate(in.Message, 120)
	case "Stop":
		h.Tool, h.ToolSummary, h.Notification, h.Message = "", "", "", ""
		h.DoneAt = now
	default:
		return
	}
	h.Event = in.Event
	h.TS = now
	source.WriteHookState(dir, h)
}
