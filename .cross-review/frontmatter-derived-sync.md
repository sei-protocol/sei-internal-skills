# Cross-review ledger — frontmatter-derived skill/agent sync + one-command + over-the-wire installer

Target:       branch `tooling/frontmatter-derived-sync` (scripts/sync-skills.sh, scripts/sync-agents.sh, scripts/install.sh, scripts/tests/catalog-coverage.test.sh, Makefile, .github/workflows/verify-catalog.yml, scripts/README.md, .claude/skills/README.md); PR from this branch
Class:        infra-tooling (shell scripts + Makefile + CI + docs)
Tier:         T2 (Component) — the skill/agent distribution mechanism; no persisted schema / wire format / one-way door
Scope:        Authoritatively fix the drift class that let `gov-ops` (release-operations) and `ebpf` (performance) fall out of the hand-maintained sync arrays: derive alias membership from each skill/agent's own `category:` frontmatter (single source of truth), CI-enforce completeness with a coverage guard, and make "get current" trivial — `make update` (cloned) and a `gh api … | bash` over-the-wire one-liner (Tide is INTERNAL; gh provides auth). Tide-local boundary (output-quality, security/tee) preserved + made explicit.
Dissenter:    fidelity (assigned) — re-runs both scripts against a `main` worktree to prove derived membership ≡ prior behavior + the intended fixes, and adversarially probes the coverage guard (orphaned + category-less + CRLF) under the real bash 3.2.

## Round 1
State:        REVISE
OpenFindings: resolved in Round 2 (1 blocking bug found independently by 2 lenses; 1 prose blocker; non-blocking polish)
Convergence:  split

| Lens | Verdict | Finding |
|---|---|---|
| fidelity (assigned dissenter — runs the scripts, bash 3.2) | DISSENT | Membership derivation correct + regression-free (portable/sei/all reproduced exactly; gov-ops joins sei, ebpf stays portable via performance, nothing else moves; sei-network-specialist override honored). **D1 [blocking]:** under `set -euo pipefail`, a skill/agent with NO `category:` made `grep` exit 1 → pipefail → `set -e` aborted the loop BEFORE the "no category" diagnostic, so `--verify` exited 1 with **zero output** (fails closed only by accident). **D2 [blocking]:** the no-category test asserted only the exit code, so it passed green on the silent crash. |
| platform-engineer (shell correctness) | REVISE | Independently found the same **D1** bug (sync-skills.sh + sync-agents.sh share it). Everything else clean — `in_list` rejects partial tokens (`code`≠`code-quality`), `want_*` non-zero returns are `if`-guarded (no `set -e` abort), bash 3.2 compatible, `make update` fails closed if the pull fails, CI `paths:` correct. Non-blocking: CRLF in a `category:` line would spuriously fail on GNU-sed CI; test fixtures lack a `trap` cleanup. |
| prose-steward | REVISE | One **blocking**: `make update` prose claimed "pull latest Tide **main**" on two README surfaces while the recipe runs `git pull --ff-only` on the current checkout. Else clean — one-path clarity strong, "three mirrored places" claim retired everywhere, Tide-local naming consistent, markdown intact. |

### Round 1 resolution (applied before Round 2)
- **D1 (blocking):** `skill_category`/`agent_category` rewritten as `{ grep -m1 … || true; } | tr -d '\r' | sed …` — a no-match can't fail the pipeline/abort the caller, so the guard PRINTS its diagnostic and exits non-zero; `tr -d '\r'` also fixes the CRLF non-blocking item. Verified: category-less skill AND agent now print `no 'category:'` + exit 1.
- **D2 (blocking):** added `check_fail_msg` (asserts exit-nonzero AND the diagnostic message) for the orphaned + category-less cases; added a `trap` to clean up live-tree fixtures.
- **Prose (blocking):** reconciled the `main` over-claim across all four surfaces (Makefile help/echo + both READMEs) to "fast-forward this checkout".
- **Scope addition (the operator's follow-on ask):** factored `make sync-all` (sync all + verify, no git) out of `make update`; added `scripts/install.sh` (clone-or-fast-forward + `make sync-all`) runnable over the wire via `gh api … | bash`; README one-liner.

## Round 2
State:        RESOLVED
OpenFindings: 0 (3 non-blocking items folded as finalization)
Convergence:  unanimous

| Lens | Verdict | Finding |
|---|---|---|
| fidelity (assigned dissenter) | RATIFY | D1 RESOLVED (category-less skill+agent print the diagnostic + exit non-zero). **D2 proven non-vacuous** by reverting the `|| true` fix to reproduce the exact Round-1 silent crash and showing `check_fail_msg` FAILs where a bare exit-code check would pass. No membership regression (skills 16/7/23, agents 17/2/19; all anchors correct). CRLF resolves correctly. install.sh fail-closes on a non-git TIDE_HOME, reuses `sync-all` (no double-pull), idempotent, `set -euo pipefail`. No surviving defects. |
| platform-engineer (shell correctness) | RATIFY | D1 fix correct + complete on both scripts; new installer + `sync-all`/`update` split shell-correct, fails closed, quotes paths, avoids the double-pull; shellcheck clean (SC1091 only); 13/13 suite. Non-blocking: `$TIDE_HOME` unquoted inside an echo hint (cosmetic); the one-liner 404s until the branch lands on `main` (reconfirm the raw fetch post-merge); commit the uncommitted README trust-note onto the branch. |
| security-specialist (over-the-wire trust model) | RATIFY | The installer is a thin, fail-closed wrapper over a trust relationship the repo already holds — `gh` gates *who* can fetch (confidentiality), GitHub is the integrity anchor (same as the existing clone+make flow); partial-download execution is benign by construction (no destructive op precedes the network op); `TIDE_HOME`/env handling is injection-safe and quoted; the non-Tide-path clobber guard is correct; no insecure residue on failure. Advisories: A1 add a trust-assumption note, A2 offer the read-before-you-run path, A3 do NOT pin a ref (floating `main` is intended). |
| prose-steward | RATIFY | Round-1 `main` over-claim fully retired (grep-confirmed across all surfaces); over-the-wire prose matches install.sh behavior exactly; three paths (over-the-wire / `make update` / `make bootstrap`) distinct and unambiguous in both READMEs; markdown intact. One low-severity style note: "ALL" vs "portable + Sei" wording of the same operation between the two READMEs. |

### Round 2 finalization (folded — all non-blocking)
- install.sh echo hint: `$TIDE_HOME` quoted (paste-safe with spaced paths) — platform-engineer.
- README trust note + read-before-you-run alternative — security A1/A2 (A3 honored: no ref pinning, floating `main` documented as intentional).
- "ALL skills/agents" → "all portable + Sei skills/agents" in scripts/README — prose style alignment with .claude/skills/README.
- Pre-merge action carried out of band: reconfirm the `gh api … contents/scripts/install.sh` raw fetch returns the file (not 404) once the branch is on `main`, before relying on the one-liner.

### Verdict
RESOLVED — unanimous RATIFY in Round 2; zero open findings. The assigned fidelity dissenter ran both scripts under the real bash 3.2 to prove (a) the derived membership is byte-identical to the deleted hand-maintained arrays except the intended `gov-ops` repair, and (b) the coverage guard now fails closed *with a diagnostic* on every gap (orphaned category, category-less, proven by reverting the fix). The blocking guard bug (D1) was found independently by two lenses and is fixed at the function source on both scripts; the test that masked it (D2) now asserts the message, proven non-vacuous. The over-the-wire installer was cleared by a dedicated security lens as trust-equal to the existing clone+make flow.

**CI:** the new `verify-catalog.yml` runs `make verify-catalog` + the regression suite on any skill/agent/script change — the orphan class can't silently regress. Cursor Bugbot + CI evaluated on PR open (Bugbot skip/NEUTRAL satisfies the check half for this tooling PR per the recorded review-gate policy; failing/findings block).
