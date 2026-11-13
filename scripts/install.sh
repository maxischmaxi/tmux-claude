#!/usr/bin/env bash
# Builds bin/tmux-claude with go, or downloads a release binary if go is missing.
set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")/.."

REPO="maxischmaxi/tmux-claude"
rev="$(git rev-parse HEAD 2>/dev/null || echo unknown)"
mkdir -p bin

if command -v go >/dev/null 2>&1; then
	go build -trimpath -o bin/tmux-claude ./cmd/tmux-claude
else
	os="$(uname -s | tr '[:upper:]' '[:lower:]')"
	arch="$(uname -m)"
	case "$arch" in
	x86_64 | amd64) arch=amd64 ;;
	aarch64 | arm64) arch=arm64 ;;
	*) echo "unsupported architecture: $arch" >&2; exit 1 ;;
	esac
	tag="$(git describe --tags --exact-match 2>/dev/null || true)"
	if [ -n "$tag" ]; then
		url="https://github.com/$REPO/releases/download/$tag/tmux-claude_${os}_${arch}"
	else
		url="https://github.com/$REPO/releases/latest/download/tmux-claude_${os}_${arch}"
	fi
	echo "downloading $url"
	curl -fsSL -o bin/tmux-claude.tmp "$url"
	chmod +x bin/tmux-claude.tmp
	mv bin/tmux-claude.tmp bin/tmux-claude
fi

echo "$rev" >bin/.rev
echo "installed $(pwd)/bin/tmux-claude"
