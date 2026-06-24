# root-cause agent bundle

The headless root-cause investigator for the Sei platform stack — the PD-dogfood
agent of the omni session-routing service (Design 13). An alert routed through
the ControlPlane's PagerDuty route opens a session against this bundle, which
runs the `/root-cause` discipline and returns a ranked, evidence-backed analysis
with a **proposed** (never executed) remediation.

## Layout

- `config.yaml` — the omni agent spec (`spec_version: 1`, `name: root-cause`).
- `skills/root-cause/` — a copy of Tide's `/root-cause` skill (SKILL.md +
  references), so the runner carries the discipline. Auto-discovered from
  `<bundle>/skills/`; no `skills:` line in the spec.

## name ↔ `_DEFAULT_PD_BUNDLE_REF` coupling

The name MUST be exactly `root-cause`. The ControlPlane's PD route emits
`bundle_ref="root-cause"`
(`sei_omnigent/src/sei_omnigent/omni/serve_receiver.py` `_DEFAULT_PD_BUNDLE_REF`),
and the `LiveSessionFactory` resolves the agent **by this name** via
`GET /v1/agents`. Rename the bundle and every PagerDuty-triggered investigation
fails to resolve. (Overridable via `OMNI_PD_BUNDLE_REF`, but the default is this.)

## Propose-only floor architecture

The bundle is propose-only — it investigates and recommends, it does not mutate
infrastructure. That floor is **not** the `guardrails.blast_radius` block in
`config.yaml` (which is "not a security boundary" by its own docstring; it is a
per-agent defense-in-depth backstop with `gate_pushes: true`). The floor is
enforced by three independent server- and host-side layers:

1. **Server-side read-only defaults** —
   `sei_omnigent.policies.read_only.read_only_default_policies`
   (`admin__github_read_only` + `admin__deny_mutating_os`), wired by the deploy.
   These deny `gh`/`git` writes and the OS file-mutation tools at runtime,
   regardless of cwd or which repo's `settings.json` loads.
2. **Credential absence (INV-3′)** — the runner is not given write credentials,
   so even an un-gated mutating call has nothing to authenticate with.
3. **Host egress sandbox (INV-11)** — the OS sandbox + egress allowlist + scoped
   read-roots enforced on the deployed host (the host NetworkPolicy + egress
   proxy, PLT-672 / the slice-3 merge gate), not declared in this spec.

`config.yaml`'s `os_env.sandbox.type: none` matches the standing claude-native
host's runner shape; the sandboxing lives on the host, not in the spec.

## harness: claude-native (required)

The spec pins `harness: claude-native`. The overlay's
`sei_omnigent/src/sei_omnigent/harness.py` invariant requires it and **rejects**
`claude-sdk`, which zeros the toolset to `["Skill"]` and wires no subagent
fan-out — breaking the multi-expert dispatch the discipline depends on. The
deployed host provides the tmux + `~/.claude` seed claude-native needs.
`permission_mode: auto` is required because a headless runner has no human to
answer ApprovalCards.

## Registration

Registered with the omni coordinator at boot:

    omnigent server --agent sei_omnigent/agents/root-cause

Per Design 13 this directory is baked into the coordinator image so the bundle
is resolvable the moment the PD route fires.

## OPEN DEPENDENCY — `.claude/agents/` roster provisioning (follow-up, NOT solved here)

The bundled `/root-cause` skill **hard-requires a `.claude/agents/` roster**.
SKILL.md's first guardrail (Step "Context check", and Step 2 "Dispatch the
expert slate" which reads `.claude/agents/`) halts when no roster is present —
the skill is multi-expert by design and refuses to run single-expert. The
PLT-715 prove-run hit exactly this halt.

So the headless `claude-native` runner needs Tide's `.claude/agents/`
specialists (and `.claude/skills/`) **provisioned in its environment** for the
discipline to actually run — via the host's `~/.claude` seed (#1201) or the
session workspace. That provisioning is **unresolved** and gates the agent from
exercising its discipline headless. This bundle does not solve it; track it as a
follow-up against the host-seed / workspace-provisioning work.
