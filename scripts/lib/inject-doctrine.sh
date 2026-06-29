# shellcheck shell=bash
# inject-doctrine.sh — shared managed-block injector for the sei-internal-skills sync scripts.
#
# Sourced (not executed) by sync-agents.sh and sync-skills.sh. Injects the
# portable operating doctrine (scripts/sei-internal-skills-doctrine.md) as a marker-delimited,
# idempotent managed block into a consuming package's root AGENTS.md, and adds a
# one-line pointer to the package's CLAUDE.md. Re-running replaces only the bytes
# between the markers; the package's own content is never touched.
#
# Three modes (the third positional arg): write (inject), dry-run (report, write
# nothing), check (report drift to stderr and return non-zero, write nothing —
# the CI drift-guard). check is the read-only inverse of write: it passes only
# when a write would be a no-op.
#
# The marker strings are a distribution contract: once a consumer has a block,
# changing the marker text orphans it (a re-sync appends a fresh block instead of
# replacing the old one). They are locked here.
#
# Known limitation: a marker line that appears inside a fenced code block in the
# package's own AGENTS.md (e.g. docs that show the marker syntax) is matched like
# a real marker. The markers are deliberately verbose to make that collision
# implausible; documenting markers in a managed AGENTS.md is unsupported.

DOCTRINE_BEGIN='<!-- BEGIN sei-internal-skills-managed (do not edit; managed by sei-internal-skills sync scripts) -->'
DOCTRINE_END='<!-- END sei-internal-skills-managed -->'
DOCTRINE_POINTER='> Operating doctrine for the sei-internal-skills-synced skills and agents lives in [AGENTS.md](./AGENTS.md).'

# inject_claude_pointer <claude_md_path> <mode:write|dry-run|check>
# Ensures the one-line AGENTS.md pointer is in CLAUDE.md (append-only, idempotent).
# In check mode, returns non-zero when the pointer is absent (drift).
inject_claude_pointer() {
  local claude_md="$1" mode="$2"

  if [[ -f "$claude_md" ]] && grep -qF -- "$DOCTRINE_POINTER" "$claude_md"; then
    return 0
  fi

  case "$mode" in
    check)
      echo "  ✗ drift: $claude_md is missing the AGENTS.md pointer" >&2
      return 1 ;;
    dry-run)
      echo "  (dry-run) would add AGENTS.md pointer to $claude_md"
      return 0 ;;
    *)
      mkdir -p "$(dirname "$claude_md")"
      if [[ ! -f "$claude_md" ]]; then
        printf '%s\n' "$DOCTRINE_POINTER" > "$claude_md"
      else
        printf '\n%s\n' "$DOCTRINE_POINTER" >> "$claude_md"
      fi
      echo "  ✓ AGENTS.md pointer → $claude_md" ;;
  esac
}

# inject_doctrine <target_dir> <doctrine_body_file> <mode:write|dry-run|check>
# write: inject the managed block into <target_dir>/AGENTS.md (creating the file,
#   and the directory, if absent) and the pointer into <target_dir>/CLAUDE.md.
# dry-run: report what write would do; write nothing.
# check: return non-zero if the block or the pointer is out of sync; write nothing.
inject_doctrine() {
  local target_dir="$1" body_file="$2" mode="$3"
  local agents_md="$target_dir/AGENTS.md"

  if [[ ! -f "$body_file" ]]; then
    echo "  ! doctrine source missing, skipping injection: $body_file" >&2
    return 1
  fi

  # The body is wrapped in the markers at write time; a marker line *inside* it
  # would make the block self-terminate and the tail re-append on every sync.
  if grep -qF -- "$DOCTRINE_BEGIN" "$body_file" || grep -qF -- "$DOCTRINE_END" "$body_file"; then
    echo "  ! doctrine source contains a reserved marker line; refusing: $body_file" >&2
    return 1
  fi

  # A malformed prior block (unequal BEGIN/END counts — an interrupted run or a
  # hand-edit) would make the replace pass swallow to EOF and delete package
  # content. Refuse and let a human reconcile rather than destroy content.
  if [[ -f "$agents_md" ]]; then
    # `|| true`: grep -c exits 1 on a zero count, which would abort the function
    # under a `set -e` caller before this guard runs (it still prints "0").
    local nbegin nend
    nbegin=$(grep -cF -- "$DOCTRINE_BEGIN" "$agents_md" || true)
    nend=$(grep -cF -- "$DOCTRINE_END" "$agents_md" || true)
    if [[ "$nbegin" != "$nend" ]]; then
      echo "  ! $agents_md has a malformed sei-internal-skills-managed block (BEGIN=$nbegin END=$nend); fix by hand, then re-sync." >&2
      return 1
    fi
  fi

  # Desired managed block (markers + body) → temp, read by awk verbatim so the
  # doctrine body never passes through shell or awk-string interpretation.
  local block_tmp; block_tmp="$(mktemp)" || return 1
  {
    printf '%s\n' "$DOCTRINE_BEGIN"
    cat "$body_file"
    printf '%s\n' "$DOCTRINE_END"
  } > "$block_tmp"

  # Compute the desired AGENTS.md into a temp in $TMPDIR — the target directory
  # may not exist yet (fresh package), and dry-run/check must write nothing under it.
  local out_tmp; out_tmp="$(mktemp)" || { rm -f "$block_tmp"; return 1; }
  if [[ ! -f "$agents_md" ]]; then
    {
      printf '# Agent Guide\n\n'
      cat "$block_tmp"
    } > "$out_tmp"
  else
    # Single awk pass: replace the first BEGIN..END region with the canonical
    # block, swallowing any well-formed duplicate block; append if no markers
    # exist. Marker match is CRLF-tolerant; a lone END outside a block is kept.
    awk -v begin="$DOCTRINE_BEGIN" -v end="$DOCTRINE_END" -v blockfile="$block_tmp" '
      BEGIN { inblk = 0; replaced = 0 }
      { cmp_line = $0; sub(/\r$/, "", cmp_line) }
      cmp_line == begin {
        inblk = 1
        if (!replaced) {
          while ((getline line < blockfile) > 0) print line
          close(blockfile)
          replaced = 1
        }
        next
      }
      inblk == 1 && cmp_line == end { inblk = 0; next }
      inblk == 1 { next }
      { print }
      END {
        if (!replaced) {
          print ""
          while ((getline line < blockfile) > 0) print line
          close(blockfile)
        }
      }
    ' "$agents_md" > "$out_tmp"
  fi

  # Is the on-disk AGENTS.md already byte-identical to the desired output?
  local agents_in_sync=0
  if [[ -f "$agents_md" ]] && cmp -s "$out_tmp" "$agents_md"; then
    agents_in_sync=1
  fi

  case "$mode" in
    check)
      local drift=0
      if [[ "$agents_in_sync" -eq 0 ]]; then
        if [[ -f "$agents_md" ]]; then
          echo "  ✗ drift: $agents_md is out of sync with $body_file (run: make sync-doctrine-self)" >&2
        else
          echo "  ✗ drift: $agents_md is missing the sei-internal-skills-managed block (run: make sync-doctrine-self)" >&2
        fi
        drift=1
      fi
      rm -f "$block_tmp" "$out_tmp"
      inject_claude_pointer "$target_dir/CLAUDE.md" check || drift=1
      return "$drift" ;;
    dry-run)
      if [[ "$agents_in_sync" -eq 0 ]]; then
        if [[ -f "$agents_md" ]]; then
          echo "  (dry-run) would update sei-internal-skills-managed block in $agents_md"
        else
          echo "  (dry-run) would create $agents_md with the sei-internal-skills-managed block"
        fi
      fi
      rm -f "$block_tmp" "$out_tmp"
      inject_claude_pointer "$target_dir/CLAUDE.md" dry-run ;;
    *)
      if [[ "$agents_in_sync" -eq 1 ]]; then
        rm -f "$block_tmp" "$out_tmp"          # byte-identical — true no-op on re-run
      else
        mkdir -p "$target_dir"
        # Stage in a temp in the SAME directory as the destination so the rename
        # is an atomic, single-filesystem operation.
        local final_tmp; final_tmp="$(mktemp "${target_dir%/}/.AGENTS.md.XXXXXX")" || { rm -f "$block_tmp" "$out_tmp"; return 1; }
        cat "$out_tmp" > "$final_tmp"
        mv "$final_tmp" "$agents_md"
        rm -f "$block_tmp" "$out_tmp"
        echo "  ✓ doctrine block → $agents_md"
      fi
      inject_claude_pointer "$target_dir/CLAUDE.md" write ;;
  esac
}
