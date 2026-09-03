# Guardrails — author-skill

Expanded safety model for the `author-skill` workflow. The SKILL.md stanza is the short form; this file is the load-bearing version.

## Scope

`author-skill` writes to:

- **Project scope (default):** `<git-repo-root>/.claude/skills/<name>/`
- **User scope (`--user`):** `~/.claude/skills/<name>/`
- **Sync list:** `<git-repo-root>/scripts/sync-skills.sh` (sei-internal-skills repo only; only when the user confirms portability)
- **Catalog:** `<git-repo-root>/.claude/skills/README.md` (only when the user confirms the catalog entry)

It does **not** write to any other path. It does not modify existing skills outside its own scope.

## Pre-flight checks

Before scaffolding (Step 11 in the procedure):

1. `git rev-parse --show-toplevel` succeeds and matches the user's expected repo. If not, halt.
2. Target path `<repo>/.claude/skills/<name>/` does not exist OR exists and is empty. If non-empty, halt with the contents listed and ask for resume / archive / abort.
3. `<name>` is kebab-case, ≤32 chars, does not collide with the protected list:
   - `coral`, `council`, `design`, `issue`, `bugbash`, `author-skill`, `harbor-dev`
   - Any other entry already in `<repo>/.claude/skills/*/` — collision is always halt.
4. If `--user` is set, the user has explicitly opted into writing under their home directory (one-time confirmation per invocation).
5. The pressure scenarios from RED converted cleanly into evals (Step 10). If evals.json is empty, halt — every skill ships with at least one happy-path and one halt-condition eval.

## Scope confirmation ritual

Before Step 11, the skill echoes:

```
About to scaffold a new skill:

  name:    <kebab-case-name>
  scope:   <project|user>
  path:    <resolved-absolute-path>
  shape:   <discipline|technique|pattern|reference>
  evals:   <N> (RED+GREEN survived)
  catalog: <will-be-added|skipped>
  sync:    <PORTABLE|SEI|none>

Confirm? (yes / adjust / abort)
```

Require literal "yes" or "confirm" — not "ok", not "sure". The confirmation must be explicit and unambiguous so the audit log captures it.

## Destructive actions requiring extra confirmation

Even inside the happy path:

- **Catalog edits** (Step 12) modify `.claude/skills/README.md`. Show the proposed diff first; require confirmation.
- **Sync-list edits** (Step 13) modify `scripts/sync-skills.sh`. Show the proposed diff first; require confirmation.
- **Issue comments** (Step 14) post to GitHub. Show the proposed comment body; require confirmation.

None of these are auto-applied. Each is a separate confirm gate.

## Anti-corruption patterns

- All draft work happens under `state/run-<ts>/`. Nothing escapes to the target path until Step 11. If the run is interrupted mid-draft, the next invocation finds the partial state and offers to resume — the target tree is untouched.
- The `state/` tree is `.gitignore`d at the repo level (`.claude/skills/*/state/`). Drafts never accidentally land in git.
- `scripts/scaffold.sh` is idempotent only over an empty target; never over a non-empty one (refuses with exit 2). This prevents a re-run from clobbering a half-finished skill.

## Unsafe patterns (NEVER, even pre-approved)

- **No silent overwrites.** The scaffold script refuses if the target tree is non-empty. The catalog/sync scripts show a diff before applying.
- **No skipping RED.** Skipping the baseline pressure test produces a skill that "looks right" but hasn't been tested against the failures it's supposed to prevent. Per Obra: "If you didn't watch an agent fail without the skill, you don't know if the skill prevents the right failures."
- **No skipping the guardrails-first rule.** If the guardrails stanza can't be drafted in Step 4, the skill isn't safe to author. Halt — don't write the rest and hope.
- **No auto-applying the catalog entry or sync-list change.** Both are reviewed by the user before the edit lands.
- **No editing protected skills through this workflow.** coral, council, design, issue, bugbash, author-skill itself — these are edited directly in a PR, with the council for substantive changes.
- **No skipping evals.** A skill ships with at least one happy-path and one halt-condition eval, derived from the pressure scenarios.
- **No "I'll skip REFACTOR, the skill is obviously clear."** The REFACTOR loop is where loopholes get plugged. Skipping it is the most common failure mode and produces skills that get bypassed under real pressure.

## When the user pushes back

If the user asks to skip a step ("can we skip RED, I'm in a hurry"), surface this explicitly:

> Skipping the RED baseline means the skill ships without verified evidence that it prevents the failures it's supposed to prevent. The trade-off is roughly 15–20 minutes saved now for a 2× to 5× higher chance the skill gets bypassed under pressure once shipped. Recommend running RED. Continue with RED, or override and skip?

Recovery, not refusal: state the trade-off, recommend the safe path, accept the override if the user insists. Log the override in `state/run-<ts>/audit.log`.

## Audit log

Every script call, every subagent dispatch, every confirm-or-deny gate writes a line to `state/run-<ts>/audit.log`:

```
2026-05-10T14:23:11Z  scaffold.sh --name terraform-review --scope project --shape technique  exit=0
2026-05-10T14:23:14Z  Agent(subagent_type=general-purpose, scenario=pressure-1, phase=RED) -> rationalization captured
2026-05-10T14:24:02Z  add-catalog-entry.sh --name terraform-review --section Hardening  exit=0  user-confirmed=yes
2026-05-10T14:24:18Z  override: user requested skip-REFACTOR after 1 cycle
```

The audit log is the post-hoc record of what the skill did and what the user authorized.
