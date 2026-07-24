# root-cause agent bundle

The headless root-cause investigator for the Sei platform stack — the PD-dogfood
agent of the omni session-routing service (Design 13). An alert routed through
the ControlPlane's PagerDuty route opens a session against this bundle, which
runs the `/root-cause` discipline and returns a ranked, evidence-backed analysis
with a **proposed** (never executed) remediation.

## MVP deployment — manual launch, Grafana-MCP signal envelope

The sections below frame this bundle for the PagerDuty-triggered path on the
standing claude-native host. The dev MVP runs the *same* bundle in a
narrower envelope, and three assumptions differ for it:

- **Manual, not PD-triggered.** An operator opens a managed session against this
  bundle; the automation trigger is a later increment.
- **`deny_shell=true`** (dev sets `SEI_OMNIGENT_DENY_SHELL=true`), *not* the
  `False` the propose-only floor section below assumes — so the `kubectl`/`curl`
  mutation classes are denied at the shell layer, a strictly stronger floor than
  the `blast_radius` backstop alone.
- **Grafana MCP is the only signal source** — metrics (Prometheus) + logs (Loki)
  via a loopback sidecar the *server* dials; no cluster API, no node RPC. So the
  runner has no *shell/OS/gh* mutating capability and no cluster/node credential
  to reach one; the Grafana MCP surface it *can* reach is held read-only by the
  sidecar flags (rollout gate 5 below), **not** by `deny_shell`. Out-of-envelope
  signals are listed in the signal ladder's "MVP deployment envelope" section.

## Layout

- `config.yaml` — the omni agent spec (`spec_version: 1`, `name: root-cause`).
- `skills/root-cause/` — a copy of sei-internal-skills's `/root-cause` skill (SKILL.md +
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
per-agent defense-in-depth backstop with `gate_pushes: true`). What enforces the
floor differs **per mutation class**, and no single layer covers all of them:

- **File mutation** (`Write` / `Edit` / `MultiEdit` / `NotebookEdit` and the
  `sys_os_*` / Pi equivalents) — DENYed by the server-side read-only default
  `admin__deny_mutating_os` (`sei_omnigent.policies.read_only`), which denies the
  file-mutation tools at runtime regardless of cwd or which repo's
  `settings.json` loads.
- **`gh` / `git` writes** — DENYed by `admin__github_read_only`
  (`omnigent.policies.builtins.github.github_policy` with `write_repos=[]` and
  `shell_tools` including `Bash`, so native `gh`/`git` shell writes are parsed,
  not abstained).
- **`kubectl` / `helm` / `terraform` apply/delete, force-push, `rm -rf`** — not
  covered by the read-only defaults (`admin__deny_mutating_os` abstains on raw
  shell: `deny_shell=False` is the deploy default, so it does NOT cover
  infra-mutation-via-shell). The per-agent `guardrails.blast_radius` backstop
  returns ASK/DENY for these. In the **headless** session an ASK **fails
  closed**: no human answers, the elicitation times out to refused
  (`omnigent/runner/pending_approvals.py` — `asyncio.TimeoutError → approved =
  False`) and the tool is BLOCKED, because `omnigent/runner/tool_dispatch.py`
  groups `POLICY_ACTION_ASK` with `POLICY_ACTION_DENY`. This is a distinct layer
  from Claude Code's `--permission-mode auto`, which only auto-approves Claude's
  own ApprovalCards — NOT omni's policy ASK.
- **Raw HTTP mutation** (`curl -X POST` / `-X DELETE`, or any write API
  reachable over the network) — matched by **no** policy pattern. Gated ONLY by
  **credential absence (INV-3′: the runner's RBAC/credentials are read-only)**
  plus the **host egress allowlist (INV-11: no write-capable endpoint reachable
  with ambient credentials)**. This is the **C4 residual** — there is no policy
  layer for it, only the two host-side controls below.
- **MCP tool calls** (the Grafana MCP surface this bundle adds under `tools:`) —
  matched by **no** policy pattern: `deny_shell` covers shell, the read-only
  defaults cover file/`gh`/`git`, none intercept an MCP `grafana__<tool>` call.
  Gated ONLY by the **sidecar launch flags** (`--disable-write
  --enabled-tools=datasource,prometheus,loki`) over a read-only Grafana **Viewer**
  SA — a second **C4-style residual**: upstream `grafana/mcp-grafana` defaults to
  write-enabled/all-tools, so the safe state is not the default (rollout gate 5).

`config.yaml`'s `os_env.sandbox.type: none` matches the standing claude-native
host's runner shape; the sandboxing lives on the host, not in the spec.

### Threat statement / rollout gates

The propose-only guarantee for the raw-shell and raw-HTTP infra-mutation classes
rests on operator-gated, host-side controls that this bundle does NOT enforce —
they MUST be verified before any live PagerDuty fire. These are hard rollout
gates, not solved by this bundle:

1. The runner ServiceAccount / kubeconfig is **read-only RBAC** (INV-3′) — so a
   raw `kubectl`/`curl` write has nothing to authenticate with.
2. The **egress allowlist** (the PLT-672 NetworkPolicy) contains **no
   write-capable endpoint reachable with ambient credentials** (INV-11).
3. The prod runner's effective `.claude/settings.json` is the **read-only** one.
   The host image bakes the read-only allowlist at `~/.claude/settings.json`
   (below); "effective" also merges the cloned workspace's project `.claude` —
   the server-side `admin__github_read_only` + `admin__deny_mutating_os` floor
   (not settings.json) is what makes "effective read-only" robust against a
   permissive workspace seed.
4. The host substrate manifest **must not mount a volume over `/home/host/.claude`**
   (or `/home/host`) — a runtime mount shadows the baked roster and reintroduces
   the PLT-715 Context-check halt that the build-time smoke (below) cannot see.
   (Long-term: a host readiness probe re-checking `~/.claude/agents/*.md` at
   container start closes this at runtime — owned by sre/platform.)
5. The **Grafana MCP sidecar is launched read-only** — `--disable-write
   --enabled-tools=datasource,prometheus,loki` over a Viewer service account —
   verified before any live run (manual MVP or PD fire). Upstream default is
   write-enabled, so an unflagged sidecar hands write-capable Grafana tools to a
   runner that ingests attacker-influenceable Loki logs. This is the checkable
   operator control for the MCP-tool mutation class above.

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

## DEPENDENCY — `.claude/agents/` roster via the host `~/.claude` seed (RESOLVED in the host image)

The bundled `/root-cause` skill **hard-requires a `.claude/agents/` roster**.
SKILL.md's first guardrail (Step "Context check", and Step 2 "Dispatch the
expert slate" which reads `.claude/agents/`) halts when no roster is present —
the skill is multi-expert by design and refuses to run single-expert. The
PLT-715 prove-run hit exactly this halt (a bare stand-up with no host image).

The headless `claude-native` runner gets the roster from the **host image**:
`Dockerfile.host` bakes the repo's `.claude/` overlay — `.claude/agents/` (the
specialist roster) + `.claude/skills/` (incl. `root-cause`) — into the runner's
`/home/host/.claude`, the user-scope path claude-code loads from. So every
claude-native runner inherits the full multi-expert discipline; this bundle does
not carry the roster itself (one maintenance point, minimal bundle), chosen over
bundling per-agent or a headless-adapted single-expert variant. The bake is
**build-time-verified**: `verify-sei-omnigent-image.yml`'s host-build-smoke
asserts `.claude/agents/*.md` and `.claude/skills/root-cause/SKILL.md` are
present in the built image — a dropped roster fails the build, not the first
prod fire (with the runtime mount-shadow caveat — rollout gate #4 above). The
baked `.claude/settings.json` is the **read-only** allowlist (read-only `gh`/`gh
api`/WebFetch; the one `gh issue comment` entry is a write the server floor
provably denies — clean it as a follow-up so the floor is a backstop, not the
sole stop). The read-only-ness is enforced by the server-side
`admin__github_read_only` + `admin__deny_mutating_os` floor, NOT by settings.json
alone — the build-smoke verifies *roster presence*, not the settings content, so
"effective read-only" rests on the floor, not on a CI assertion of the file.
What remains operator-gated is the host's prod **deployment** (the substrate
manifest + coordinator `--agent`, and the gate-#1/#2/#4 invariants above), not
the roster provisioning, which is built + build-verified here.
