package hook

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const settings = `{
  "zeta": 1,
  "hooks": {
    "Stop": [
      {
        "hooks": [
          {
            "type": "command",
            "command": "notify.sh"
          }
        ]
      }
    ]
  },
  "alpha": "<b>"
}
`

func TestInstallRoundtrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	os.WriteFile(path, []byte(settings), 0o600)

	changed, err := Install(path, "/opt/tmux-claude", false)
	if err != nil || !changed {
		t.Fatalf("install: %v %v", changed, err)
	}
	b, _ := os.ReadFile(path)
	if !json.Valid(b) {
		t.Fatal("invalid json")
	}
	s := string(b)
	if strings.Index(s, `"zeta"`) > strings.Index(s, `"alpha"`) {
		t.Error("key order changed")
	}
	if !strings.Contains(s, `"<b>"`) || !strings.Contains(s, "notify.sh") {
		t.Error("existing content altered")
	}
	for _, ev := range Events {
		if !strings.Contains(s, `"`+ev+`"`) {
			t.Errorf("missing %s", ev)
		}
	}

	if changed, _ := Install(path, "/opt/tmux-claude", false); changed {
		t.Error("second install should be a no-op")
	}
	if changed, err := Install(path, "/opt/tmux-claude", true); err != nil || !changed {
		t.Fatalf("uninstall: %v %v", changed, err)
	}
	b, _ = os.ReadFile(path)
	if string(b) != settings {
		t.Errorf("roundtrip differs:\n%s", b)
	}
}
