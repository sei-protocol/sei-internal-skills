"""claude-native harness invariant + roster-discoverable guard (PLT-670).

Two Phase-1 invariants, both pure (no omnigent import) so they're unit-testable
and enforceable as a CI lint / a session-launch precondition:

1. **Harness invariant.** Every Tide session must launch under ``claude-native``
   with ``skills_filter == "all"`` (and the project ``.claude/`` on the loaded
   setting sources). ``claude-sdk`` zeros the model's tool set to ``["Skill"]``
   and wires no Task/subagent dispatch — it breaks Tide's ``/coral``/``/council``
   fan-out. ``skills_filter`` ``"none"`` (or a named list) is forbidden — see #2.

2. **Roster-discoverable guard.** ``skills_filter: "none"`` emits
   ``--setting-sources ""`` (omnigent/inner/bundle_skills.py), which *silently*
   suppresses project ``.claude/skills/`` AND ``.claude/agents/`` — disabling
   Tide's entire ~19-agent specialist roster while only meaning to sandbox
   skills (empirically verified, design #11 §2.4a/§2.5). This guard fails
   **closed**: if the roster can't be confirmed present, abort — never proceed
   on a silently-empty roster (which looks like success to the model and
   corrupts multi-specialist output).

Scope (Phase-1): the static, omnigent-free invariant + guard live here. The
*one-shot canary dispatch* at session open (a live Task-dispatch probe that the
roster resolves) needs a running claude session and is the integration-time
complement — deferred to the session-launch wiring (PLT-672), like the
header-posture behavioral test. The wiring that calls these at launch is left to
where the harness session is constructed.
"""

from __future__ import annotations

from pathlib import Path

#: The only harness Tide runs under (claude-sdk breaks subagent fan-out).
CLAUDE_NATIVE_HARNESS = "claude-native"

#: The only skills_filter that keeps the project .claude/ (skills + agents)
#: discoverable. "none"/list suppress the roster.
REQUIRED_SKILLS_FILTER = "all"

#: Expected specialist-roster size, tracking ``Tide/.claude/agents/*.md``. CI
#: ties this to the synced agent count (see :func:`count_roster_agents`); a
#: drift (roster grew but baseline not bumped) shows up as a false-healthy guard,
#: so the two are checked equal in tests.
ROSTER_BASELINE = 19


def assert_harness_invariant(*, harness: str, skills_filter: object) -> None:
    """Raise unless a session launches under claude-native with skills_filter='all'.

    Enforced as a launch precondition + CI lint — not hoped-for at runtime.
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
            "'none'/list emit --setting-sources \"\", silently suppressing the "
            "project .claude/agents/ specialist roster."
        )


def roster_discoverable_error(
    *,
    skills_filter: object,
    setting_sources_suppressed: bool,
    discovered_agent_count: int,
    baseline: int = ROSTER_BASELINE,
) -> str | None:
    """Fail-closed roster-discoverability check (pure). Returns an error message
    if the specialist roster cannot be confirmed fully discoverable, else None.

    :param skills_filter: the resolved skills_filter for the session.
    :param setting_sources_suppressed: True if ``--setting-sources ""`` is emitted
        (suppresses project ``.claude/agents/``).
    :param discovered_agent_count: agents the session can actually dispatch
        (or, statically, the count under the mounted ``.claude/agents/``).
    :param baseline: expected roster size.
    """
    if skills_filter != REQUIRED_SKILLS_FILTER:
        return (
            f"roster suppressed: skills_filter={skills_filter!r} (must be "
            f"{REQUIRED_SKILLS_FILTER!r}); 'none'/list disables .claude/agents/."
        )
    if setting_sources_suppressed:
        return (
            "roster suppressed: --setting-sources \"\" is emitted, which disables "
            "project .claude/agents/ discovery."
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
