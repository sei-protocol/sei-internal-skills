#!/usr/bin/env bash
# install.sh — one command to install/refresh the Tide agentic toolkit into
# ~/.claude, even if you've never cloned Tide. Safe to run over the wire.
#
# Tide is an INTERNAL GitHub repo, so the fetch needs auth — `gh` provides it.
# Run straight over the wire:
#
#   gh api repos/sei-protocol/Tide/contents/scripts/install.sh \
#     -H 'Accept: application/vnd.github.raw' | bash
#
# Or, once cloned, just `make update` from the repo. Override the local checkout
# location with TIDE_HOME (default: ~/.tide).
#
# What it does: clone Tide if absent (or fast-forward it if present), then run
# `make sync-all` to sync ALL skills/agents into ~/.claude and verify the
# catalog. Idempotent — safe to re-run any time.

set -euo pipefail

REPO="sei-protocol/Tide"
TIDE_HOME="${TIDE_HOME:-$HOME/.tide}"

have() { command -v "$1" >/dev/null 2>&1; }

clone_repo() {
  if have gh; then
    gh repo clone "$REPO" "$TIDE_HOME"
  else
    echo "→ gh not found; falling back to git clone (needs SSH or credential-helper auth for an internal repo)" >&2
    git clone "git@github.com:$REPO.git" "$TIDE_HOME" 2>/dev/null \
      || git clone "https://github.com/$REPO.git" "$TIDE_HOME"
  fi
}

if [ -d "$TIDE_HOME/.git" ]; then
  echo "→ updating existing Tide checkout at $TIDE_HOME"
  git -C "$TIDE_HOME" pull --ff-only
elif [ -e "$TIDE_HOME" ]; then
  echo "Error: $TIDE_HOME exists but is not a Tide git checkout." >&2
  echo "       Move it aside or set TIDE_HOME=<free path> and re-run." >&2
  exit 1
else
  echo "→ cloning $REPO into $TIDE_HOME"
  clone_repo
fi

# Single sync entrypoint, shared with `make update`. No git pull here — the
# clone/fast-forward above already made the checkout current.
make -C "$TIDE_HOME" sync-all

echo "✓ Tide toolkit synced into ~/.claude (checkout: $TIDE_HOME)"
echo "  Re-run any time with:  make -C $TIDE_HOME update"
