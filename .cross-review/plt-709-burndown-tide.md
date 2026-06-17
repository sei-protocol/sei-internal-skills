# Cross-review ledger — PLT-709 Tide burn-down (PR Tide#199)

Artifact: the burn-down half of the Design-05 relocation (remove `design/`+`docs/designs/`;
make `/tee` self-contained + portable + first-class; update `/research`+`/design` conventions).
Workstream: PLT-709 (child of PLT-497). Full-team collaborative refinement per the owner's directive.

Slate (blinded, independent; one assigned dissenter): tee-specialist, security-specialist,
idiomatic-reviewer, prose-steward, skill-authoring/audit lens, technical-program-manager,
+ assigned dissenter. Checks: Cursor Bugbot.

## Round 1 — findings

| Reviewer | Verdict | Load-bearing findings |
|---|---|---|
| idiomatic-reviewer | RATIFY | Contract inverted coherently across all surfaces; 2 style nits. |
| technical-program-manager | RATIFY | Conservation 24/24; rename map complete; 2 Phase-D handoffs (non-blocking). |
| security-specialist | REVISE | Citation-integrity intact; README repo-root stale (MED); kit-sei-onchain bare refs (LOW). |
| prose-steward | REVISE | §7 Citations + kit-TEMPLATE:31/73 still name the access-gated archive as "Ground truth". |
| skill-authoring/audit | GAPS (1 block) | D2 description 1029>1024; eval/sync citation drift; dispatch claim oversold. |
| tee-specialist | REVISE (2 block) | B1 §7 leads with archive → self-containment aspirational; B2 agent file routes to archive. No attestation fact lost. |
| assigned dissenter | REFUTED | B1 README dangling; B2 DRI-repo directive has no resolution mechanism (inert); M1 "at most once" violated; M2 Sei cost-ranking cited only to private corpus (fabrication risk in no-access sync). |

## Round 1 — resolutions applied (HEAD after fix commit)

- **§7 reframe (tee B1 / prose / skill-audit / M1):** all 6 kits + method + kit-TEMPLATE + tee-profile — primary sources promoted to "Ground truth", the access-gated archive demoted to "Provenance (access-gated, not required)". kit-sei-onchain header reframed. Template authoring contract flipped (cite primary, not the research doc; archive is breadcrumb-only).
- **tee-specialist.md (B2):** lines 11/17 repointed from the archive to public primary sources / vendor specs.
- **D2:** `/tee` description trimmed to ≤1024.
- **B1 README:** repo-root `README.md` layout block + doc-map row for deleted `design/` removed.
- **B2 DRI-repo resolution:** `/design` + `/research` now define how to resolve the DRI repo (`--designs-repo` flag → sibling `<name>-designs` checkout → ask the user); `docs/designs/` is a confirmed-last-resort fallback, never a silent fall-through.
- **M2 cost-ranking:** `method.md` anchors the public figures (sei-chain precompile floor, Marlin Oyster, Trail-of-Bits-audited Automata) and marks the cross-vendor synthesis a non-citable planning estimate (unverified without corpus access).
- **Mechanical:** evals citation assertion → primary source; `sync-skills.sh` stale `security` refusal arm removed; "How this fits" reworded (domain lens, not auto-wired steward); idiomatic style nits.
- **Accepted-with-rationale (non-blocking):** kit-sei-onchain bare body breadcrumbs (LOW, facts inlined; the reframed header + §7 make authority unambiguous); historical `.cross-review/`+`docs/skill-audits/` pointers (immutable artifacts, covered by the rename map for Phase-D); the 2 TPM Phase-D handoffs.

State: OPEN — round-2 verification pending (re-dispatch tee-specialist, prose-steward, skill-audit, dissenter + Bugbot).
Convergence: pending.
Dissenter: held (assigned, REFUTED round 1).
OpenFindings: round-2 verification of the applied resolutions.
