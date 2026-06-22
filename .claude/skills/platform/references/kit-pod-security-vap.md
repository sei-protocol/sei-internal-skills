# Pod Security + CEL ValidatingAdmissionPolicy kit

## 1. What this concern is

Pod Security on this fleet is **two layers**: PSS `restricted` (version-pinned per namespace) for the baseline hardening, **plus** an in-tree CEL `ValidatingAdmissionPolicy` (VAP) for the structural vectors PSS does not cover. The generic mental model — "PSS `restricted` is sufficient" — is wrong here: PSS does **not** bar ephemeral containers and does **not** enforce a container-name allowlist, and on the VPC-CNI hub NetworkPolicy is documentation-grade, so the VAP is a *load-bearing* control, not a nicety. *Cited:* `clusters/prod/walle/namespace.yaml` (PSS labels), `clusters/prod/walle/podcomposition-policy.yaml` (the VAP); `sources.md` §pod-security (the generic floor this specializes).

## 2. The pattern (how this fleet does it)

- **PSS `restricted`, version-pinned.** A namespace pins `pod-security.kubernetes.io/{enforce,audit,warn}: restricted` **and** the matching `*-version: v1.34` (the cluster minor) — pinning the version is what stops a cluster upgrade from silently loosening or tightening the policy. *Cited:* `clusters/prod/walle/namespace.yaml`.
- **The VAP closes PSS's gaps with CEL — no new controller** (`ValidatingAdmissionPolicy` is in-tree GA). The `walle` pod-composition lock bars what PSS misses: (1) **ephemeral containers entirely** (`ephemeralCount == 0`) — the `kubectl debug` / debug-sidecar vector, where an ephemeral container joins the pod netns and could reach a `127.0.0.1` loopback listener, bypassing a verifying proxy; (2) a **container-name allowlist** (`['omnigent-server','kube-rbac-proxy']`) on regular *and* init containers — bars an injected mesh proxy / smuggled sidecar / second app container; (3) a **max-container count** (`<= 2`) as defense-in-depth. *Cited:* `walle/podcomposition-policy.yaml` validations 1–4.
- **The binding's `validationActions: [Deny]` + `failurePolicy: Fail`** make it fail-closed — an un-evaluable policy *rejects* the pod (safe because `walle` is an app namespace, not a system one; confirm a system namespace wouldn't deadlock bootstrap). `Audit,Warn` is the dry-run alternative for a first prod VAP. *Cited:* `walle/podcomposition-policy.yaml` binding.
- **The control holds by LOGIC, not by an accidental CEL error.** Every `object.spec.*` access is wrapped in `has(object.spec)` so a spec-less object shape (e.g. on the `pods/ephemeralcontainers` subresource) doesn't *error* into fail-closed — empty lists short-circuit `.all()` true and `ephemeralCount` is `0`, so the intended check still evaluates correctly. *Cited:* `walle/podcomposition-policy.yaml` variables comment.

## 3. Anti-patterns / failure modes

- **Relying on PSS `restricted` alone for netns/sidecar isolation.** PSS doesn't bar ephemeral containers or extra named containers. Cue: a loopback-bound listener (`127.0.0.1`) "protected" only by PSS, no VAP. Rewrite: add a pod-composition VAP (ephemeral-bar + name allowlist).
- **Un-pinned PSS version.** `enforce: restricted` with no `enforce-version`. Cue: a namespace label set without the matching `*-version`. Rewrite: pin to the cluster minor so an upgrade can't shift the policy silently.
- **A fail-closed CEL that holds by *error*, not logic.** A validation that throws (rather than evaluating false/true) on an unexpected object shape — brittle, and silently relies on `failurePolicy: Fail`. Cue: `object.spec.containers` accessed without `has(object.spec)`. Rewrite: guard every spec access so the rule evaluates correctly when spec is absent.
- **Silent exit from a label-coupled binding.** A VAP binding scoped by `objectSelector` (e.g. `app: omnigent-server`) stops gating a pod if that pod-template label is ever moved/removed — the pod silently leaves the lock. Cue: a relabel / label-to-metadata-only change on a workload a VAP binding selects. Rewrite: treat the selector label as load-bearing security config; any change must re-confirm the binding still selects the pod.
- **`Audit/Warn` left on a control meant to enforce.** A VAP that should reject is in dry-run. Cue: `validationActions: [Audit, Warn]` on a load-bearing lock past its bake-in. Rewrite: `[Deny]` once render-verified.

## 4. Review cues

- **Dimension 1 (security posture & least-privilege):** PSS `restricted` enforced **and** version-pinned; a VAP covers the vectors PSS misses (ephemeral containers, container-name allowlist) for any pod with a loopback-bound listener or strict composition need; the VAP fails closed (`failurePolicy: Fail`, `[Deny]`) and holds by logic (guarded spec access), not by CEL error. *Basis:* profile §5, `sources.md` §pod-security.
- **Dimension 3 (GitOps-reconcilability):** the VAP + binding are declarative in the cell tree, not a `kubectl`-applied one-off. *Basis:* profile §1.
- **Label-coupling check:** if a VAP binding uses an `objectSelector`, the selected pod-template label is flagged as load-bearing — a change to it re-confirms the binding. *Basis:* `walle/podcomposition-policy.yaml` header (the silent-exit warning).

## 5. One-way doors in this concern

- **Tightening a VAP to `[Deny]` on a system/bootstrap namespace** can deadlock cluster bring-up if the policy can't evaluate — a fail-closed gate on a system path is blast-radius-wide; flag for human approval (app namespaces like `walle` are safe).
- **Loosening the allowlist / lifting the ephemeral-container bar** on a pod that fronts a loopback listener re-opens the netns vector — a security-boundary change, not a tweak; flag it.
- **Changing the PSS enforce *version* or level on a prod namespace** shifts what admission rejects fleet-wide on the next reconcile; flag for human approval.
