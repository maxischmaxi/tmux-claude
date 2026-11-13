package tui

import "testing"

func TestLoadKeys(t *testing.T) {
	km := loadKeys(func(string) string { return "" })
	for key, want := range map[string]action{"j": actDown, "down": actDown, "k": actUp, " ": actJump, "enter": actJump, "q": actClose, "G": actBottom} {
		if a, _ := km.lookup(key); a != want {
			t.Errorf("default %q: %v, want %v", key, a, want)
		}
	}
	if a, n := km.lookup("3"); a != actJump || n != 3 {
		t.Errorf("number: %v %d", a, n)
	}

	opts := map[string]string{
		"@claude-sidebar-keys-down":   "ctrl+n",
		"@claude-sidebar-keys-close":  "none",
		"@claude-sidebar-keys-jump":   "space o",
		"@claude-sidebar-number-keys": "off",
	}
	km = loadKeys(func(k string) string { return opts[k] })
	cases := map[string]action{"ctrl+n": actDown, "j": actNone, "q": actNone, " ": actJump, "o": actJump, "k": actUp, "3": actNone}
	for key, want := range cases {
		if a, _ := km.lookup(key); a != want {
			t.Errorf("custom %q: %v, want %v", key, a, want)
		}
	}
}
