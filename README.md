# tmux-claude

A tmux sidebar that lists every running [Claude Code](https://claude.com/claude-code) instance
together with its live state. Click an entry (or press its number) to jump straight to its
session, window and pane.

```
 claude                  1 new / 3

 ─ work ──────────────────────────
 1 ! api                       2.0
     needs approval Bash        4s
 2 ◆ frontend                  3.1
     editing Header.tsx        12s

 ─ dotfiles ──────────────────────
 3 ● dotfiles                  0.0
     done awaiting input        2m
```

| icon | meaning |
|---|---|
| `!` red | waiting for a permission prompt or an answer |
| `◆` yellow | thinking, editing files or running commands |
| `◆` blue | reading or researching |
| `◆` magenta | running subagents |
| `●` green | finished and not looked at yet |
| `○` gray | idle, awaiting input |

There is a single global sidebar: it follows you into whatever window or session you switch to.

## Requirements

- tmux 3.2 or newer with `set -g mouse on` for clicking
- Linux or macOS
- Claude Code
- Go 1.24+ to build from source (optional, a prebuilt binary is downloaded otherwise)

## Installation

### With [tpm](https://github.com/tmux-plugins/tpm)

Add the plugin to your `tmux.conf`:

```tmux
set -g @plugin 'maxischmaxi/tmux-claude'
```

Press `prefix + I` to install. On first load the plugin builds `bin/tmux-claude` with Go, or
downloads the matching release binary when Go is not available. This happens in the background
and tmux shows `tmux-claude: ready` when it is done. After `prefix + U` (update) the binary is
rebuilt automatically.

### Manually

```sh
git clone https://github.com/maxischmaxi/tmux-claude ~/.tmux/plugins/tmux-claude
~/.tmux/plugins/tmux-claude/scripts/install.sh
```

and add to your `tmux.conf`:

```tmux
run-shell ~/.tmux/plugins/tmux-claude/tmux-claude.tmux
```

### Claude Code hooks (recommended)

The sidebar works without any Claude Code configuration: Claude Code keeps a small registry of
its running instances (`~/.claude/sessions/*.json`) with their tmux pane and a coarse state
(busy / idle / waiting).

For detailed states (which tool is running, permission prompts, finished turns) register the
hooks. Use the binary inside your plugin directory, e.g. with tpm:

```sh
~/.tmux/plugins/tmux-claude/bin/tmux-claude install-hooks --print   # only show the snippet
~/.tmux/plugins/tmux-claude/bin/tmux-claude install-hooks           # merge into ~/.claude/settings.json
```

`install-hooks` keeps your existing settings and hooks untouched, writes a backup next to the
file and is safe to run repeatedly. `uninstall-hooks` removes the entries again. Running
Claude Code sessions pick up the hooks without a restart. `CLAUDE_CONFIG_DIR` is respected.

### Status line (optional)

Show a compact summary (`! 1 ● 2 ◆ 3`) in your status bar:

```tmux
set -ag status-right "#(~/.tmux/plugins/tmux-claude/bin/tmux-claude status)"
```

## Usage

| key | action |
|---|---|
| `prefix + A` | toggle the sidebar |
| `@claude-sidebar-root-key` | toggle without prefix, if configured |
| click, `enter`, `l`, `space` | jump to the selected instance |
| `1` – `9` | jump to instance n |
| `j` / `k`, mouse wheel | move the selection |
| `g` / `G` | first / last entry |
| `q` | close the sidebar |

The selected entry also shows the last prompt sent to that instance. An instance stays marked as
`done` until its pane has been focused once.

`tmux-claude list` prints all instances on the command line.

## Options

Set options before the plugin is loaded (before the tpm `run` line).

```tmux
set -g @claude-sidebar-key 'A'      # toggle key after prefix, 'none' to disable
set -g @claude-sidebar-root-key 'M-s' # toggle key without prefix (unset by default)
set -g @claude-sidebar-width 34     # sidebar width in columns
```

## How it works

- **Discovery**: `~/.claude/sessions/<pid>.json` (an internal Claude Code format, parsed
  leniently), then the hook state files, then any tmux pane whose foreground command is `claude`.
- **Liveness**: pid plus process start time from `/proc` so reused pids are ignored
  (`kill -0` on macOS).
- **Hooks** write `~/.cache/tmux-claude/state/<session_id>.json` atomically. The sidebar watches
  both directories and refreshes every two seconds for the durations.
- **Following**: the sidebar pane is moved with `join-pane` from the tmux hooks
  `session-window-changed`, `client-session-changed` and `after-select-window`
  (hook index `4711`, removed again when the sidebar closes).

## Commands

```
tmux-claude sidebar              run the sidebar in the current pane
tmux-claude toggle [window]      open or close the global sidebar
tmux-claude follow <window>      move the sidebar into a window (used by tmux hooks)
tmux-claude list                 print all instances with their state
tmux-claude status               summary for the tmux status line
tmux-claude hook                 Claude Code hook handler (reads the event from stdin)
tmux-claude install-hooks        register the hooks (--print to only show them)
tmux-claude uninstall-hooks      remove the hooks
```

## Development

```sh
go test ./...
scripts/install.sh   # rebuild bin/tmux-claude
```

Releases are built by GitHub Actions when a `v*` tag is pushed.

## License

[MIT](LICENSE)
