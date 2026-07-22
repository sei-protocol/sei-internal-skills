#!/usr/bin/env bash
# install.sh — one command to install/refresh the sei-internal-skills agentic toolkit into
# ~/.claude, even if you've never cloned sei-internal-skills. Safe to run over the wire.
#
# sei-internal-skills is an INTERNAL GitHub repo, so the fetch needs auth — `gh` provides it.
# Run straight over the wire:
#
#   gh api repos/sei-protocol/sei-internal-skills/contents/scripts/install.sh \
#     -H 'Accept: application/vnd.github.raw' | bash
#
# Or, once cloned, just `make update` from the repo. Override the local checkout
# location with SEI_INTERNAL_SKILLS_HOME (default: ~/.sei-internal-skills).
#
# What it does: clone sei-internal-skills if absent (or fast-forward it if present), then run
# `make sync-all` to sync ALL skills/agents into ~/.claude and verify the
# catalog. Idempotent — safe to re-run any time.

set -euo pipefail

REPO="sei-protocol/sei-internal-skills"
SEI_INTERNAL_SKILLS_HOME="${SEI_INTERNAL_SKILLS_HOME:-$HOME/.sei-internal-skills}"

have() { command -v "$1" >/dev/null 2>&1; }

clone_repo() {
  if have gh; then
    gh repo clone "$REPO" "$SEI_INTERNAL_SKILLS_HOME"
  else
    echo "→ gh not found; falling back to git clone (needs SSH or credential-helper auth for an internal repo)" >&2
    git clone "git@github.com:$REPO.git" "$SEI_INTERNAL_SKILLS_HOME" 2>/dev/null \
      || git clone "https://github.com/$REPO.git" "$SEI_INTERNAL_SKILLS_HOME"
  fi
}

if [ -d "$SEI_INTERNAL_SKILLS_HOME/.git" ]; then
  echo "→ updating existing sei-internal-skills checkout at $SEI_INTERNAL_SKILLS_HOME"
  git -C "$SEI_INTERNAL_SKILLS_HOME" pull --ff-only
elif [ -e "$SEI_INTERNAL_SKILLS_HOME" ]; then
  echo "Error: $SEI_INTERNAL_SKILLS_HOME exists but is not a sei-internal-skills git checkout." >&2
  echo "       Move it aside or set SEI_INTERNAL_SKILLS_HOME=<free path> and re-run." >&2
  exit 1
else
  echo "→ cloning $REPO into $SEI_INTERNAL_SKILLS_HOME"
  clone_repo
fi

# Single sync entrypoint, shared with `make update`. No git pull here — the
# clone/fast-forward above already made the checkout current.
make -C "$SEI_INTERNAL_SKILLS_HOME" sync-all

echo "✓ sei-internal-skills toolkit synced into ~/.claude (checkout: $SEI_INTERNAL_SKILLS_HOME)"
echo "  Re-run any time with:  make -C \"$SEI_INTERNAL_SKILLS_HOME\" update"
