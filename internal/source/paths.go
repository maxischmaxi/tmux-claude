package source

import (
	"os"
	"path/filepath"
)

// SessionsDir is where claude code registers running instances (internal format).
func SessionsDir() string {
	if d := os.Getenv("CLAUDE_CONFIG_DIR"); d != "" {
		return filepath.Join(d, "sessions")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".claude", "sessions")
}

// CacheDir holds state written by our claude hooks and the sidebar.
func CacheDir() string {
	if d := os.Getenv("TMUX_CLAUDE_CACHE"); d != "" {
		return d
	}
	base, err := os.UserCacheDir()
	if err != nil {
		home, _ := os.UserHomeDir()
		base = filepath.Join(home, ".cache")
	}
	return filepath.Join(base, "tmux-claude")
}

func StateDir() string { return filepath.Join(CacheDir(), "state") }

func SeenFile() string { return filepath.Join(CacheDir(), "seen.json") }
