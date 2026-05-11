#!/usr/bin/env bash
# scaffold.sh — Create the directory tree for a new skill.
#
# Usage:
#   scaffold.sh --name <kebab-case-name> --scope <project|user> --shape <discipline|technique|pattern|reference|procedural>
#
# Behavior:
#   - Resolves target path:
#       project → $(git rev-parse --show-toplevel)/.claude/skills/<name>/
#       user    → $HOME/.claude/skills/<name>/
#   - Refuses if target exists and is non-empty (exit 2).
#   - Refuses if name collides with the protected list (exit 3).
#   - Creates the directory tree per the shape:
#       discipline / technique / pattern / reference  → flat (Obra style)
#       procedural                                    → full (Tide SKILL-TEMPLATE.md)
#   - Populates stubs for SKILL.md (from drafted state) and evals.json.
#   - Echoes the resolved path on success (exit 0).

set -euo pipefail

NAME=""
SCOPE=""
SHAPE=""
DRAFT_DIR=""   # path to state/run-<ts>/draft/ — used to copy the prepared SKILL.md and evals.json into place

PROTECTED=("coral" "council" "design" "issue" "bugbash" "author-skill" "chaos-suite" "harbor-dev")

die() { printf 'scaffold.sh: %s\n' "$1" >&2; exit "${2:-1}"; }

while [[ $# -gt 0 ]]; do
  case "$1" in
    --name)   NAME="$2"; shift 2 ;;
    --scope)  SCOPE="$2"; shift 2 ;;
    --shape)  SHAPE="$2"; shift 2 ;;
    --draft-dir) DRAFT_DIR="$2"; shift 2 ;;
    *) die "unknown flag: $1" 1 ;;
  esac
done

# --- Validate inputs ---
[[ -n "$NAME" ]]  || die "--name is required" 1
[[ -n "$SCOPE" ]] || die "--scope is required (project|user)" 1
[[ -n "$SHAPE" ]] || die "--shape is required (discipline|technique|pattern|reference|procedural)" 1

# Kebab-case check
if ! [[ "$NAME" =~ ^[a-z][a-z0-9-]{0,31}$ ]]; then
  die "name must be kebab-case, start with a letter, and be ≤ 32 chars: got '$NAME'" 1
fi

# Protected list collision
for p in "${PROTECTED[@]}"; do
  if [[ "$NAME" == "$p" ]]; then
    die "name '$NAME' collides with a protected canonical skill; refusing" 3
  fi
done

# Resolve target path
case "$SCOPE" in
  project)
    REPO_ROOT="$(git rev-parse --show-toplevel 2>/dev/null)" || die "not in a git repo; cd to your target repo or pass --scope user" 1
    TARGET="$REPO_ROOT/.claude/skills/$NAME"
    ;;
  user)
    TARGET="$HOME/.claude/skills/$NAME"
    ;;
  *)
    die "--scope must be project or user, got '$SCOPE'" 1
    ;;
esac

# Non-empty refusal
if [[ -d "$TARGET" ]] && [[ -n "$(ls -A "$TARGET" 2>/dev/null || true)" ]]; then
  die "target already exists and is non-empty: $TARGET" 2
fi

# --- Create directory tree ---
mkdir -p "$TARGET/references"

case "$SHAPE" in
  procedural)
    mkdir -p "$TARGET/scripts" "$TARGET/evals" "$TARGET/state"
    : > "$TARGET/state/.gitkeep"
    cat > "$TARGET/scripts/README.md" <<'EOF'
# Scripts

Deterministic steps used by this skill. Each script is debuggable standalone.
EOF
    ;;
  discipline|technique|pattern|reference)
    # Flat Obra-style: SKILL.md + references/ only. Evals optional but recommended.
    mkdir -p "$TARGET/evals"
    ;;
  *)
    die "--shape must be one of: discipline, technique, pattern, reference, procedural" 1
    ;;
esac

# --- Populate from draft ---
if [[ -n "$DRAFT_DIR" ]]; then
  [[ -d "$DRAFT_DIR" ]] || die "draft dir does not exist: $DRAFT_DIR" 1
  [[ -f "$DRAFT_DIR/SKILL.md" ]]   && cp "$DRAFT_DIR/SKILL.md"   "$TARGET/SKILL.md"
  [[ -f "$DRAFT_DIR/evals.json" ]] && cp "$DRAFT_DIR/evals.json" "$TARGET/evals/evals.json"
  # Copy any reference files the draft prepared.
  if [[ -d "$DRAFT_DIR/references" ]]; then
    cp -R "$DRAFT_DIR/references/." "$TARGET/references/"
  fi
fi

# Stub SKILL.md if nothing was copied — caller should not hit this path in normal use.
if [[ ! -f "$TARGET/SKILL.md" ]]; then
  cat > "$TARGET/SKILL.md" <<EOF
---
name: $NAME
description: "TODO — draft from author-skill's Step 5 (description-crafting). Use 'Use when ...' style; no workflow summary."
---

# $NAME

TODO — body drafted by author-skill (Step 8). Do not ship this stub.
EOF
fi

# Stub evals.json if the shape has an evals/ dir and nothing was copied.
if [[ -d "$TARGET/evals" ]] && [[ ! -f "$TARGET/evals/evals.json" ]]; then
  cat > "$TARGET/evals/evals.json" <<EOF
{
  "skill": "$NAME",
  "version": "1",
  "evals": []
}
EOF
fi

printf '%s\n' "$TARGET"
