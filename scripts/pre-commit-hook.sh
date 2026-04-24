#!/usr/bin/env bash
# Pre-commit hook for Tide interface registry enforcement.
#
# Install:
#   cp scripts/pre-commit-hook.sh .git/hooks/pre-commit
#   chmod +x .git/hooks/pre-commit
#
# What it does:
#   If any staged files touch the interface boundary (Go in pkg/, Python in runtimes/,
#   the registry itself, or K8s manifests), runs scripts/verify_registry.py to catch
#   mismatches before commit.

REPO_ROOT="$(git rev-parse --show-toplevel)"
SCRIPT_DIR="$REPO_ROOT/scripts"

# Only run if interface-relevant files are staged
STAGED_FILES=$(git diff --cached --name-only)
SHOULD_CHECK=false

for f in $STAGED_FILES; do
    case "$f" in
        pkg/*.go|runtimes/*.py|tide/interface-registry.yaml|manifests/*.yaml)
            SHOULD_CHECK=true
            break
            ;;
    esac
done

if [ "$SHOULD_CHECK" = false ]; then
    exit 0
fi

echo "Verifying interface registry consistency..."
python3 "$SCRIPT_DIR/verify_registry.py" --repo-root "$REPO_ROOT"
exit $?
