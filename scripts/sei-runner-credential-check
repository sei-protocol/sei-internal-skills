#!/bin/sh
# Assert the runner's GitHub credential bridge resolves a live credential for
# both gh and plain git. Meant as a startup / readiness probe: it runs in a
# bare non-login shell, so it exercises exactly the path a headless sandbox
# command hits, and fails loudly when a token is mounted but authentication
# resolves anonymous — a broken bridge then fails at deploy, not mid-review.
#
#   exit 0  authenticated, or no token mounted and not required
#   exit 1  token mounted but gh or git resolves anonymous / invalid
#
# Optional env knobs:
#   SEI_RUNNER_REQUIRE_GIT_TOKEN=1              a missing mount is a failure
#   SEI_RUNNER_CREDENTIAL_CHECK_SKIP_NETWORK=1  skip the gh API validity call
#                                               (local resolution still checked)
set -u

# Fixed PATH so gh resolves to the token-reading wrapper (not the real binary
# behind it) and git to the system install, whatever the caller's environment.
PATH=/usr/local/bin:/usr/bin:/bin:${PATH:-}
export PATH

TOKEN_FILE=/mnt/secrets/git/token
HOST=github.com
TAG=sei-cred-check

fail() { echo "$TAG: FAIL: $*" >&2; exit 1; }
info() { echo "$TAG: $*" >&2; }

if [ ! -s "$TOKEN_FILE" ]; then
	if [ -e "$TOKEN_FILE" ]; then
		fail "token file $TOKEN_FILE is present but empty"
	fi
	if [ "${SEI_RUNNER_REQUIRE_GIT_TOKEN:-0}" = 1 ]; then
		fail "no token mounted at $TOKEN_FILE and SEI_RUNNER_REQUIRE_GIT_TOKEN=1"
	fi
	info "no token mounted at $TOKEN_FILE; anonymous mode, nothing to assert"
	exit 0
fi

# gh bridge, local resolution: the PATH wrapper must export GH_TOKEN from the
# mount, so gh resolves a token without touching the network. 'gh auth token'
# prints it, or fails with 'no oauth token found'.
gh auth token --hostname "$HOST" >/dev/null 2>&1 \
	|| fail "gh resolved no token for $HOST; the gh PATH wrapper is not exporting GH_TOKEN from $TOKEN_FILE"

# gh bridge, API validity (network): confirms the token is accepted by GitHub,
# not merely present. Skip where readiness must not couple to GitHub reachability.
if [ "${SEI_RUNNER_CREDENTIAL_CHECK_SKIP_NETWORK:-0}" != 1 ]; then
	if ! status="$(gh auth status --hostname "$HOST" 2>&1)"; then
		printf '%s\n' "$status" >&2
		fail "gh auth status reports not authenticated to $HOST"
	fi
fi

# git bridge: drive the credential helper exactly as clone/fetch does and
# require a non-empty password. GIT_TERMINAL_PROMPT=0 keeps a missing credential
# a clean failure instead of an interactive prompt or hang.
cred="$(printf 'protocol=https\nhost=%s\n\n' "$HOST" \
	| GIT_TERMINAL_PROMPT=0 git credential fill 2>/dev/null)" \
	|| fail "git credential fill errored for https://$HOST"

printf '%s\n' "$cred" | grep -q '^password=.' \
	|| fail "git credential helper returned no password for https://$HOST; the git bridge is not reading $TOKEN_FILE"

info "OK: gh and git both resolve a credential for $HOST"
exit 0
