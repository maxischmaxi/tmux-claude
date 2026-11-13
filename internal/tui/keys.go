package tui

import "strings"

type action int

const (
	actNone action = iota
	actDown
	actUp
	actTop
	actBottom
	actJump
	actRefresh
	actClose
)

// keyOptions maps tmux options to actions and their default keys.
// Values are space separated bubbletea key names, "none" disables an action.
var keyOptions = []struct {
	option string
	act    action
	def    string
}{
	{"@claude-sidebar-keys-down", actDown, "j down"},
	{"@claude-sidebar-keys-up", actUp, "k up"},
	{"@claude-sidebar-keys-top", actTop, "g home"},
	{"@claude-sidebar-keys-bottom", actBottom, "G end"},
	{"@claude-sidebar-keys-jump", actJump, "enter l space"},
	{"@claude-sidebar-keys-refresh", actRefresh, "r"},
	{"@claude-sidebar-keys-close", actClose, "q ctrl+c"},
}

type keymap struct {
	actions map[string]action
	numbers bool // 1-9 jump to an entry
}

// loadKeys builds the keymap; get returns an option value or "" if unset.
func loadKeys(get func(string) string) keymap {
	km := keymap{actions: map[string]action{}, numbers: get("@claude-sidebar-number-keys") != "off"}
	for _, o := range keyOptions {
		v := strings.TrimSpace(get(o.option))
		if v == "" {
			v = o.def
		}
		if v == "none" {
			continue
		}
		for _, k := range strings.Fields(v) {
			km.actions[normalizeKey(k)] = o.act
		}
	}
	return km
}

// normalizeKey maps config names to bubbletea's key strings.
func normalizeKey(k string) string {
	switch strings.ToLower(k) {
	case "space":
		return " "
	case "return":
		return "enter"
	case "escape":
		return "esc"
	}
	return k
}

func (km keymap) lookup(key string) (action, int) {
	if a, ok := km.actions[key]; ok {
		return a, 0
	}
	if km.numbers && len(key) == 1 && key[0] >= '1' && key[0] <= '9' {
		return actJump, int(key[0]-'1') + 1
	}
	return actNone, 0
}
