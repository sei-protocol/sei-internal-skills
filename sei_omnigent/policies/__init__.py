"""Sei policy handlers + config for the Omnigent overlay (PLT-668)."""

from sei_omnigent.policies.read_only import (
    READ_ONLY_POLICY_MODULES,
    deny_mutating_os,
    read_only_default_policies,
)

__all__ = [
    "deny_mutating_os",
    "read_only_default_policies",
    "READ_ONLY_POLICY_MODULES",
]
