---
name: kubernetes
category: platform-infra
model: claude-opus-4-8
description: "Use when designing or reviewing Kubernetes operator/controller code — CRDs, reconcilers, controller-runtime/kubebuilder work, child-resource lifecycle, status/conditions — especially in sei-k8s-controller: '/kubernetes', 'design this CRD', 'review this reconciler', 'is this reconcile idempotent', 'how should this controller signal the sidecar', 'will this status patch race'. A citable corpus (Kubernetes API conventions, controller-runtime, kubebuilder, CRD versioning) plus an always-first Sei-controller profile that encodes sei-k8s-controller's real conventions (plan-driven reconcile, optimistic-lock status patches, always-present conditions, CEL immutability one-way-doors). The operating manual for the kubernetes-specialist agent; pluggable kits per controller concern. Anti-triggers: NOT general Go/idiom conformance (use /idiomatic); NOT workload right-sizing / Karpenter NodePool / HPA / scheduling (use k8s-capacity-management); NOT platform manifests / Kustomize / GitOps / cloud-auth (use /platform + platform-engineer); NOT telemetry-stack values or PromQL (use observability agents); NOT Sei node P2P/RPC networking (use sei-network-specialist). Designs and reviews how the controller behaves; does not run the cluster."
---

# Kubernetes

Design and review Kubernetes **operator/controller** code so it is correct, idempotent, and durable at its CRD contracts — grounded in the upstream canon (Kubernetes API conventions, controller-runtime, kubebuilder) and, above all, in **sei-k8s-controller's own established patterns**. A *reference/technique* skill with a discipline spine. It is the operating manual for the `kubernetes-specialist` agent and is directly invocable (`/kubernetes <target>`).

## Why this skill exists

A capable model knows generic controller-runtime. The skill's job is the **citable upstream corpus** (the specific convention + source the model can't reliably reproduce) plus the **always-first Sei-controller profile** — the conventions this codebase enforces that *override* generic habit, the way `/idiomatic`'s repo profile outranks generic idiom. The failure mode it prevents: writing a plausible generic reconciler that violates the repo's hard rules (a non-optimistic-lock status write that silently drops a concurrent plan; a condition expressed by *removal*; a panic that crashes the manager; a CRD field change that breaks a live consumer).

The corpus is grounded in primary sources (`references/sources.md`) and stays copyright-clean: our-own-words checklists that cite, never reproduce.

## Guardrails

Refusal conditions — they hold under time pressure and a "just make it reconcile" urge:

1. **Profile- and kit-first.** Load `references/sei-controller-profile.md` (the always-first overlay — it encodes the repo's hard conventions and **outranks** generic best-practice) **and** the relevant kit(s) before designing or reviewing. When working *in* a repo, also read its governing doc (`CLAUDE.md`) — the live repo wins over this skill's snapshot; flag drift, don't silently follow the stale copy.
2. **Cite every finding; stay copyright-clean.** A primary source (`sources.md`) and/or the repo profile per finding — never a naked "this isn't idiomatic." Never reproduce reserved source text; summarize and link.
3. **Suggest-when-reviewing; author-when-building.** As a *review* lens, produce findings the human/calling agent applies — don't rewrite their files. As the `kubernetes-specialist` *building* the system, write the code, but flag one-way doors (CRD field/semantics changes, event signatures) for human approval before finalizing.
4. **The CRD contract is a one-way door.** A served-version spec field, its validation, or its semantics cannot change incompatibly once a controller or user depends on it (the upstream compatibility law). Flag any such change for human approval and route evolution through a new version + storage-version/conversion strategy — never assert the breaking change as the fix.
5. **Don't duplicate the adjacent lenses.** Pure Go idiom → `/idiomatic`. Capacity/scheduling (requests/limits, Karpenter, HPA, topology) → `k8s-capacity-management`. Manifests/Kustomize/GitOps/cloud-auth → `/platform`. Telemetry-stack values/PromQL → the observability agents. Sei node P2P/RPC → `sei-network-specialist`. This skill is the *controller code and its CRD contract*.

## The method

`references/method.md` holds the full method; the spine:

1. **Load the profile + the kit(s)** for the concern in hand (reconciliation, CRD design, sidecar integration, child-resource lifecycle, …). Read the repo's `CLAUDE.md` if working in one.
2. **Design or review against the profile first, the upstream canon second.** The profile's conventions (optimistic-lock status, always-present conditions, planner-owns-conditions, CEL immutability) are hard rules here; the upstream canon (`sources.md`) is the generic floor beneath them.
3. **Score/identify by the five controller dimensions** (`method.md`): reconcile correctness & idempotency · CRD-contract durability · failure-mode handling · RBAC least-privilege · observability (conditions/`observedGeneration`) — plus testability.
4. **Cite every finding** (a `sources.md` anchor and/or a profile rule) and rank one-way-door / correctness findings above style. Flag CRD/wire one-way doors for human approval.

## Kit index

| Concern | Kit |
|---|---|
| The plan-driven, level-triggered reconcile model (ResolvePlan → persist → ExecutePlan) | `references/kit-plan-driven-reconciliation.md` |
| Signaling the per-node seictl sidecar over its HTTP task API | `references/kit-sidecar-task-integration.md` |
| CRD design — discriminated unions, CEL immutability/validation, status subresource, the `kubectl wait` latch | `references/kit-crd-design.md` |
| Child-resource (StatefulSet/Service/PVC/Job) lifecycle via server-side apply — SSA field-ownership, `OnDelete` replace-pod, impostor detection, orphan-on-Retain | `references/kit-child-resource-lifecycle.md` |
| Chain-bootstrap modes (genesis ceremony, state-sync witness gate, snapshot Job, replayer/archive) | *(deferred — `kit-chain-bootstrap-modes`)* |
| Watches/requeue (predicates, Owns vs Watches, requeue cadences) | *(deferred — `kit-watches-and-requeue`)* |
| Controller deployment on EKS (manager setup, RBAC markers, IRSA/Pod-Identity, Karpenter) | *(deferred — cross-links `/platform`; `kit-controller-deployment-eks`)* |

The deferred kits are scoped in `references/kit-TEMPLATE.md`'s roster; add each as a conforming kit when the work is first encountered (the corpus grows by use, not up front).

## How the kubernetes-specialist agent hooks in

The `kubernetes-specialist` persona's first step loads `sei-controller-profile.md` + the kit for the work, then designs or reviews against the profile first. The agent authors the system; `idiomatic-reviewer` does the pure-idiom pass on top (idiom ⊂ controller quality), and `/platform` owns the manifests/deployment around it.

## Halt conditions

- **No target** to design/review (no code, CRD, or spec) — ask for it; never review a controller from memory.
- **A one-way door** (incompatible CRD-field/semantics change, an event/sidecar-contract change consumers depend on) — flag for human approval, don't assert it.
- **The work is really another lens** — idiom (`/idiomatic`), capacity (`k8s-capacity-management`), manifests/GitOps (`/platform`), node networking (`sei-network-specialist`) — redirect rather than stretch this skill over it.

## What this skill defers

The three deferred kits above (add by use); a controller-code eval-harness beyond the shipped evals; `/coral`+`/council` auto-dispatch wiring (un-defer when standalone is validated). The Sei-architecture profile is a *snapshot* of sei-k8s-controller's conventions — when working in that repo, its live `CLAUDE.md` is authoritative; this profile is the portable, cited distillation for work elsewhere and for review.
