# Guardrails — audit-skill

Expanded safety model. The SKILL.md stanza is the short form; this file is the load-bearing version.

## Scope

`audit-skill` operates on:

- **Read (always):** `<repo>/.claude/skills/<name>/` or `~/.claude/skills/<name>/` and the conventions catalog at `<repo>/.claude/skills/audit-skill/references/conventions-catalog.md`.
- **Write (Phase 1 only):** the audit report — a **lineage artifact** that lands in the DRI's `<engineer>-designs` repo at `designs/<arc>/audits/<name>-<YYYY-MM-DD>.md` (repo-default arc `tide-skill-stack` for Tide; Design 13 — process-lineage relocation). Resolve the DRI repo as `/design` does (`--designs-repo` → sibling `<engineer>-designs` checkout → ask; in a non-interactive run, HALT and surface — never guess). The in-repo `docs/skill-audits/<name>-<YYYY-MM-DD>.md` is the fallback used **only when no DRI repo is resolvable and the user confirms**. Plus `state/run-<ts>/` (working state).
- **Write (Phase 2 only):** files inside the target skill's directory, after diff confirmation. Plus `state/run-<ts>/backups/` and `state/run-<ts>/proposals/`.

It does **not** write to any other path. It does not modify the conventions catalog (catalog edits are a separate, manual decision).

## Pre-flight checks

Before reading any skill files (Phase 1):

1. `git rev-parse --show-toplevel` succeeds. If not in a git repo, halt.
2. Target skill directory exists. If not, halt with the resolved path.
3. `<target>/SKILL.md` exists and parses (has frontmatter delimiters). If not, halt.
4. Conventions catalog at `<repo>/.claude/skills/audit-skill/references/conventions-catalog.md` exists and is readable. If not, halt — the audit cannot run without it.

Before entering Phase 2 (refactor):

5. Phase 1 completed in this session (a `state/run-<ts>/static-findings.jsonl` exists). No skipping straight to edits.
6. Target skill is not on the protected list, OR `--override-protected` was passed.
7. Target skill's directory has no uncommitted local changes (run `git status` on the target subtree). Refactor on a dirty tree is high-risk; halt and ask the user to commit or stash first.

## Protected canonical skills

The protected list:

- `coral`, `council`, `design`, `issue`, `bugbash`, `author-skill`, `audit-skill`, `chaos-suite`, `harbor-dev`

**Audit is always allowed on protected skills.** That's the whole point — these are the most important skills to audit.

**Refactor on protected skills requires `--override-protected`** plus an explicit second confirmation gate at Phase 2 entry:

```
You're about to refactor a CANONICAL skill: <name>.

Canonical skills are owned by Tide directly and are edited via PR with the council
on substantive changes. This pass will apply diffs directly to the working tree;
you will need to commit and PR them.

Are you sure? (yes / abort)
```

Require literal "yes".

## Scope confirmation ritual

Before Phase 1 begins, echo:

```
About to audit:

  skill:           <name>
  path:            <resolved-absolute-path>
  inferred shape:  <discipline|technique|pattern|reference|procedural>
  report path:     <designs/<arc>/audits/<name>-<YYYY-MM-DD>.md in <engineer>-designs (DRI repo; in-repo docs/skill-audits/ only if no DRI repo)>
  phase:           audit-only (refactor opt-in after report)
  protected:       <yes|no>

Confirm? (yes / adjust / abort)
```

Before Phase 2 begins, echo a separate Phase-2 confirmation:

```
About to refactor:

  skill:           <name>
  findings:        <N block, M warn>
  proposals queued:<N> (one-at-a-time, with diff-before-write)
  override-protected: <yes|no>
  uncommitted state in target subtree: <yes|no>

Confirm? (yes / audit-only-stop / abort)
```

## Destructive actions requiring extra confirmation

- **Each per-finding apply** requires its own confirmation after the diff is shown. No batching, no rubber-stamping.
- **`--override-protected`** requires both the flag AND the second confirmation gate. The flag alone is not enough.
- **`apply-refactor.sh` verify failures** roll back automatically — no proceeding past a failed apply.

## Anti-corruption patterns

- **Audit is read-only.** Phase 1 never writes to the target skill. Only writes go to `state/run-<ts>/` (working) and the audit report's lineage home — `designs/<arc>/audits/` in the DRI repo (in-repo `docs/skill-audits/` only as the no-DRI-repo fallback).
- **Backups before every apply.** `scripts/apply-refactor.sh` writes `<file>.before` to `state/run-<ts>/backups/` before each apply. Rollback is a `cp` away.
- **Verify after every apply.** Parse frontmatter, count lines, syntax-check shell scripts. Verify failure → automatic rollback.
- **State is per-run, gitignored.** Interrupted runs leave state intact; the next invocation can resume.
- **Append-only evals.** Phase 2 never overwrites existing eval entries; it appends.

## Unsafe patterns (NEVER, even pre-approved)

- **No batched diff applies.** One diff, one decision. Bulk-applying findings encourages rubber-stamping.
- **No silent overwrites.** Every edit passes a diff-before-write gate.
- **No skipping Phase 1.** Phase 2 requires Phase 1 to have completed in the same session.
- **No editing the conventions catalog from inside an audit.** Catalog changes are out-of-band — open a separate PR.
- **No retrying failed verifies.** A verify failure means the proposed change was wrong, not transient. Roll back, surface, ask.
- **No skipping the override gate for protected skills.** The flag alone is not authorization; the second confirmation gate is.
- **No bypassing the dirty-tree check.** A refactor on uncommitted local changes risks losing the user's in-progress work.

## When the user pushes back

If the user asks to skip a gate ("just apply all the warnings, I trust the audit"), surface explicitly:

> Bulk-applying findings without the per-diff review skips the load-bearing safety in this skill. The trade-off is roughly 10 minutes of review per audit vs. a 2-5× higher rate of incorrect edits landing silently. Recommend the per-diff review. Continue with per-diff, or override and bulk-apply?

If they insist on the bulk override:

1. Log the override in `state/run-<ts>/audit.log`.
2. Still show each diff (one at a time) but with a default "apply" rather than "confirm".
3. Still verify each apply and roll back on failure.

The override changes the default; it does not remove the safety.

## Multi-Skill Session Fatigue (P7.B defense)

Surfaced as a partial-fail in audit-skill's own self-audit (PR #49 docs/skill-audits/audit-skill-2026-05-11.md). The pressure scenario: at 8pm on audit 5 of 7, with the same D1/eval-source warnings recurring across skills, the temptation to bulk-accept on "we've already reviewed the pattern five times" was real. The skill's bulk-apply override path catches the *first* form of this (asking to skip per-diff review for a single skill); this section catches the second form (cumulative fatigue across multiple skills in one session).

### The rationalization

> "I've reviewed the same D1 fix on four skills already. The fifth one is the same pattern. I can skim instead of carefully reading the diff."

The risk: pattern-recognized warnings feel safe to bulk-accept, but cross-skill the diffs differ in subtle ways (different anti-trigger phrasings, different evals shape, different terminology to consolidate). Skim-accepting drops these subtle differences, and the audit's value collapses.

### The counter

When applying findings across multiple skills in a single session, the orchestrator should:

1. **Surface the per-session apply count** in every confirm prompt past the third apply. Format: `[session apply #N — fatigue note: consider a break or splitting the work after N=10]`.
2. **At N ≥ 10**, recommend taking a break or splitting the remaining work into a separate session. The recommendation surfaces; the user decides.
3. **At N ≥ 15**, refuse to default to "apply" — every apply requires explicit "confirm" regardless of any prior bulk-override. The override doesn't carry across this many applies.

### Rationalization counters

| Excuse | Reality |
|---|---|
| "We've reviewed the same pattern five times — I can skim this one." | The pattern is similar; the per-skill specifics (anti-triggers, evals, terminology) differ. Pattern-recognized accept loses the differences. |
| "It's late and I just want to finish the cycle." | Cycle completion is not a safety override. Splitting the work into the next session preserves the audit's value. |
| "The diff looks the same as the previous skill's diff." | "Looks the same" is the failure mode — diffs that look the same but apply to different contexts produce different results. |
| "The pattern review I did earlier covers this case." | The session apply log records prior reviews; the current diff is its own decision. |

Red flags during long remediation sessions:

- "I'll just trust the pattern at this point"
- "I'll come back and audit-check these tomorrow"
- "We're 4 skills in, let me batch the rest"
- "I'm not going to find anything new at this stage"

If any of these surface, take a break or split the work. Don't override into them.

## Audit log

Every script call, every subagent dispatch, every confirm-or-deny gate writes a line to `state/run-<ts>/audit.log`:

```
2026-05-10T14:23:11Z  static-checks.sh --skill-dir /path/to/skill  exit=0  findings=14
2026-05-10T14:23:14Z  Agent(subagent_type=general-purpose, check=semantic-description)  findings=3
2026-05-10T14:23:42Z  Agent(subagent_type=general-purpose, scenario=discipline-A, phase=audit)  result=fail
2026-05-10T14:24:02Z  apply-refactor.sh --diff B1.diff --target SKILL.md  exit=0  verify=pass
2026-05-10T14:24:18Z  override: user requested bulk-apply for warn findings
```

The log is the post-hoc record of what the audit did and what the user authorized.
