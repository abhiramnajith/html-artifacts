#!/usr/bin/env sh
# install.sh — copy a per-agent adapter (plus the canonical CORE.md) into the
# agent's skills directory.
#
# Deliberately readable and dumb: no `curl | bash`, no network calls beyond the
# `git clone` you already ran to get here. Re-running it is idempotent — it
# overwrites the installed adapter in place.
#
# Usage:
#   ./install.sh --agent claude [--local] [--with-binary]
#
#   --agent        claude | codex | opencode | copilot   (v1 implements: claude)
#   --local        install into ./.claude/skills/ instead of ~/.claude/skills/
#   --with-binary  eagerly fetch the server binary now, instead of waiting for
#                  first use (ensure-server.sh fetches it lazily either way)

set -eu

AGENT=""
LOCAL=0
WITH_BINARY=0

usage() {
	echo "usage: ./install.sh --agent claude|codex|opencode|copilot [--local] [--with-binary]" >&2
}

while [ $# -gt 0 ]; do
	case "$1" in
	--agent)
		shift
		[ $# -gt 0 ] || { echo "error: --agent needs a value" >&2; usage; exit 2; }
		AGENT="$1"
		;;
	--agent=*)
		AGENT="${1#--agent=}"
		;;
	--local)
		LOCAL=1
		;;
	--with-binary)
		WITH_BINARY=1
		;;
	-h | --help)
		usage
		exit 0
		;;
	*)
		echo "error: unknown argument: $1" >&2
		usage
		exit 2
		;;
	esac
	shift
done

if [ -z "$AGENT" ]; then
	echo "error: --agent is required" >&2
	usage
	exit 2
fi

# Resolve the repo root (directory containing this script) so the script works
# regardless of the caller's working directory.
REPO_ROOT=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
CORE="$REPO_ROOT/instructions/CORE.md"

# Map the agent name to its adapter directory. Only 'claude' is implemented in
# v1; the other directories exist to prove the adapter pattern.
case "$AGENT" in
claude)
	ADAPTER_NAME="claude-code"
	;;
codex | opencode | copilot | copilot-cli)
	echo "error: the '$AGENT' adapter is not implemented yet (v1 ships Claude Code only)." >&2
	echo "       The adapter directory exists to prove the pattern; add real content when needed." >&2
	exit 1
	;;
*)
	echo "error: unknown agent '$AGENT' (expected: claude|codex|opencode|copilot)" >&2
	exit 2
	;;
esac

ADAPTER_DIR="$REPO_ROOT/adapters/$ADAPTER_NAME"

if [ ! -f "$ADAPTER_DIR/SKILL.md" ]; then
	echo "error: no SKILL.md found in $ADAPTER_DIR" >&2
	exit 1
fi

if [ "$LOCAL" -eq 1 ]; then
	SKILLS_ROOT="./.claude/skills"
else
	SKILLS_ROOT="$HOME/.claude/skills"
fi

TARGET="$SKILLS_ROOT/html-artifacts"

mkdir -p "$TARGET"
# Copy the thin adapter, the canonical CORE.md it defers to, the
# ensure-server.sh bootstrap script CORE.md shells out to, and the base.html
# template authoring needs. Mermaid is embedded in the server binary now (not
# copied here) — ensure-server.sh fetches that binary lazily on first use.
# Overwriting in place keeps re-runs idempotent.
cp "$ADAPTER_DIR/SKILL.md" "$TARGET/SKILL.md"
cp "$CORE" "$TARGET/CORE.md"
cp "$REPO_ROOT/scripts/ensure-server.sh" "$TARGET/ensure-server.sh"
chmod +x "$TARGET/ensure-server.sh"
mkdir -p "$TARGET/templates"
cp "$REPO_ROOT/instructions/templates/base.html" "$TARGET/templates/base.html"

echo "installed html-artifacts adapter for '$AGENT' -> $TARGET"

if [ "$WITH_BINARY" -eq 1 ]; then
	echo "fetching server binary now (--with-binary)..."
	if HTML_ARTIFACTS_HOME="$HOME/.html-artifacts" "$REPO_ROOT/scripts/ensure-server.sh" >/dev/null; then
		echo "server binary ready."
	else
		echo "warning: failed to fetch the server binary; it will be fetched on first use instead." >&2
	fi
else
	echo "note: the server binary is fetched automatically on first use (run with --with-binary to fetch it now)."
fi
