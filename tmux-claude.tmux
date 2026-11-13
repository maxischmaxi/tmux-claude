#!/usr/bin/env bash
# tpm entry point: makes sure the binary matches the checkout and binds the toggle key.
DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BIN="$DIR/bin/tmux-claude"

rev="$(git -C "$DIR" rev-parse HEAD 2>/dev/null || echo unknown)"
if [ ! -x "$BIN" ] || [ "$(cat "$DIR/bin/.rev" 2>/dev/null)" != "$rev" ]; then
	# build in the background so tmux startup is not blocked
	(
		if "$DIR/scripts/install.sh" >"$DIR/install.log" 2>&1; then
			tmux display-message "tmux-claude: ready"
		else
			tmux display-message "tmux-claude: install failed, see $DIR/install.log"
		fi
	) &
fi

key="$(tmux show -gqv @claude-sidebar-key)"
tmux bind-key "${key:-A}" run-shell "'$BIN' toggle '#{window_id}'"
