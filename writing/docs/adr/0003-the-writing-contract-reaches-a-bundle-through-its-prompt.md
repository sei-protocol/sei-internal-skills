# 3. Deliver the writing contract to an agent bundle through its prompt and a baked gate

## Status

Accepted, 2026-09-08. Applies ADR 0001 to an Omnigent agent bundle, which that ADR did
not consider: it assumed a consumer repository with CI, not an agent in a sandbox.

## Context

`agents/sei-spec` authors specification prose and answered to no writing contract. Its
`config.yaml` and `README.md` held no reference to Vale, ASD-STE100, or this
repository's rules. Lane 1 of `AGENTS.md` names `vale specs/` as the verifier for
normative language, and a specification is prose, so Lane 2 governs it too.

A bundle constrains the delivery. It takes no guidelines file, and no pull request from
which to read a base branch, so the mechanism that works for the seidroid reviewer does
not transfer. Four shapes were available, and the choice turned on two measurements.

**Vale reads Markdown and lints no YAML.** `vale agents/sei-spec/config.yaml` reports
zero findings in zero files. A prompt is therefore the one artifact here that can state
the contract while sitting outside every check of it.

**The rules are in this repository, not only on a laptop.** They live in
`writing/styles`, 144K of rule files, already fetched by `writing-contract.yml` for
consumers. Nothing prevented baking them into an image.

**Which rules run depends on the path Vale receives, and the failure is silent.** The
contract is a directory convention: `.vale.ini` turns the spec rules on for
`specs/**/spec.md`. Vale matches that glob against the argument, not against the file
it opens. One specification carrying a lowercase `must` and four missing sections,
checked three ways:

| Invocation | Result |
|---|---|
| `cd <repo> && vale specs/001-x/spec.md` | 5 errors, exit 1 |
| `cd <repo> && vale /abs/specs/001-x/spec.md` | 0 errors, exit 0 |
| `cd specs/001-x && vale spec.md` | 0 errors, exit 0 |

An agent that lints from the directory it is working in gets a clean report on prose
the contract rejects.

## Decision

Take shape 1 and shape 3 together, as ADR 0001 requires, and reject shapes 2 and 4.

**The prompt carries the contract, generated from the registry.**
`render-context.py --target bundle-prompt` renders one section of
`agents/sei-spec/config.yaml` from `writing/anchors/registry.yaml` — the same source
`CONTEXT.md` comes from. It covers three parts: the normative language a requirement
carries, the eight anchors, and the banned list.
`writing/scripts/check-bundle-prompt.sh` diffs the section against the generator and
lints the rendered text as the Markdown it is. A paraphrase fails the gate, which
matters more than a deletion: it weakens the contract while still reading like it.

**The gate runs in the sandbox.** `Dockerfile.runner` installs Vale 3.17.1, checksum
verified, and bakes `writing/styles` with a configuration whose `StylesPath` is
absolute. `vale sync` runs at image build, so a sandbox needs no egress.
`scripts/sei-writing-lint` normalises every argument to a root-relative path and runs
from the root, which makes all three invocations above behave like the first. It finds
the root by walking up for `.git` rather than asking git, because git returns nothing
for a repository it considers unsafely owned, and a fallback to the working directory
is the silent under-check the wrapper exists to remove. With no root it refuses to run.

`verify-runner-image.yml` asserts reachability rather than presence: it builds the
overlay over a public stub base, then lints a known-bad specification with
`--network none` in all three invocation forms, and lints the Spec Kit template to
confirm the gate passes what it should.

## Consequences

Positive:

- The contract is a control in the session, not a hint. The agent can run the gate
  before it pushes, so a finding costs a rewrite rather than a review round.
- Three copies of the contract, one source. A registry edit reaches `CONTRACT.md`,
  `CONTEXT.md`, and the bundle prompt, and two gates hold them there.
- The wrapper closes the silent under-check for every caller, not only for `sei-spec`.
- No change to either `EXPECT` map, so the bundle's registration risk is unchanged.

Negative:

- A rule change reaches a sandbox only after an image rebuild and a digest bump in
  platform, because the image carries the rules. Both `ecr-runner.yml` and
  `verify-runner-image.yml` gained the rule paths, so the rebuild triggers. The deploy
  stays manual.
- Vale lands on every runner, not only on `sei-spec`. The layer is additive and small,
  and no other agent's prompt names it.
- The checksum covers the amd64 asset, matching the fleet. On another architecture the
  build fails at `vale --version` rather than shipping a broken binary.
- A gate reads patterns in finished text and cannot read meaning. The cycle reported in
  `writing/docs/design/closing-the-spec-loop.md` shipped four artifacts that agreed
  with each other and were wrong together, past a full mechanical verifier chain. This
  raises the floor and decides nothing about whether a requirement is the right one.

## Alternatives considered

**A ninth vendored skill.** Rejected on this repository's own doctrine, before cost.
Principle IV of `writing/CONTRACT.md` states that a skill exists only where a procedure
has side effects outside the repository, and that knowledge lives in the contract. A
writing contract carries knowledge.

The blast radius decides it independently. A bundled skill parses in strict mode, so
malformed frontmatter raises, and the caller skips the whole agent. A ninth skill would
put the Spec Kit agent's registration at the mercy of that file. It also needs a
one-line edit to two `EXPECT` maps that assert eight skills and a `speckit-` prefix.
And it is pull rather than push: the model must choose to load it.

**A verifier the agent installs at runtime.** Rejected. A sandbox routes egress through
a policy proxy, so the fetch is unmeasured and can fail mid-session, and an unpinned
download is the supply-chain gap `Dockerfile.runner` already calls out for the Spec Kit
pin. Baking moves the fetch to a build that a checksum guards.

**Review after the fact by `prose-steward`.** Rejected as the whole answer, and it
remains available beside this. It changes no bundle and stops nobody from writing
badly, which is the position `closing-the-spec-loop.md` records: independent
review found every defect, after implementation.

**Prompt text alone.** Rejected, and ADR 0001 already gives the reason: no
verification and no regression signal. Shipping it alone would describe an anchor as a
control, which Principle V of `writing/CONTRACT.md` forbids.
