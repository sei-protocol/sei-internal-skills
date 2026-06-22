"""sei_omnigent — Sei's out-of-tree overlay on Omnigent.

Phase-1 of the Sei Agentic Mesh's Omnigent adoption (design #11). This package
is the *seam* through which Sei owns the store/identity layer while adopting
Omnigent's runtime/harness/tunnel wholesale (the "hybrid line": adopt the glue,
own the substrate). It is carried out-of-tree against a **pinned** Omnigent
release — never a fork of the Omnigent source.

Pinned upstream: omnigent == 0.1.1  (see pyproject.toml). Every Omnigent symbol
this overlay depends on is imported through ``sei_omnigent._omnigent_shim`` so a
tag bump's drift hits exactly one file (the DECISION-1 adapter-shim discipline).

PLT-667 (this package's first slice) ships the store/identity seam with the
stock stores behind it; PLT-668/669/670/672 layer policy/auth/harness/deploy on
top; Phase-2/3 swap individual store implementations *through* this seam.
"""

__all__ = ["__version__", "PINNED_OMNIGENT"]

__version__ = "0.1.0"

# The Omnigent release this overlay is verified against. Bumping this is the
# moment to re-run the drift check on _omnigent_shim (the cloned boot sequence
# is the highest-watch surface — DECISION-1).
PINNED_OMNIGENT = "0.2.0"
