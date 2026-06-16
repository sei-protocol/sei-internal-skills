"""Tests for the read-only permission policy (PLT-668).

The handler is pure logic over a PolicyEvent dict, so these run without Omnigent
installed. They pin both the deny/abstain semantics and the load-bearing config
invariants the cross-review flagged (the github_policy shell_tools fail-open).
"""

from __future__ import annotations

from sei_omnigent.policies.read_only import (
    POLICY_REGISTRY,
    READ_ONLY_POLICY_MODULES,
    deny_mutating_os,
    read_only_default_policies,
)


def _call(tool: str, *, deny_shell: bool = False) -> dict:
    handler = deny_mutating_os(deny_shell=deny_shell)
    return handler({"type": "tool_call", "data": {"name": tool, "arguments": {}}})


def test_file_mutation_tools_are_denied() -> None:
    for tool in ("Write", "Edit", "sys_os_write", "sys_os_edit", "write", "edit"):
        assert _call(tool)["result"] == "DENY", f"{tool} must be DENY"


def test_read_tools_are_allowed() -> None:
    for tool in ("Read", "Glob", "Grep", "sys_os_read", "read"):
        assert _call(tool)["result"] == "ALLOW", f"{tool} must be ALLOW (abstain)"


def test_shell_abstains_by_default_and_denies_when_deny_shell() -> None:
    for tool in ("Bash", "sys_os_shell", "bash"):
        assert _call(tool)["result"] == "ALLOW", f"{tool} abstains by default"
        assert _call(tool, deny_shell=True)["result"] == "DENY", f"{tool} denied under deny_shell"


def test_non_os_tool_and_non_tool_call_events_allow() -> None:
    handler = deny_mutating_os()
    assert handler({"type": "tool_call", "data": {"name": "some_mcp_tool"}})["result"] == "ALLOW"
    assert handler({"type": "request"})["result"] == "ALLOW"
    assert handler({"type": "tool_call", "data": None})["result"] == "ALLOW"


def test_policy_registry_is_well_formed() -> None:
    assert isinstance(POLICY_REGISTRY, list) and POLICY_REGISTRY
    entry = POLICY_REGISTRY[0]
    assert entry["handler"] == "sei_omnigent.policies.read_only.deny_mutating_os"
    assert entry["kind"] == "factory"
    assert set(entry) >= {"handler", "kind", "name", "description", "params_schema"}
    assert "deny_shell" in entry["params_schema"]["properties"]


def test_github_policy_config_includes_Bash_shell_tool() -> None:
    """The load-bearing fail-open guard: without Bash, native gh/git writes abstain->ALLOW."""
    cfg = read_only_default_policies()
    gh = cfg["sei_github_read_only"]["function"]
    assert gh["path"] == "omnigent.policies.builtins.github.github_policy"
    assert "Bash" in gh["arguments"]["shell_tools"], "MUST include Bash or gh/git writes fail open"
    assert gh["arguments"]["write_repos"] == [], "default-deny: no write repos"


def test_config_wires_both_policies_and_module() -> None:
    cfg = read_only_default_policies()
    assert set(cfg) == {"sei_github_read_only", "sei_deny_mutating_os"}
    assert (
        cfg["sei_deny_mutating_os"]["function"]["path"]
        == "sei_omnigent.policies.read_only.deny_mutating_os"
    )
    # deny_mutating_os module must be in policy_modules so it is allow-listed.
    assert "sei_omnigent.policies.read_only" in READ_ONLY_POLICY_MODULES


def test_deny_shell_propagates_into_config() -> None:
    assert read_only_default_policies(deny_shell=True)["sei_deny_mutating_os"][
        "function"
    ]["arguments"]["deny_shell"] is True
