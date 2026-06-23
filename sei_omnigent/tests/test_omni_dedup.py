"""Tests for the in-memory dedup/single-flight store (PLT-715).

Pure, omnigent-free. Covers the atomic+leased run-claim (SHED while live, re-win after
lease expiry = crash recovery), owner-checked release, and the post once-ness chokepoint.
The clock is injected so lease expiry is exercised without sleeping (mirrors driver tests).
"""

from __future__ import annotations

import pytest

from sei_omnigent.omni._dedup import InMemoryDedupStore


class _Clock:
    """A manually-advanced monotonic clock for lease tests."""

    def __init__(self) -> None:
        self.t = 0.0

    def __call__(self) -> float:
        return self.t

    def advance(self, dt: float) -> None:
        self.t += dt


def test_first_claim_wins_second_sheds() -> None:
    store = InMemoryDedupStore(now=_Clock())
    assert store.claim_run("inc-1", "run-a", lease_s=100.0) is True
    # A second trigger while the first claim is live → SHED (attach, not queue).
    assert store.claim_run("inc-1", "run-b", lease_s=100.0) is False


def test_distinct_incidents_do_not_collide() -> None:
    store = InMemoryDedupStore(now=_Clock())
    assert store.claim_run("inc-1", "run-a", lease_s=100.0) is True
    assert store.claim_run("inc-2", "run-b", lease_s=100.0) is True


def test_lease_expiry_lets_a_fresh_trigger_re_win() -> None:
    # The crash-recovery path: a claim never released (crashed run) must not wedge the
    # incident in SHED forever — after the lease, a fresh trigger re-wins.
    clock = _Clock()
    store = InMemoryDedupStore(now=clock)
    assert store.claim_run("inc-1", "run-a", lease_s=100.0) is True
    clock.advance(101.0)
    assert store.claim_run("inc-1", "run-b", lease_s=100.0) is True


def test_release_is_owner_checked() -> None:
    # A release from a run that no longer owns the claim (its lease expired + a fresh run
    # re-claimed) must NOT free the new run's claim.
    clock = _Clock()
    store = InMemoryDedupStore(now=clock)
    store.claim_run("inc-1", "run-a", lease_s=100.0)
    clock.advance(101.0)
    store.claim_run("inc-1", "run-b", lease_s=100.0)  # run-b now owns it
    store.release_run("inc-1", "run-a")  # stale owner — no-op
    assert store.claim_run("inc-1", "run-c", lease_s=100.0) is False  # run-b's claim survived


def test_release_frees_the_claim_for_the_owner() -> None:
    store = InMemoryDedupStore(now=_Clock())
    store.claim_run("inc-1", "run-a", lease_s=100.0)
    store.release_run("inc-1", "run-a")
    assert store.claim_run("inc-1", "run-b", lease_s=100.0) is True


def test_non_positive_lease_is_rejected() -> None:
    store = InMemoryDedupStore(now=_Clock())
    with pytest.raises(ValueError):
        store.claim_run("inc-1", "run-a", lease_s=0.0)
    with pytest.raises(ValueError):
        store.claim_run("inc-1", "run-a", lease_s=-1.0)


def test_post_claim_is_once_per_incident() -> None:
    store = InMemoryDedupStore(now=_Clock())
    assert store.claim_post("inc-1") is True
    assert store.claim_post("inc-1") is False  # the second post must shed (idempotent)
    assert store.claim_post("inc-2") is True  # a different incident is independent
