# Prove-the-run — operator handoff

`prove_headless_run.py` is the **live half** of the omni-session work: the unit suite proves the
budget driver's logic with a fake session; this runs the **same merged driver** against a **real
omnigent 0.2.0 session** in your environment. It is a deliberate, hand-run, budget-bounded spike
— it does **not** auto-fire and nothing in the deployed cluster triggers it (the trigger path is
all unmerged at the operator gate).

## What it answers

1. **Foundational (Open-Q #2):** does a budget-truncated `/root-cause`-shaped run terminate
   `TRUNCATED` (`budget-exhausted`, `cancelled=True`) with a partial artifact — i.e. is the
   client-side driver (not the observer-only Stop hook) the load-bearing terminator on the live
   runtime? **This gates building more on the run layer.**
2. **The Design-13 Blocking dependency (the auth principal):** what principal does the server
   resolve the session under? It prints `server-resolved owner`. **If, behind the production
   kube-rbac-proxy sidecar, that is the SA name** (`system:serviceaccount:walle:omni-trigger`)
   **rather than the forwarded human email, the per-human PDP/owner model needs the
   in-band-initiator rework Design 13 flags** — so we want this answer before slices 3+.

## Prerequisites (your env — none exist in CI)

- **A runnable omnigent 0.2.0 server (header mode).** Two flavors, for the two purposes:
  - *Local* (answers Purpose 1): stand one up per `omnigent/deploy/README.md`. No sidecar.
  - *Deployed, sidecar-fronted* (answers Purpose 2 conclusively): the `walle` cluster's
    `omnigent` Service. A local server has **no** kube-rbac-proxy, so it trusts the
    `X-Forwarded-Email` we send verbatim — Purpose 2 is only conclusive against the sidecar (or a
    kube-rbac-proxy `--auth-header-fields-enabled` config audit).
- **`ANTHROPIC_API_KEY`** reachable by the host/runner that executes the agent (this is what
  incurs cost — see below).
- **A gzipped agent bundle** — e.g. omnigent's `examples/polly/agents/claude_code`:
  ```bash
  tar -C omnigent/examples/polly/agents -czf /tmp/claude_code.tar.gz claude_code
  ```
  (Confirm the exact layout your server expects against `omnigent/deploy/README.md`.)

## Run it

```bash
cd sei_omnigent   # the package must be importable (pip install -e . in your venv)
python spike/prove_headless_run.py \
    --server   http://127.0.0.1:8443 \
    --bundle   /tmp/claude_code.tar.gz \
    --goal     "Investigate: error rate spiked on service X at 14:02 UTC; find the cause." \
    --forwarded-email sei-omnigent@seinetwork.io \
    --wall-clock-s 20 --max-iterations 3
```

Tight caps (`--wall-clock-s 20 --max-iterations 3`) make truncation easy to demonstrate and keep
the run tiny. Loosen them for a real investigation.

## Reading the result

**Purpose 1 (exit code):**
- `PASS` (`truncated=True` + `cancelled=True`, exit 0) — the driver bounded a real run at the
  budget. **Foundation validated → slice 3 (bundle/model application) can proceed.**
- `FAIL` (`terminal_reason=ERRORED`, exit 1) — the run errored before completing (connect / auth
  / bundle / server). The reason is in the printed artifact; fix the env and re-run. (This is the
  honest ERRORED terminal, not a budget cut.)
- `INCONCLUSIVE` (within budget, exit 2) — the run finished before any cap; lower `--wall-clock-s`
  / `--max-iterations` and re-run. Not a failure.

**Purpose 2 (the `server-resolved owner` line):**
- Run against the **deployed sidecar-fronted** server and read `server-resolved owner`:
  - owner == the forwarded email → the per-human principal flows through → the PDP/owner model
    holds as designed.
  - owner == the SA name (or anything not the email) → **the Blocking dependency bites**: the
    per-human authz / SessionRegistry-owner / cross-user work needs the in-band-initiator rework
    (carry the initiator identity in a session label the router controls, distinct from the
    transport SA). Tell me the result and I'll fold the rework into the slice-4 design.
  - Against a *local* (no-sidecar) server this just confirms the header round-trips — not
    conclusive for the sidecar question.

## Cost

This runs a **real** Anthropic-backed session, so it incurs API cost — but it is a **single,
deliberate, budget-bounded** run you invoke by hand (20s wall-clock / 3 iterations by default ≈
a few cents). Nothing here is wired to PagerDuty/Alertmanager; there is no automatic or repeated
firing. Stop it any time with Ctrl-C (the driver cancels the session on teardown).

## Report back

Paste the printed block (the terminal-reason line + the `server-resolved owner`). That tells us
(1) the foundation is live and (2) the Blocking-dependency answer — which together unblock the
remaining slices.
