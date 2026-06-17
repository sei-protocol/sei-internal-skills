"""Tests for the claude-native harness invariant + roster-discoverable guard (PLT-670).

Pure logic — no omnigent install needed. The count test ties ROSTER_BASELINE to
the actual Tide ``.claude/agents/`` so a roster that grows without a baseline
bump fails loudly (the false-healthy failure mode the guard exists to prevent).
"""

from __future__ import annotations

from pathlib import Path

import pytest

from sei_omnigent.harness import (
    CLAUDE_NATIVE_HARNESS,
    REQUIRED_SKILLS_FILTER,
    ROSTER_BASELINE,
    assert_harness_invariant,
    count_roster_agents,
    roster_discoverable_error,
)

# Tide/.claude/agents  (test file is Tide/sei_omnigent/tests/test_harness.py)
_AGENTS_DIR = Path(__file__).resolve().parents[2] / ".claude" / "agents"


def test_harness_invariant_accepts_claude_native_all() -> None:
    assert_harness_invariant(harness=CLAUDE_NATIVE_HARNESS, skills_filter=REQUIRED_SKILLS_FILTER)


def test_harness_invariant_rejects_claude_sdk() -> None:
    with pytest.raises(RuntimeError, match="claude-native"):
        assert_harness_invariant(harness="claude-sdk", skills_filter="all")


def test_harness_invariant_rejects_non_all_filter() -> None:
    # "none" and a named-list subset both fail — the invariant only accepts "all".
    with pytest.raises(RuntimeError, match="skills_filter"):
        assert_harness_invariant(harness=CLAUDE_NATIVE_HARNESS, skills_filter="none")
    with pytest.raises(RuntimeError, match="skills_filter"):
        assert_harness_invariant(harness=CLAUDE_NATIVE_HARNESS, skills_filter=["coral", "council"])


def test_roster_guard_passes_when_healthy() -> None:
    assert (
        roster_discoverable_error(
            skills_filter="all", setting_sources_suppressed=False,
            discovered_agent_count=ROSTER_BASELINE,
        )
        is None
    )


def test_roster_guard_fails_closed() -> None:
    # skills_filter != all
    assert roster_discoverable_error(
        skills_filter="none", setting_sources_suppressed=False, discovered_agent_count=99
    )
    # setting-sources suppressed via an *independent* arg path (skills_filter is
    # "all" yet the sources are suppressed) — the check is not subsumed by #1.
    assert roster_discoverable_error(
        skills_filter="all", setting_sources_suppressed=True, discovered_agent_count=99
    )
    # reduced roster
    err = roster_discoverable_error(
        skills_filter="all",
        setting_sources_suppressed=False,
        discovered_agent_count=ROSTER_BASELINE - 1,
    )
    assert err is not None and "roster incomplete" in err


def test_baseline_tracks_the_actual_roster() -> None:
    """ROSTER_BASELINE must equal the real synced agent count — else the guard is
    false-healthy. Bump ROSTER_BASELINE when the roster changes."""
    actual = count_roster_agents(_AGENTS_DIR)
    assert actual == ROSTER_BASELINE, (
        f"roster drift: {actual} agents under .claude/agents/ but ROSTER_BASELINE="
        f"{ROSTER_BASELINE}. Bump the baseline (and the deploy guard) to match."
    )
