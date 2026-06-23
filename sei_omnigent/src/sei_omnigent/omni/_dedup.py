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
``_runs`` that is benign — the runs they leased died with the process too. For ``_posted``
the restart double-propose window is now CLOSED at the PD layer, not here: the
``PagerDutyClient`` embeds a durable per-incident note marker and skips a re-post whose marker
already exists, so a re-fired-then-restarted incident that ``claim_post``s again is still a
no-op when the client checks PD. ``_posted`` is therefore a within-process fast-path + the
post-then-claim provisional slot (``unclaim_post`` releases it on a failed post so a corrected
re-post is not locked out), no longer the load-bearing once-guarantee.

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

    def claim_post(self, incident_key: str) -> bool:
        """Atomically claim the post slot for ``incident_key`` (the egress chokepoint).

        Returns ``True`` the first time a caller claims for this incident (→ proceed to
        post), ``False`` thereafter. The claim is PROVISIONAL under the post-then-claim
        ordering: the chokepoint claims, attempts the post, and :meth:`unclaim_post` releases
        the slot on a redaction/post failure so a corrected re-post is not locked out. Feeds
        ``engine.admit_post(incident_already_posted=not claim_post(...))``. This is a
        within-process fast-path, NOT the load-bearing once-guarantee — the durable
        double-propose close is the PagerDutyClient's per-incident note marker (a restart that
        loses this in-memory claim is still idempotent at the PD layer).
        """
        ...

    def unclaim_post(self, incident_key: str) -> None:
        """Release a provisional post-claim for ``incident_key`` (best-effort, idempotent).

        Called by the post-then-claim chokepoint when the redact-or-post failed, so the
        once-slot does not strand the incident: a corrected re-post can re-claim. Unclaiming a
        slot that was never claimed is a no-op. (Cross-restart double-propose is closed at the
        PD layer by the note marker, independent of this in-memory slot.)
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
    _posted: set[str] = field(default_factory=set, init=False, repr=False)

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

    def claim_post(self, incident_key: str) -> bool:
        with self._lock:
            if incident_key in self._posted:
                return False
            self._posted.add(incident_key)
            return True

    def unclaim_post(self, incident_key: str) -> None:
        with self._lock:
            self._posted.discard(incident_key)
