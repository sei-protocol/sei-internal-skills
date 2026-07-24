"""Server-side read-only permission discipline (PLT-668).

Ports sei-internal-skills's read-only posture from per-repo CI (`settings.json` allow-list,
enforced by `make verify-agent-permissions`) to an **org-wide, server-side
Omnigent default policy** — enforced at runtime regardless of cwd or which
repo's `settings.json` loads.

Two server-default policies compose to make "read-only" true (research H3: both
the Omnigent policy layer AND Claude Code's own permission engine apply):

1. **`github_policy`** (an Omnigent builtin) — gates `gh`/`git` and GitHub-MCP
   tools. **Critical config:** `shell_tools=["Bash", "sys_os_shell"]`. Without
   `Bash`, native `gh`/`git` writes are unparsed and **fail open** (abstain →
   ALLOW) — `github_policy` defaults `shell_tools=("sys_os_shell",)` only, but
   claude-native surfaces shell as `Bash`. This single key is `signal-guard`-grade.
2. **`deny_mutating_os`** (this module) — denies the non-GitHub OS *file*-mutation
   tools that `github_policy` does not see (`Write`/`Edit` and their `sys_os_*` /
   Pi equivalents). github_policy covers GitHub; this covers the rest of the file
   surface so "read-only" is not merely "GitHub-write-blocked".

Shell coverage (deliberate boundary): `deny_mutating_os` **abstains** on raw
shell (`Bash`/`sys_os_shell`/`bash`) by default (`deny_shell=False`) and delegates
it to (a) `github_policy` for `gh`/`git`, (b) sei-internal-skills's per-repo `settings.json`
allow-list — both layers apply — and (c) the egress NetworkPolicy (PLT-672) for
network mutations (raw `curl`). A blanket shell DENY here would also block sei-internal-skills's
*read-only* shell (e.g. `kubectl get`), since the server DENY overrides
`settings.json`'s read allowance. Operators who run without sei-internal-skills's `settings.json`
and want a hard server-side shell block set `deny_shell=True` (denies ALL shell —
reads included, since the tool layer cannot classify a shell command). The raw-
`rm`-via-`Bash` residual gap under the default is the documented C4 boundary,
covered by `settings.json` + trusted-operator posture in Phase-1.
"""

from __future__ import annotations

from typing import TYPE_CHECKING, Any

if TYPE_CHECKING:
    # Types only, via the shim — keeps the "only the shim imports omnigent"
    # contract (this module makes zero runtime omnigent imports).
    from sei_omnigent._omnigent_shim import PolicyCallable, PolicyEvent, PolicyResponse

# Tool-name families: sys_os_* MCP tools · Claude/Codex native · Pi native
# (lowercase). Built from omnigent's `safety.py` families AND extended for the
# native edit set: omnigent/policies/builtins/safety.py::_NATIVE_OS_TOOLS omits
# `MultiEdit`/`NotebookEdit`, but omnigent's own server treats them as
# first-class native file-edit tools (server/routes/sessions.py, the
# `_CLAUDE_NATIVE_EDIT_TOOLS` frozenset — anchored by symbol since 0.6.0 moved
# its line) and they arrive verbatim in the PreToolUse tool_call event. They
# MUST be denied, or a native multi-file / notebook write slips through on a
# read-only server. (Re-verify this set against safety.py::_NATIVE_OS_TOOLS +
# sessions.py::_CLAUDE_NATIVE_EDIT_TOOLS on each pinned-tag bump.)
_READ_TOOLS = frozenset({"sys_os_read", "Read", "Glob", "Grep", "read"})
_FILE_MUTATION_TOOLS = frozenset(
    {
        "sys_os_write",
        "sys_os_edit",
        "Write",
        "Edit",
        "MultiEdit",
        "NotebookEdit",
        "write",
        "edit",
    }
)
_SHELL_TOOLS = frozenset({"sys_os_shell", "Bash", "bash"})

_ALLOW: PolicyResponse = {"result": "ALLOW"}


def deny_mutating_os(*, deny_shell: bool = False) -> PolicyCallable:
    """Factory: a server-default policy that DENYs non-GitHub OS file mutations.

    :param deny_shell: When ``True``, also DENY shell tools (``Bash``/
        ``sys_os_shell``/``bash``) — a hard server-side block that does not
        distinguish read from write shell (use only when sei-internal-skills's per-repo
        ``settings.json`` allow-list is *not* the governing layer). Default
        ``False``: abstain on shell and delegate to ``github_policy`` +
        ``settings.json`` + the egress NetworkPolicy.
    :returns: a ``PolicyEvent -> PolicyResponse`` callable.
    """

    def evaluate(event: PolicyEvent) -> PolicyResponse:
        """DENY file-mutation tools (and shell iff deny_shell); abstain (ALLOW) otherwise."""
        if event.get("type") != "tool_call":
            return _ALLOW
        data = event.get("data")
        if not isinstance(data, dict):
            return _ALLOW
        tool = data.get("name", "")

        if tool in _FILE_MUTATION_TOOLS:
            return {
                "result": "DENY",
                "reason": (
                    f"Read-only policy: {tool} is a file-mutation tool; "
                    "writes are not permitted on this server."
                ),
            }
        if deny_shell and tool in _SHELL_TOOLS:
            return {
                "result": "DENY",
                "reason": (
                    f"Read-only policy (deny_shell): shell tool {tool} is blocked; "
                    "no shell execution is permitted on this server."
                ),
            }
        # Reads, shell-when-delegated, and all non-OS tools: abstain (let
        # github_policy / settings.json / other defaults decide).
        return _ALLOW

    # evaluate matches the PolicyCallable structural type; mypy can't prove the
    # plain-dict return satisfies the PolicyResponse TypedDict shape.
    return evaluate  # type: ignore[return-value]


# Scanned by omnigent.policies.registry.load_registry(extra_modules=...) so the
# handler is allow-listed (attachable). Plain dicts, matching the builtin format
# (omnigent/policies/builtins/safety.py::POLICY_REGISTRY).
POLICY_REGISTRY: list[dict[str, Any]] = [
    {
        "handler": "sei_omnigent.policies.read_only.deny_mutating_os",
        "kind": "factory",
        "name": "Deny Mutating OS Tools (read-only)",
        "description": (
            "Server-side read-only backstop: DENYs non-GitHub OS file-mutation "
            "tools (Write/Edit and sys_os_*/Pi equivalents). Composes with "
            "github_policy (gh/git) and sei-internal-skills settings.json. Optionally denies "
            "shell via deny_shell."
        ),
        "params_schema": {
            "type": "object",
            "properties": {
                "deny_shell": {
                    "type": "boolean",
                    "description": (
                        "Also DENY shell tools (Bash/sys_os_shell/bash). Blocks "
                        "read-only shell too — use only without sei-internal-skills settings.json."
                    ),
                    "default": False,
                },
            },
            "required": [],
        },
    },
]


# Module to add to the server's `policy_modules` so deny_mutating_os is
# allow-listed (attachable) via load_registry(extra_modules=...).
READ_ONLY_POLICY_MODULES: list[str] = ["sei_omnigent.policies.read_only"]


def read_only_default_policies(*, deny_shell: bool = False) -> dict[str, Any]:
    """The canonical read-only server-default policy config (the cfg["policies"] block).

    Merge into the server config alongside ``policy_modules += READ_ONLY_POLICY_MODULES``;
    ``build_server`` feeds it to ``RuntimeCaps`` via ``parse_default_policies``. The
    deploy (PLT-672) wires this into the running config.

    NOTE (one-way door — config/audit contract): the policy *names* below
    (``admin__github_read_only`` / ``admin__deny_mutating_os``) are referenced by
    audit tooling; renaming after deploy breaks deployed configs. The ``admin__``
    prefix follows omnigent's documented convention for server-default policies
    (spec/parser.py ``parse_default_policies`` docstring examples), so an operator
    reading the config sees the standard server-default marker. (Operator decision,
    2026-06-16: adopt the upstream ``admin__`` convention over a Sei-specific
    prefix — open-standards alignment.)
    """
    return {
        "admin__github_read_only": {
            "type": "function",
            "function": {
                "path": "omnigent.policies.builtins.github.github_policy",
                "arguments": {
                    "read_all": True,
                    "write_repos": [],  # default-deny: no writes to any repo
                    # CRITICAL: include "Bash" or native gh/git writes fail OPEN.
                    "shell_tools": ["Bash", "sys_os_shell"],
                },
            },
        },
        "admin__deny_mutating_os": {
            "type": "function",
            "function": {
                "path": "sei_omnigent.policies.read_only.deny_mutating_os",
                "arguments": {"deny_shell": deny_shell},
            },
        },
    }
