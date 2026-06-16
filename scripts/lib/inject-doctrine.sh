# inject-doctrine.sh — shared managed-block injector for the Tide sync scripts.
#
# Sourced (not executed) by sync-agents.sh and sync-skills.sh. Injects the
# portable operating doctrine (scripts/tide-doctrine.md) as a marker-delimited,
# idempotent managed block into a consuming package's root AGENTS.md, and adds a
# one-line pointer to the package's CLAUDE.md. Re-running replaces only the bytes
# between the markers; the package's own content is never touched.
#
# The marker strings are a distribution contract: once a consumer has a block,
# changing the marker text orphans it (a re-sync appends a fresh block instead of
# replacing the old one). They are locked here.
#
# Known limitation: a marker line that appears inside a fenced code block in the
# package's own AGENTS.md (e.g. docs that show the marker syntax) is matched like
# a real marker. The markers are deliberately verbose to make that collision
# implausible; documenting markers in a managed AGENTS.md is unsupported.

DOCTRINE_BEGIN='<!-- BEGIN tide-managed (do not edit; managed by Tide sync scripts) -->'
DOCTRINE_END='<!-- END tide-managed -->'
DOCTRINE_POINTER='> Operating doctrine for the Tide-synced skills and agents lives in [AGENTS.md](./AGENTS.md).'

# inject_claude_pointer <claude_md_path> <dry_run:true|false>
# Adds the one-line AGENTS.md pointer to CLAUDE.md, append-only, idempotent.
inject_claude_pointer() {
  local claude_md="$1" dry_run="$2"

  if [[ -f "$claude_md" ]] && grep -qF -- "$DOCTRINE_POINTER" "$claude_md"; then
    return 0
  fi

  if [[ "$dry_run" == true ]]; then
    echo "  (dry-run) would add AGENTS.md pointer to $claude_md"
    return 0
  fi

  mkdir -p "$(dirname "$claude_md")"
  if [[ ! -f "$claude_md" ]]; then
    printf '%s\n' "$DOCTRINE_POINTER" > "$claude_md"
  else
    printf '\n%s\n' "$DOCTRINE_POINTER" >> "$claude_md"
  fi
  echo "  ✓ AGENTS.md pointer → $claude_md"
}

# inject_doctrine <target_dir> <doctrine_body_file> <dry_run:true|false>
# Injects the managed block into <target_dir>/AGENTS.md (creating the file, and
# the directory, if absent) and the pointer into <target_dir>/CLAUDE.md.
inject_doctrine() {
  local target_dir="$1" body_file="$2" dry_run="$3"
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
    local nbegin nend
    nbegin=$(grep -cF -- "$DOCTRINE_BEGIN" "$agents_md")
    nend=$(grep -cF -- "$DOCTRINE_END" "$agents_md")
    if [[ "$nbegin" != "$nend" ]]; then
      echo "  ! $agents_md has a malformed tide-managed block (BEGIN=$nbegin END=$nend); fix by hand, then re-sync." >&2
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
  # may not exist yet (fresh package), and a dry-run must write nothing under it.
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

  if [[ -f "$agents_md" ]] && cmp -s "$out_tmp" "$agents_md"; then
    rm -f "$block_tmp" "$out_tmp"           # byte-identical — true no-op on re-run
  elif [[ "$dry_run" == true ]]; then
    if [[ -f "$agents_md" ]]; then
      echo "  (dry-run) would update tide-managed block in $agents_md"
    else
      echo "  (dry-run) would create $agents_md with the tide-managed block"
    fi
    rm -f "$block_tmp" "$out_tmp"
  else
    mkdir -p "$target_dir"
    # Stage in a temp in the SAME directory as the destination so the rename is
    # an atomic, single-filesystem operation.
    local final_tmp; final_tmp="$(mktemp "${target_dir%/}/.AGENTS.md.XXXXXX")" || { rm -f "$block_tmp" "$out_tmp"; return 1; }
    cat "$out_tmp" > "$final_tmp"
    mv "$final_tmp" "$agents_md"
    rm -f "$block_tmp" "$out_tmp"
    echo "  ✓ doctrine block → $agents_md"
  fi

  inject_claude_pointer "$target_dir/CLAUDE.md" "$dry_run"
}
