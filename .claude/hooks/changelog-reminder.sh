#!/bin/sh
# PostToolUse hook: remind Claude to record documentation changes in CHANGELOG.md.
#
# Fires after a tool run in which any file under docs/ was actually modified, and
# injects that reminder back into the model's context.
#
# Detection is by file modification time, not by inspecting the tool's arguments:
#
#   - it catches a change however it was made — the Edit and Write tools, a scripted
#     Bash heredoc, sed -i, a redirect, a generator;
#   - it stays silent for reads, which change no file;
#   - it needs no knowledge of which argument of which tool holds a path.
#
# "Modified" means newer than a marker file touched at the end of each run, so a
# change is reported once and not repeated. It stays silent when CHANGELOG.md was
# part of the same change.
#
# POSIX sh and coreutils only — no python, no jq — so it runs wherever a shell does.

set -u

MAX_LISTED=8

root=${CLAUDE_PROJECT_DIR:-}
if [ -z "$root" ]; then
    root=$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)
fi

docs="$root/docs"
[ -d "$docs" ] || exit 0

# One marker per project, keyed by a hash of its path.
key=$(printf '%s' "$root" | cksum | cut -d' ' -f1)
marker="${TMPDIR:-/tmp}/claude-changelog-hook-$key"

# First run in this environment: start the clock, say nothing.
if [ ! -e "$marker" ]; then
    : > "$marker" 2>/dev/null || exit 0
    exit 0
fi

changed=$(find "$docs" -type f -newer "$marker" 2>/dev/null)
[ -n "$changed" ] || exit 0

# The changelog was part of this same change — nothing to remind about.
# `find -newer` rather than `[ -nt ]`, which is a shell extension and not POSIX.
if [ -n "$(find "$root" -maxdepth 1 -name CHANGELOG.md -newer "$marker" 2>/dev/null)" ]; then
    : > "$marker" 2>/dev/null
    exit 0
fi

# Report each change once.
: > "$marker" 2>/dev/null

count=$(printf '%s\n' "$changed" | wc -l | tr -d ' ')
names=$(printf '%s\n' "$changed" \
    | sed "s|^$root/||" \
    | sort \
    | head -n "$MAX_LISTED" \
    | sed 's/\\/\\\\/g; s/"/\\"/g' \
    | paste -sd, - \
    | sed 's/,/, /g')
if [ "$count" -gt "$MAX_LISTED" ]; then
    names="$names, and $((count - MAX_LISTED)) more"
fi
if [ "$count" -eq 1 ]; then was="was"; else was="were"; fi

cat <<JSON
{
  "hookSpecificOutput": {
    "hookEventName": "PostToolUse",
    "additionalContext": "$names under docs/ $was just modified. Per CLAUDE.md, documentation changes must be recorded in CHANGELOG.md before this turn ends: start a new entry at the top, headed \`YYYY-MM-DD HH:MM\`. KEEP IT SHORT — a line or two per change: what changed, and why in a clause. If an explanation needs a paragraph it belongs in the ADR or the document, and the entry points there instead. Never edit an entry that is already committed. If this is part of a larger change you are still making, write the entry once at the end rather than after each file."
  }
}
JSON
