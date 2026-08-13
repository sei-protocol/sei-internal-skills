# experimental/

Skills and agents that are **not part of the shipped core**.

sei-internal-skills ships a focused set: the skills and agents an engineering team reaches for
on ordinary work. This folder holds everything else — resources that are still
being shaped, serve a narrow audience, or came out of a product exploration. They
keep their full history and stay runnable. They just do not ride along with a
default install.

## Nothing here ships by default

`make update`, `make sync-all`, `make bootstrap`, and the over-the-wire installer
all ignore this folder. `sync-skills.sh` and `sync-agents.sh` read `.claude/skills/`
and `.claude/agents/` and nothing else, so a resource is excluded **by living
here** — there is no category to set and none to drift out of sync.

To install them anyway, opt in by name:

```bash
make sync-experimental          # into ~/.claude
./scripts/sync-experimental.sh --target ~/work/some-repo --dry-run
```

## What's here

| Group | Resources |
|---|---|
| **Workflow orchestration** | `coral`, `council`, `workstream`, `issue`, `design`, `research` |
| **Exec reporting** | `impact-weekly`, `impact-portfolio`, `execution-plan` + `technical-program-manager` |
| **Deep-dive engineering** | `ebpf`, `bugbash` |
| **Recruiting** | `interview` + `sei-interview-expert` |

`language` and `prose-steward` are **not** here. `/xreview` pins `prose-steward`
unconditionally on any `skill-package` change and halts when a pinned steward is
missing from its registry, and `prose-steward`'s operating manual *is* `/language` —
so both are load-bearing for a core skill and stay in the core.

## Getting rid of one you already have

Parking a resource does not un-install it from environments that synced it earlier.

```sh
make prune-retired          # report what is stale. Deletes nothing.
make prune-retired-apply    # remove them
```

Removing a parked resource is reversible — `make sync-experimental` puts it back.

## Promoting and parking

Moving a resource is the whole mechanism — there is no manifest to update.

```bash
# Promote into the shipped core:
git mv experimental/skills/<name> .claude/skills/<name>
make verify-catalog     # the coverage guard now applies: its category: must map to an alias

# Park a core resource here:
git mv .claude/skills/<name> experimental/skills/<name>
```

Promotion is a real bar, not a formality. A resource belongs in the core when an
engineering team outside its author would reach for it on ordinary work, and when
it is stable enough that changing it is a considered act.

## The archive

Resources cut entirely in the 2026-08 slim-down — `data-mesh`, `prfaq`, `tee`,
`diagram`, and the `data-platform-architect`, `tee-specialist`, and
`diagram-architect` agents — are not here. They live with full history in the
snapshot taken before the cut:
[`bdchatham/sei-internal-skills-archive`](https://github.com/bdchatham/sei-internal-skills-archive).
