# sei-spec agent bundle

Spec-driven development on Omnigent, running GitHub Spec Kit and nothing else. An
engineer picks this agent, describes a feature, and the session walks the Spec Kit
phases: specify, clarify, plan, tasks, analyze, implement, converge. The artifacts
land in the repo's `.specify/` tree on the runner's checkout.

The bundle carries no Sei-authored workflow. It is a skills filter, a vendored copy
of the Spec Kit skills, and a prompt that names the phase order and the stop
conditions. Spec Kit needs no glue: `specify init . --integration claude` already
writes the layout Omnigent's skill discovery reads.

## Why This Exists

The motivation is collaboration, not spec discipline. Discipline was already
available through the Coral skills. What failed was sharing, because the artifacts
sat on a branch someone had to find and check out, and in practice nobody
collaborated. Omnigent's co-drive model puts the conversation and the artifacts in
the same place, so a teammate who attaches has their messages execute on the
runner's machine.

## Layout

- `config.yaml` — the omni agent spec (`spec_version: 1`, `name: sei-spec`).
- `skills/speckit-*/` — the eight vendored Spec Kit skills (SKILL.md each), from
  spec-kit 0.15.0. Auto-discovered from `<bundle>/skills/`.

Unlike the `xreview` and `root-cause` bundles, these skill bodies are upstream Spec
Kit, not copies of a sei-internal-skills skill. They are vendored byte-for-byte, so
the update path is a re-vendor from a newer spec-kit release rather than an edit
here.

## name ↔ Agent Selection

The name is what appears in the Omnigent agent dropdown and what a session's
`bundle_ref` resolves against via `GET /v1/agents`. Nothing machine-generated emits
`bundle_ref="sei-spec"` today, so a rename does not break a route the way it does
for `xreview` and `root-cause`. It does change what an engineer selects, so treat it
as a user-facing name.

## Hermeticity via skills: none Plus Vendored Bodies

`skills: none` switches off host-scope discovery only. Bundled skills load
unconditionally: the spec parser calls `_discover_skills(root / "skills")` with no
filter, while `discover_host_skills` returns an empty list the moment the filter is
`none` (`omnigent/spec/parser.py`). So the vendored eight survive and the Coral
skills cannot leak in regardless of what sits on the runner's machine.

This matters because Omnigent is not a separate lane from Claude Code.
`~/.claude/skills/` is scanned unconditionally for a host-scope agent, so a session
in a Spec Kit repo would otherwise discover both the Spec Kit skills and the Coral
skills, and with the default filter the model reaches for Coral on exactly the
deliberation-heavy work where Coral is stronger.

A name allowlist (`skills: [<eight names>]`) was the earlier design and is wrong for
this. It resolves against the runner host's filesystem at session time rather than
against the bundle, so it enumerates nothing on a machine that never ran
`specify init`, two engineers can run different method bodies, and a list of names
carries no content integrity.

### Hermetic Against Coral, Not Hermetic Generally

A live enumeration returned the eight vendored skills namespaced `sei-spec:speckit-*`
and zero Coral skills, from a working directory where ten Coral skills are visible to
any host-scope agent. It also returned nine skills that are in neither the bundle nor
`~/.claude/skills/`: `dataviz`, `simplify`, `loop`, `schedule`, `claude-api`,
`artifact-design`, `artifact-capabilities`, `update-config`, `keybindings-help`. Those
are the `claude-native` harness's own built-ins, which the `skills:` field does not
govern.

The goal holds, no rival methodology is reachable, but "sees only eight skills" is
false. If a built-in ever competes with a phase, and `simplify` is the nearest, the
lever is `safety.block_skills`.

## Excluded Spec Kit Skills

Two of the ten upstream skills are deliberately absent.

- `constitution`: there is no Sei constitution in v1, so the prompt's phase order
  opens at `specify`. This has a cost, see Known Limits.
- `taskstoissues`: it needs a GitHub MCP server this agent does not have, and its
  `tools:` frontmatter key is Copilot-flavoured, which Omnigent ignores.

## harness: claude-native (required)

`config.yaml` pins `harness: claude-native`. The skill bodies run shell scripts out
of `.specify/scripts/` and write files, and `claude-native` is the harness whose
toolset carries that. It is also the harness the live verification ran on.

## Registration

There is no `POST /v1/agents`. The only route into the dropdown is
`OMNIGENT_BUILTIN_AGENT_DIRS`, which names operator-supplied bundle paths the server
materialises at startup. This directory is meant to be baked into the server image so
the path is stable, the same way `xreview` and `root-cause` are.

Three properties an operator has to accept:

1. Registration runs only at lifespan startup, so every version bump restarts the
   shared multi-tenant server process.
2. It is keyed on directory name and refreshes in place, so it is mutable-latest
   rather than versioned. Rollback is redeploy plus restart, and there is no
   addressable `sei-spec@v1`.
3. A bad path is logged and skipped rather than fatal, so a typo yields the agent
   silently absent from an otherwise healthy server. Asserting `GET /v1/agents`
   after the restart is the only thing that catches this.

## Session Shape

A managed session is opened with `workspace` set to a git URL carrying an optional
branch fragment, Docker build-context style:

    https://github.com/sei-protocol/<repo>.git#<branch>

The server clones that `--single-branch` into the sandbox and starts the agent in the
clone, which is what `cwd: .` resolves to. Everything the flow needs is therefore
repository content plus the runner image, not the engineer's machine.

The agent owns git. Spec Kit computes a feature branch name but performs no git
operations at all, so the branch, the commits, and the push are the agent's to make.
It commits after each phase, pushes, and opens a draft pull request once `spec.md`
exists. That last part is load-bearing rather than tidiness: the sandbox is ephemeral,
the server retains the conversation but not the files, so an artifact that is never
pushed is gone when the session ends. The pull request is also what makes the work
readable to someone who was never in the session.

If the repository has no `.specify/`, the agent scaffolds it with
`specify init . --integration claude` and commits it. That requires the `specify` CLI
in the runner image. Note that `specify init` also writes `.claude/skills/speckit-*`
into the repository, which is inert for this agent because `skills: none` suppresses
host scope, and useful to anyone running Claude Code in that repo directly.

## Sandboxing Comes From The Deploy, Not This Bundle

The bundle sets `os_env.type: caller_process` and deliberately declares no sandbox,
write scoping, or credential posture. Declaring one would be pointless, because
`enforce_sandbox` never returns DENY: it returns ALLOW carrying a data override that
merges the admin sandbox over whatever the agent asked for.

On seigent that override is a real boundary. The sandbox provider is `kubernetes`,
runner pods land in `omnigent-sandboxes` under a deliberately powerless
`omnigent-runner` service account, and they are pinned to an IMDS hop-limited nodepool
so sandbox code cannot reach the node's instance role. Harness and git credentials
arrive as a mounted rotating token rather than as ambient laptop credentials.

That is a materially better posture than the laptop-runner topology this work was
originally scoped against, where the agent would have run unsandboxed with the
engineer's own credentials. The executor safety floor is still tracked separately
under PLT-872, and two things about the current posture are worth holding onto.
`write_paths` bounds writes and not reads, so a sandboxed agent can still read what
its token can read. And the mechanism that makes the admin sandbox win is that admin
defaults are concatenated last in the policy builder, an ordering invariant that is
undocumented upstream and fragile to reordering.

One consequence of the never-DENY behavior is easy to misread. `enforce_sandbox` never returns DENY. It
returns ALLOW with a data override that merges the admin sandbox over the agent's, so
an agent-supplied `sandbox: {type: none}` is silently overridden rather than refused,
and any acceptance test phrased as "is refused" will observe a successful session and
mislead you. What protects that override from an agent-supplied one is that admin
defaults are concatenated last in the policy builder, an ordering invariant that is
undocumented upstream and fragile to reordering.

## Known Limits

These were argued through review and accepted, so they are recorded here rather than
left to be rediscovered.

- **Drift defence is weaker than Coral's.** `converge` treats the artifacts as the
  sole source of intent and must not modify them, so it cannot detect a stale spec,
  only drag correct code back toward one. `analyze` runs before implementation and
  never reads code. Neither is fail-closed. Excluding `constitution` compounds this,
  because constitution conflicts are the only automatically-CRITICAL class in either
  skill. Human PR review is the backstop.
- **`converge`'s clean branch can be a false green.** It asserts the implementation
  satisfies intent while leaving `tasks.md` byte-for-byte unchanged, and against a
  stale spec that green light is actively misleading. This is the risk spec-driven
  development adds rather than one it removes.
- **Nothing decides whether a task deserves the ceremony.** Choosing the agent is
  itself the commitment, and it is made before the task is described. The
  one-sentence rule and the one-way-door stop in the prompt are advisory, because
  they are prompt-level.
- **One active feature per session, now enforced rather than hoped for.**
  `.specify/feature.json` is a repo-global pointer the phase scripts rewrite whenever
  they read it, so a second session on the same repository can move it. `common.sh`
  gives `SPECIFY_FEATURE_DIRECTORY` precedence over that file, so the prompt pins the
  feature through the environment variable for the life of the session. Co-driving one
  session was always safe. This is what makes two concurrent sessions on one
  repository safe as well.

## Preconditions Outside This Directory

- **The `specify` CLI in the runner image.** The agent scaffolds `.specify/` when a
  repository lacks it, and cannot without the CLI. The host image
  (`deploy/docker/Dockerfile.sei --target host` in the `bdchatham/omnigent` fork)
  carries git, `gh`, and the credential bridge, but not `specify`. It should be added
  with a pinned version, the same way `gh` and the other CLIs in that image are pinned
  and checksum-verified.
- **A git token that can push and open pull requests.** The runner image's credential
  bridge reads a rotating token from `/mnt/secrets/git/token` and exports it for both
  git and `gh`. Read-only scope is enough for `xreview`, which never writes. It is not
  enough for this agent, whose whole durability story is push plus pull request.
- **An admin for the deploy.** The `OMNIGENT_BUILTIN_AGENT_DIRS` mount is image and
  environment configuration with no API behind it, and the server-wide policy surface
  is `is_admin`-gated. Registration is what puts the agent in the console dropdown,
  which is the intended way engineers reach it.

## Lineage

- Ticket: PLT-872.
- Design: `bdchatham-designs`, `designs/sei-agentic-mesh/25-omnigent-speckit-agent.md`.
- The prior decision this reopens: `designs/sei-agentic-mesh/sdd-thin-layer.md`,
  which evaluated and rejected adopting Spec Kit. Its thin-layer work on `/design`
  and `/workstream` is untouched and still stands.
