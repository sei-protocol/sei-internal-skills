# Agent bundles

Omni agent bundles — each a `config.yaml` (the omni agent spec) plus a `skills/`
directory carrying the discipline the bundle runs. These get baked into the
omnigent server's Docker image (`OMNIGENT_BUILTIN_AGENT_DIRS` points at them at
the in-image path) and registered as built-in agents on boot.

- `xreview/` — the headless multi-specialist code reviewer.
- `root-cause/` — the headless root-cause investigator.
- `sei-spec/` — spec-driven development, running GitHub Spec Kit for a human who
  selects it from the agent dropdown.

`xreview` and `root-cause` are headless: a route resolves them by name and consumes
a returned verdict, so their bundle name is a machine contract. `sei-spec` is
interactive and co-driven, and it vendors upstream Spec Kit skills rather than a copy
of a sei-internal-skills skill, so its update path is a re-vendor from a newer
spec-kit release.

Formerly `sei_omnigent/agents/`, alongside the custom omnigent overlay package.
The overlay is retired (the platform now runs vanilla omnigent directly); these
bundles are the only part of `sei_omnigent/` that's still live, relocated here
as a standalone concern.
