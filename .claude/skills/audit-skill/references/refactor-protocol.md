# Refactor Protocol — Phase 2

How the optional refactor pass works after an audit completes. The user opts in with `--apply` or by confirming after seeing the audit report.

## When this fires

After Phase 1 (audit) completes and the user explicitly opts into Phase 2. Default is audit-only — no edits unless asked.

## Why a separate phase

Audit is read-only; refactor is destructive (it modifies the target skill's files). Keeping them separate phases means:

1. **Audit results are durable** — the report exists whether or not you choose to fix anything.
2. **Refactor is gated** — you see the findings, decide which to address, then opt in.
3. **Time-shifted remediation** — audit today, refactor tomorrow. The findings report is the handoff.

## Pre-conditions for Phase 2

Phase 2 refuses to run if any of:

- Phase 1 didn't complete (no findings file in `state/run-<ts>/`).
- Target skill is on the protected list AND `--override-protected` was not passed.
- The target skill has uncommitted local changes (refactor on a dirty tree is high-risk).

## Per-finding refactor proposals

For each finding the user wants to address:

1. **Generate the tightening proposal.** The orchestrator drafts a unified diff against the relevant file (SKILL.md, a reference, a script). The diff is grounded in the catalog rule's recommendation and the finding's evidence.

2. **Save the proposal.** Write to `state/run-<ts>/proposals/<finding-id>.diff`. Keep proposals as standalone diffs so they're reviewable independently — the user might accept B1 and skip B7.

3. **Show the diff.** Display inline in the conversation. Format: standard unified diff with file headers. If the diff is large (>50 lines), summarize first then show the full diff on confirmation.

4. **One diff, one decision.** Don't batch. The user confirms / skips / edits per proposal. Batching encourages rubber-stamping; one-at-a-time encourages reading.

## Applying confirmed diffs

`scripts/apply-refactor.sh` handles the apply with safety wrapping:

1. Backup: `cp <target-file> state/run-<ts>/backups/<target-file>.before`.
2. Apply: `patch <target-file> < <diff>`.
3. Verify:
   - For SKILL.md: parse frontmatter, count lines (≤500), check description still under 1024 chars.
   - For reference files: count lines, check TOC if >100 lines.
   - For shell scripts: `bash -n <script>` (syntax check), check shebang + `set -euo pipefail` still present.
4. If verify fails: restore backup, report failure with the verify error.
5. If verify passes: write a line to `state/run-<ts>/audit.log` recording the apply.

## Eval append-only policy

When Phase 2 produces new pressure scenarios that survived REFACTOR cycles, they become new entries in the target skill's `evals/evals.json`. The policy is **append-only**:

- New eval entries get fresh IDs (suffixed `-N` if needed for uniqueness).
- Existing entries are never modified or removed.
- The `source` field of new entries cites the audit: `"source": "audit-skill <YYYY-MM-DD> RED scenario discipline-A"`.

Why append-only: existing evals are someone else's RED-GREEN-REFACTOR work. Overwriting them silently undoes that work. If an old eval is genuinely wrong, it's a separate decision (delete) — never collateral damage from a refactor pass.

## REFACTOR cycle

After the diffs apply, re-run the same pressure scenarios from Phase 1 (skill-loaded). Capture outcomes:

1. **Scenarios that previously failed and now pass:** mark the corresponding `P7.<id>` finding as `resolved`.
2. **Scenarios that previously failed and still fail:** the tightening didn't close the loophole. Propose further tightening, cycle again.
3. **Scenarios that previously passed and now fail:** the tightening introduced a regression. Roll back that specific diff, propose an alternative.
4. **Brand-new failures (scenarios that always passed but a new section of the skill is now vulnerable):** the refactor surfaced a previously-hidden weakness. Add to findings, propose tightening.

Cap at 3 cycles total. After cycle 3, surface unresolved findings and halt — the skill design has a deeper issue.

## What to do when a finding can't be auto-fixed

Some findings need human judgment, not a diff:

- **C2 (catalog section appropriate)** — semantic judgment on placement.
- **B5 (success criteria per step)** — may require restructuring the procedure.
- **R4 (refs don't duplicate)** — may require merging or splitting files.

For these, the refactor pass records them as `skipped — manual` in the report and surfaces them as "needs author review" rather than applying a guess.

## Rollback

`scripts/apply-refactor.sh` writes a per-file backup before each apply. To roll back all changes from a run:

```bash
for backup in state/run-<ts>/backups/*.before; do
  target=$(echo "$backup" | sed 's|state/run-[^/]*/backups/||; s|\.before$||')
  cp "$backup" "$target"
done
```

This is a manual operation, not scripted — rollbacks should be deliberate and reviewed. The backups stay until the run is archived.

## Commit hygiene

Audit-skill does NOT commit changes. After Phase 2 completes:

1. Show `git diff` of all applied changes.
2. Recommend a conventional-commit message: `refactor(skills/<name>): apply audit findings <YYYY-MM-DD>`.
3. Let the user commit when they're ready.

This keeps the audit pass and the commit decision separate — the user can review the changes once more before committing, or amend before the next push.

## When refactor goes wrong

If a diff apply fails verify and rolls back, the audit report records:

- The finding the diff targeted.
- The verify error.
- The recommendation: re-attempt manually or surface to the user for design discussion.

Don't retry automatically — verify failures are signal that the proposed change was wrong, not transient.
