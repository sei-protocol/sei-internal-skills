"""claude-native harness invariant + roster-discoverable guard (PLT-670).

Two Phase-1 invariants, both pure (no omnigent import) so they're unit-testable
and enforceable as a CI lint / a session-launch precondition:

1. **Harness invariant** (a hard *precondition* — raises). Every Tide session
   must launch under ``claude-native`` with ``skills_filter == "all"``.
   ``claude-sdk`` zeros the model's tool set to ``["Skill"]`` and wires no
   Task/subagent dispatch — it breaks Tide's ``/coral``//``/council`` fan-out.
   Any ``skills_filter`` but ``"all"`` changes how host setting-sources load
   (see #2), so the roster the session sees can't be confirmed — refused.

2. **Roster-discoverable guard** (a fail-closed *check* — returns a message; the
   caller raises). What the omnigent source actually establishes (verified
   against ``inner/bundle_skills.py:82-114`` + ``inner/claude_sdk_executor.py``
   ``:1004-1014``, omnigent==0.1.1): ``skills_filter: "none"`` emits
   ``--setting-sources ""``, which suppresses *host setting-source discovery*
   (``~/.claude/skills/`` and the cwd's project ``.claude/`` scope). omnigent
   documents this for **skills**; it does not document — and this module does
   **not** assert — that the same flag gates ``.claude/agents/`` by that exact
   mechanism. The roster degradation Tide relies on was observed *empirically* (a
   failed specialist dispatch under a restricted filter; design #11 C7), not
   proven from source. So the guard is deliberately conservative: it refuses any
   ``skills_filter`` but ``"all"``, any emitted ``--setting-sources ""``, and any
   roster smaller than the baseline. It fails **closed** — if the roster can't be
   confirmed present, abort rather than proceed on a silently-reduced roster
   (which looks like success to the model and corrupts multi-specialist output).

   (A ``list[str]`` filter does *not* emit ``--setting-sources ""`` — the SDK/CLI
   treat it like ``"all"`` for host sources, silently ignoring the named subset
   since there is no per-name allowlist flag. It is refused not because it
   suppresses the roster but because it is not ``"all"`` and yields a lossy,
   unconfirmable roster view.)

Scope (Phase-1, design #11 C7): the static, omnigent-free assertion + guard live
here and prove *config inputs* — not that the roster actually loaded. The
behavioral proof is the **one-shot canary dispatch** at session open (a live
Task-dispatch the roster must answer), which also closes the inputs this static
layer cannot observe: the launch **cwd** (project ``.claude/`` only loads when
cwd is the Tide repo — design §2.2/§2.5) and whether some *other* arg path
emitted ``--setting-sources ""`` (the independent ``setting_sources_suppressed``
signal below). The canary needs a running claude session and is deferred to the
session-launch wiring (PLT-672), like the header-posture behavioral test.
``--strict-mcp-config`` (named in design §2.5) does not appear in omnigent==0.1.1
source — no check is wired for a flag the pinned runtime never emits; re-confirm
on bump.

Non-goal: roster *tampering* (a malicious agent *added* to ``.claude/agents/``).
The count guard detects subtraction, not addition; content/digest pinning of the
roster is a separate control, out of scope here — ``.claude/agents/`` is
repo-controlled and sync-owned today.
"""

from __future__ import annotations

from pathlib import Path

#: The only harness Tide runs under (claude-sdk breaks subagent fan-out).
CLAUDE_NATIVE_HARNESS = "claude-native"

#: The only skills_filter that leaves host setting-source discovery at the CLI
#: default. Any other value changes --setting-sources handling (see module doc),
#: so the specialist roster can't be confirmed — refused conservatively.
REQUIRED_SKILLS_FILTER = "all"

#: Expected specialist-roster size, tracking ``Tide/.claude/agents/*.md``. CI
#: ties this to the synced agent count (see :func:`count_roster_agents`); a
#: drift (roster grew but baseline not bumped) shows up as a false-healthy guard,
#: so the two are checked equal in tests.
ROSTER_BASELINE = 19


def assert_harness_invariant(*, harness: str, skills_filter: object) -> None:
    """Raise unless a session launches under claude-native with skills_filter='all'.

    A hard launch *precondition*: an illegal harness/filter is a config/program
    error, so this raises ``RuntimeError`` directly. Contrast
    :func:`roster_discoverable_error`, which is a fail-closed *check* that returns
    a message for the caller to surface — the two error-surfacing styles are
    deliberate (precondition vs. runtime check), not an inconsistency. Enforced
    as a launch precondition + CI lint — not hoped-for at runtime.
    """
    if harness != CLAUDE_NATIVE_HARNESS:
        raise RuntimeError(
            f"Tide sessions must run under {CLAUDE_NATIVE_HARNESS!r}, not {harness!r}: "
            "claude-sdk zeros the tool set to ['Skill'] and wires no subagent "
            "dispatch, breaking /coral//council fan-out."
        )
    if skills_filter != REQUIRED_SKILLS_FILTER:
        raise RuntimeError(
            f"skills_filter must be {REQUIRED_SKILLS_FILTER!r}, not {skills_filter!r}: "
            "any other value changes host setting-source loading ('none' emits "
            '--setting-sources "", a list is silently treated like \'all\' with the '
            "named subset unenforced), so the specialist roster the session sees "
            "can't be confirmed. See the roster-discoverable guard."
        )


def roster_discoverable_error(
    *,
    skills_filter: object,
    setting_sources_suppressed: bool,
    discovered_agent_count: int,
    baseline: int = ROSTER_BASELINE,
) -> str | None:
    """Fail-closed roster-discoverability check (pure). Returns an error message
    if the roster cannot be confirmed fully discoverable from the session's
    *config inputs*, else None. Proving the roster actually *loaded* is the
    canary's job (PLT-672); this is the static, omnigent-free half.

    :param skills_filter: the resolved skills_filter for the session.
    :param setting_sources_suppressed: True if the resolved launch args emit
        ``--setting-sources ""`` (from *any* arg path, not only
        ``skills_filter="none"``). Computed by the launch wiring/canary from the
        actual CLI args — an independent signal from ``skills_filter``, so this
        check still bites if some other path suppresses the sources.
    :param discovered_agent_count: agents the session can actually dispatch
        (the canary's live count), or, statically, the count under the mounted
        ``.claude/agents/``.
    :param baseline: expected roster size.
    """
    if skills_filter != REQUIRED_SKILLS_FILTER:
        return (
            f"roster unconfirmable: skills_filter={skills_filter!r} (must be "
            f"{REQUIRED_SKILLS_FILTER!r}); any other value changes host "
            "setting-source loading, so the roster can't be confirmed intact."
        )
    if setting_sources_suppressed:
        return (
            'roster unconfirmable: --setting-sources "" is emitted, which '
            "suppresses host setting-source discovery (design #11 C7)."
        )
    if discovered_agent_count < baseline:
        return (
            f"roster incomplete: {discovered_agent_count} agents discoverable, "
            f"expected >= {baseline}. Refusing to proceed on a reduced roster."
        )
    return None


def count_roster_agents(agents_dir: Path) -> int:
    """Count specialist-agent definitions (``*.md``) under a ``.claude/agents/`` dir.

    Direct ``*.md`` children only — Claude Code discovers agents flat, so nested
    files are not part of the roster.
    """
    if not agents_dir.is_dir():
        return 0
    return sum(1 for p in agents_dir.glob("*.md") if p.is_file())
