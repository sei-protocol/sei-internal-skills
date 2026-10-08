#!/usr/bin/env bash
# Asserts the runner image's GitHub credential bridge (Dockerfile.runner) against a
# fake token on a fake mount, so it needs no secret and proves the wiring rather than
# the credential. A broken bridge is invisible at runtime: a runner with no credential
# clones nothing and reports that it read nothing, which reads as the agent failing.
#
# Run it inside the image as root, in a login shell, which is how the provider runs
# the host:
#
#   docker run --rm --user 0 -e SEI_RUNNER_CREDENTIAL_CHECK_SKIP_NETWORK=1 \
#     -v "$PWD/scripts/check-runner-credential-bridge.sh:/tmp/check-bridge.sh:ro" \
#     --entrypoint bash <image> -l /tmp/check-bridge.sh
#
# Do not set SEI_RUNNER_REQUIRE_GIT_TOKEN on the container. The image runs the
# readiness check from profile.d in every login shell, before this script writes the
# fake token, and with REQUIRE set that first run fails the shell. This script sets
# it per call instead.
set -euo pipefail

TOKEN=ghp_fakefakefakefakefakefakefakefake
mkdir -p /mnt/secrets/git
printf '%s' "$TOKEN" > /mnt/secrets/git/token

# gh resolves the mounted token through the PATH wrapper in a non-login shell, which
# is the shell the agent's tools get.
bash -c 'gh auth token --hostname github.com' | grep -q "^$TOKEN" \
  || { echo "FAIL: gh wrapper did not export GH_TOKEN from the mount"; exit 1; }
echo "  ok gh bridge"

# git, asserted with GIT_TOKEN unset and set: a launch can inject a credential as
# environment, and both scoping and order must hold when it does.
ENV_TOKEN=ghp_envenvenvenvenvenvenvenvenvenvenv
fill() { printf 'protocol=https\nhost=%s\n\n' "$1" | GIT_TERMINAL_PROMPT=0 git credential fill 2>/dev/null; }

git config --system --get-all credential.helper >/dev/null \
  && { echo "FAIL: an unscoped credential.helper is configured"; exit 1; }
echo "  ok no unscoped helper"

fill github.com | grep -q "^password=$TOKEN\$" \
  || { echo "FAIL: git helper returned no mounted token for github.com"; exit 1; }
echo "  ok git bridge"

GIT_TOKEN="$ENV_TOKEN" fill github.com | grep -q "^password=$TOKEN\$" \
  || { echo "FAIL: GIT_TOKEN won over the mounted rotating token"; exit 1; }
echo "  ok mounted token first"

if GIT_TOKEN="$ENV_TOKEN" fill evil.com | grep -q -e "^password=$TOKEN\$" -e "^password=$ENV_TOKEN\$"; then
  echo "FAIL: a token leaked to a non-github host"; exit 1
fi
echo "  ok host scoping"

command -v sei-runner-credential-check >/dev/null \
  || { echo "FAIL: sei-runner-credential-check absent"; exit 1; }
sei-runner-credential-check >/dev/null 2>&1 \
  || { echo "FAIL: the readiness check rejected a mounted token"; exit 1; }
echo "  ok readiness check accepts a mounted token"

# The readiness check fails closed when a token is required and absent, which is
# what makes it mean something.
rm -f /mnt/secrets/git/token

GIT_TOKEN="$ENV_TOKEN" fill github.com | grep -q "^password=$ENV_TOKEN\$" \
  || { echo "FAIL: with no mount, git did not fall back to GIT_TOKEN for github.com"; exit 1; }
echo "  ok GIT_TOKEN fallback"

if SEI_RUNNER_REQUIRE_GIT_TOKEN=1 sei-runner-credential-check >/dev/null 2>&1; then
  echo "FAIL: the readiness check passed with no token and REQUIRE set"; exit 1
fi
echo "  ok readiness check fails closed"

echo "credential-bridge-ok"
