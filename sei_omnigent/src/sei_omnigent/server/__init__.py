"""Sei's custom Omnigent server entrypoint (the store/identity seam)."""

from sei_omnigent.server.serve import Stores, build_server, make_stores

__all__ = ["Stores", "build_server", "make_stores"]
