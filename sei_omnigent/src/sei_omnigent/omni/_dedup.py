"""Dedup / single-flight store seam for the trigger receiver (PLT-715) — no omnigent import.

``engine.admit_run`` / ``engine.admit_post`` are *pure predicates* that decode an
already-read bit; their docstrings state the hard precondition the store MUST satisfy:
the read-and-claim is **atomic** and the run-claim is **leased/TTL'd**. This module is
that store contract (:class:`DedupStore`) plus an in-memory implementation for the MVP /
single-replica deploy.

ATOMICITY (run-claim): :meth:`DedupStore.claim_run` reads-and-sets the in-flight bit for
an ``incident_key`` under one atomic step and returns whether the claim was won. A
non-atomic check-then-act lets two concurrent webhook deliveries both observe "not in
flight" and both launch, voiding single-flight (engine.admit_run's precondition).

LEASE (run-claim): the claim is TTL'd. A claim never released on crash would wedge the
incident permanently in SHED — even legitimate retries shed. The lease expiry is the
crash-recovery path: after ``lease_s`` a fresh trigger re-wins the claim.

Both state maps (``_posted`` and ``_runs``) are process-local and lost on restart. For
``_runs`` that is benign — the runs they leased died with the process too. ``_posted`` is the
within-process post fast-path + the post-then-claim provisional slot (``unclaim_post`` releases
it on a failed post so a corrected re-post is not locked out); it is NOT the durable
double-propose guard. The PD-layer guard is the ``PagerDutyClient``'s per-incident note
marker, which NARROWS but does not atomically close the double-propose window (PD's notes
endpoint has no conditional-create, so it remains best-effort under notes-list propagation
lag). RESIDUAL: a true cross-process once-guarantee needs a shared claim store (the
multi-replica un-defer).

Design 12 §2; engine.admit_run / engine.admit_post preconditions.
"""

from __future__ import annotations

import threading
import time
from collections.abc import Callable
from dataclasses import dataclass, field
from typing import Protocol


class DedupStore(Protocol):
    """The atomic, leased single-flight store ``engine.admit_run``/``admit_post`` require.

    Three operations, all keyed on the deterministic ``incident_key``
    (:func:`_alert.derive_incident_key`). An implementation MUST make :meth:`claim_run`
    and :meth:`claim_post` atomic read-and-set steps (compare-and-set / conditional write
    / per-key lock) — the engine predicates decode an already-claimed bit and cannot
    provide atomicity themselves.
    """

    def claim_run(self, incident_key: str, run_id: str, *, lease_s: float) -> bool:
        """Atomically claim the in-flight slot for ``incident_key`` under a ``lease_s`` TTL.

        Returns ``True`` if the claim was won (no live claim existed → caller PROCEEDs),
        ``False`` if a live (un-expired) claim already holds it (caller SHEDs — attach,
        not queue). The lease MUST expire after ``lease_s`` so a crashed run does not
        wedge the incident permanently in SHED.
        """
        ...

    def release_run(self, incident_key: str, run_id: str) -> None:
        """Release the in-flight claim for ``incident_key`` IFF ``run_id`` still owns it.

        Best-effort + owner-checked: a run releases only its OWN claim, so a release
        arriving after the lease expired and a fresh run re-claimed (a different
        ``run_id``) is a no-op rather than freeing the new run's claim. A missing release
        (crash) is covered by the lease expiry, not by this call.
        """
        ...

    def claim_post(self, incident_key: str, owner_token: str) -> bool:
        """Atomically claim the post slot for ``incident_key`` under ``owner_token``.

        Returns ``True`` the first time a caller claims for this incident (→ proceed to
        post), ``False`` thereafter. The claim is PROVISIONAL under the post-then-claim
        ordering: the chokepoint claims, attempts the post, and :meth:`unclaim_post` releases
        the slot (owner-checked) on a redaction/post failure so a corrected re-post is not
        locked out. Feeds ``engine.admit_post(incident_already_posted=not claim_post(...))``.
        This is a within-process fast-path, NOT a durable once-guarantee — the PD-layer guard
        is the PagerDutyClient's per-incident note marker (best-effort, narrows the window).
        """
        ...

    def unclaim_post(self, incident_key: str, owner_token: str) -> None:
        """Release a provisional post-claim for ``incident_key`` IFF ``owner_token`` owns it.

        Owner-checked (mirroring :meth:`release_run`): the chokepoint releases only the slot it
        itself claimed, so a release racing a different caller's fresh claim does not free that
        caller's slot (an ABA hazard a shared-store impl would otherwise carry). Unclaiming a
        slot that was never claimed, or one now held by a different owner, is a no-op.
        """
        ...


@dataclass
class _Lease:
    run_id: str
    expires_at: float


@dataclass
class InMemoryDedupStore:
    """Process-local atomic+leased :class:`DedupStore` for the MVP / single-replica deploy.

    Atomicity is a single ``threading.Lock`` around each read-and-set — correct WITHIN one
    process (the receiver's bg tasks + handler all share it), which is the single-replica
    deploy's scope. NOT correct across replicas (two pods have two locks + two maps) — the
    multi-replica deploy is the un-defer trigger for a shared store (module RESIDUAL).

    ``now`` is injected so the lease/expiry logic is unit-testable without sleeping (mirrors
    ``driver.py``'s injected clock). Defaults to ``time.monotonic`` — monotonic, NOT
    wall-clock, so a clock step (NTP slew) cannot retroactively expire or extend a lease.
    """

    now: Callable[[], float] = field(default=time.monotonic)
    _lock: threading.Lock = field(default_factory=threading.Lock, init=False, repr=False)
    _runs: dict[str, _Lease] = field(default_factory=dict, init=False, repr=False)
    #: incident_key → the owner_token that holds the provisional post-slot (owner-checked
    #: release, mirroring _runs). A non-membership means the slot is free.
    _posted: dict[str, str] = field(default_factory=dict, init=False, repr=False)

    def claim_run(self, incident_key: str, run_id: str, *, lease_s: float) -> bool:
        if lease_s <= 0:
            # A non-positive lease would expire instantly → every trigger PROCEEDs →
            # single-flight is voided. Reject it as the misconfiguration it is (mirrors
            # engine.Budget's positive-cap stance), rather than silently shedding nothing.
            raise ValueError(f"lease_s must be positive, got {lease_s!r}")
        with self._lock:
            now = self.now()
            existing = self._runs.get(incident_key)
            if existing is not None and existing.expires_at > now:
                return False  # a live claim holds it → SHED (attach, not queue)
            # Won the claim (no claim, or the prior one expired → crash-recovery path).
            self._runs[incident_key] = _Lease(run_id=run_id, expires_at=now + lease_s)
            return True

    def release_run(self, incident_key: str, run_id: str) -> None:
        with self._lock:
            existing = self._runs.get(incident_key)
            if existing is not None and existing.run_id == run_id:
                del self._runs[incident_key]

    def claim_post(self, incident_key: str, owner_token: str) -> bool:
        with self._lock:
            if incident_key in self._posted:
                return False
            self._posted[incident_key] = owner_token
            return True

    def unclaim_post(self, incident_key: str, owner_token: str) -> None:
        with self._lock:
            if self._posted.get(incident_key) == owner_token:
                del self._posted[incident_key]
