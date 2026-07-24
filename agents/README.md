# Agent bundles

Omni agent bundles — each a `config.yaml` (the omni agent spec) plus a `skills/`
directory carrying the discipline the bundle runs. These get baked into the
omnigent server's Docker image (`OMNIGENT_BUILTIN_AGENT_DIRS` points at them at
the in-image path) and registered as built-in agents on boot.

- `xreview/` — the headless multi-specialist code reviewer.
- `root-cause/` — the headless root-cause investigator.

Formerly `sei_omnigent/agents/`, alongside the custom omnigent overlay package.
The overlay is retired (the platform now runs vanilla omnigent directly); these
bundles are the only part of `sei_omnigent/` that's still live, relocated here
as a standalone concern.
