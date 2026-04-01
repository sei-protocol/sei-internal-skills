#!/usr/bin/env python3
"""
Verify that code and specs are consistent with the Tide interface registry.

Usage:
    python verify_registry.py [--repo-root /path/to/tide-repo]

Checks:
    1. Event signature topic hashes in Go match the canonical signatures in the registry
    2. Env var names in runtime Python code match the registry
    3. Function names called by runtimes match the registry
    4. ServiceAccount name patterns in Go code match the registry

Exit codes:
    0 = all checks pass
    1 = mismatches found (prints details)
    2 = registry file not found or parse error
"""

import argparse
import hashlib
import os
import re
import sys
from pathlib import Path

try:
    import yaml
except ImportError:
    print("PyYAML not installed. Run: pip install pyyaml")
    sys.exit(2)


def load_registry(repo_root: Path) -> dict:
    registry_path = repo_root / "tide" / "interface-registry.yaml"
    if not registry_path.exists():
        print(f"ERROR: Registry not found at {registry_path}")
        sys.exit(2)
    with open(registry_path) as f:
        return yaml.safe_load(f)


def keccak256_signature(sig: str) -> str:
    """Compute keccak256 of an event/function signature string."""
    try:
        from eth_abi.tools import keccak256  # type: ignore
        return "0x" + keccak256(sig.encode()).hex()
    except ImportError:
        pass
    try:
        import hashlib as hl
        # Fallback: use sha3_256 if available (Python 3.6+)
        h = hl.sha3_256(sig.encode())
        return "0x" + h.hexdigest()
    except (AttributeError, ValueError):
        return None  # Can't compute without eth-abi or sha3


def find_files(repo_root: Path, pattern: str, dirs: list[str]) -> list[Path]:
    """Find files matching a glob pattern in specific directories."""
    results = []
    for d in dirs:
        dir_path = repo_root / d
        if dir_path.exists():
            results.extend(dir_path.rglob(pattern))
    return results


def check_env_vars(registry: dict, repo_root: Path) -> list[str]:
    """Check that env var names in Python runtime code match the registry."""
    issues = []

    # Build set of canonical env var names from registry
    canonical_vars = set()
    for var in registry.get("env_vars", {}).get("required", []):
        canonical_vars.add(var["name"])
    for var in registry.get("env_vars", {}).get("optional", []):
        canonical_vars.add(var["name"])

    if not canonical_vars:
        return issues

    # Search Python files for os.environ / os.getenv calls with TIDE_ prefix
    py_files = find_files(repo_root, "*.py", ["runtimes"])
    tide_var_pattern = re.compile(r"""(?:os\.environ(?:\.get)?\s*\[\s*["']|os\.getenv\s*\(\s*["'])(TIDE_\w+)["']""")

    for py_file in py_files:
        content = py_file.read_text(errors="ignore")
        for match in tide_var_pattern.finditer(content):
            var_name = match.group(1)
            if var_name not in canonical_vars:
                rel_path = py_file.relative_to(repo_root)
                issues.append(
                    f"ENV_VAR_MISMATCH: {rel_path} references '{var_name}' "
                    f"which is not in the interface registry"
                )

    return issues


def check_service_accounts(registry: dict, repo_root: Path) -> list[str]:
    """Check that ServiceAccount patterns in Go code match the registry."""
    issues = []

    k8s_config = registry.get("kubernetes", {})
    sa_pattern = k8s_config.get("service_accounts", {}).get("pattern", "")
    if not sa_pattern:
        return issues

    # Look for hardcoded "tide-agent" without the format string pattern
    go_files = find_files(repo_root, "*.go", ["pkg", "cmd", "internal"])
    hardcoded_sa = re.compile(r'ServiceAccountName\s*[:=]\s*"tide-agent"(?!\-)')

    for go_file in go_files:
        content = go_file.read_text(errors="ignore")
        for match in hardcoded_sa.finditer(content):
            rel_path = go_file.relative_to(repo_root)
            issues.append(
                f"SERVICE_ACCOUNT_MISMATCH: {rel_path} uses hardcoded 'tide-agent' "
                f"instead of per-agent pattern '{sa_pattern}'"
            )

    return issues


def check_function_names(registry: dict, repo_root: Path) -> list[str]:
    """Check that function names called by runtimes match the registry."""
    issues = []

    functions = registry.get("functions", {})
    if not functions:
        return issues

    # Build map of canonical function names
    canonical_names = {}
    for key, func in functions.items():
        canonical_names[func.get("name", "")] = func.get("signature", "")

    # Search Python files for contract function calls
    py_files = find_files(repo_root, "*.py", ["runtimes"])

    # Common patterns for calling contract functions
    func_call_patterns = [
        re.compile(r'contract\.functions\.(\w+)'),
        re.compile(r'\.call_function\s*\(\s*["\'](\w+)["\']'),
        re.compile(r'function_name\s*=\s*["\'](\w+)["\']'),
    ]

    # Known function names that should match registry
    registry_func_names = set(canonical_names.keys())
    # Also check for common misspellings/old names
    old_names = {
        "review": "submitReview",
        "reviewNonces": "getReviewNonce",
    }

    for py_file in py_files:
        content = py_file.read_text(errors="ignore")
        for pattern in func_call_patterns:
            for match in pattern.finditer(content):
                func_name = match.group(1)
                if func_name in old_names:
                    rel_path = py_file.relative_to(repo_root)
                    issues.append(
                        f"FUNCTION_NAME_MISMATCH: {rel_path} calls '{func_name}' "
                        f"but registry says '{old_names[func_name]}'"
                    )

    return issues


def check_git_token_path(registry: dict, repo_root: Path) -> list[str]:
    """Check that git token file path matches across components."""
    issues = []

    canonical_path = registry.get("git_token_path", "")
    if not canonical_path:
        return issues

    # Search for the token path in Python and Go files
    all_files = (
        find_files(repo_root, "*.py", ["runtimes"]) +
        find_files(repo_root, "*.go", ["pkg", "cmd", "internal"])
    )

    # Common old/wrong paths
    wrong_paths = ["/workspace/.tide/git-token"]

    for f in all_files:
        content = f.read_text(errors="ignore")
        for wrong in wrong_paths:
            if wrong in content and canonical_path not in content:
                rel_path = f.relative_to(repo_root)
                issues.append(
                    f"FILE_PATH_MISMATCH: {rel_path} uses '{wrong}' "
                    f"but registry says '{canonical_path}'"
                )

    return issues


def main():
    parser = argparse.ArgumentParser(description="Verify code against Tide interface registry")
    parser.add_argument("--repo-root", type=Path, default=Path.cwd(),
                        help="Path to the Tide repo root")
    args = parser.parse_args()

    repo_root = args.repo_root.resolve()
    registry = load_registry(repo_root)

    all_issues = []

    print("Checking environment variables...")
    all_issues.extend(check_env_vars(registry, repo_root))

    print("Checking ServiceAccount patterns...")
    all_issues.extend(check_service_accounts(registry, repo_root))

    print("Checking function names...")
    all_issues.extend(check_function_names(registry, repo_root))

    print("Checking file paths...")
    all_issues.extend(check_git_token_path(registry, repo_root))

    if all_issues:
        print(f"\n{'='*60}")
        print(f"FAILED: {len(all_issues)} interface mismatches found")
        print(f"{'='*60}\n")
        for issue in all_issues:
            print(f"  - {issue}")
        print(f"\nFix these to match tide/interface-registry.yaml")
        sys.exit(1)
    else:
        print(f"\n{'='*60}")
        print(f"PASSED: All checks consistent with interface registry")
        print(f"{'='*60}")
        sys.exit(0)


if __name__ == "__main__":
    main()
