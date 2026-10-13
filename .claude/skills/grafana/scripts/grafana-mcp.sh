#!/usr/bin/env bash
# grafana-mcp.sh: run mcp-grafana read-only against prod Grafana. The script reads
# the agent-viewer token from the prod cluster, so no token is stored on disk.
# Claude Code starts it as the `sei-grafana` MCP server. README.md beside this
# file lists the exit codes.
set -euo pipefail

readonly GRAFANA_URL_DEFAULT="https://grafana.prod.platform.sei.io"
readonly TOKEN_CLUSTER="prod"
readonly TOKEN_REGION="eu-central-1"
readonly TOKEN_NAMESPACE="monitoring"
readonly TOKEN_SECRET="grafana-agent-viewer-token"
readonly MCP_GRAFANA_VERSION="0.17.2"
readonly ENABLED_TOOLS="datasource,prometheus,loki,alerting"
readonly SERVER_NAME="sei-grafana"

WORKDIR=""
TOKEN=""
BIN=""

usage() {
  cat <<'USAGE'
grafana-mcp.sh: read-only Grafana MCP server for prod Grafana.

  grafana-mcp.sh --aws-profile <name>                       serve MCP on stdio
  grafana-mcp.sh --aws-profile <name> --check               prove access, then exit
  grafana-mcp.sh --aws-profile <name> --register [--dry-run]
                                                            add the server to Claude Code

  --aws-profile <name>  AWS SSO profile with EKS access to the prod cluster.
                        Defaults to $AWS_PROFILE.
  --dry-run             print what the script would do, and change nothing.

Environment: SEI_GRAFANA_URL overrides https://grafana.prod.platform.sei.io.
USAGE
}

log() { echo "grafana-mcp: $*" >&2; }
die() {
  local code="$1"
  shift
  log "$*"
  exit "$code"
}

cleanup() {
  if [ -n "$WORKDIR" ]; then rm -rf "$WORKDIR"; fi
  WORKDIR=""
}
trap cleanup EXIT

# sha256 of each release archive, from mcp-grafana_0.17.2_checksums.txt.
release_sha256() {
  case "$1" in
    mcp-grafana_Darwin_arm64.tar.gz) echo 08e4a3193c484c7f5c1d377c8c25c63734026009023fc8e8f32e3e4bdde0f9db ;;
    mcp-grafana_Darwin_x86_64.tar.gz) echo 5381556cedf7ba79286fb920f2d245745d858af6fb252eae22efb3d91a81ef00 ;;
    mcp-grafana_Linux_arm64.tar.gz) echo bbe89103716e47b599d6b4d8e0961dce176744c855e7ec8ec372080112417cc0 ;;
    mcp-grafana_Linux_x86_64.tar.gz) echo 595543f740a2f69377e9f37deefdd399006378e6c3731d4627030f970a3b90c5 ;;
    *) return 1 ;;
  esac
}

require_tools() {
  local tool
  for tool in "$@"; do
    command -v "$tool" >/dev/null 2>&1 || die 3 "missing tool: $tool"
  done
}

make_workdir() {
  WORKDIR="$(mktemp -d "${TMPDIR:-/tmp}/grafana-mcp.XXXXXX")"
}

# Reads the token into TOKEN. The kubeconfig is temporary and holds no secret: it
# authenticates through `aws eks get-token` with the given profile.
fetch_token() {
  local profile="$1" kubeconfig="$WORKDIR/kubeconfig" err="$WORKDIR/err"
  aws eks update-kubeconfig --name "$TOKEN_CLUSTER" --region "$TOKEN_REGION" \
    --profile "$profile" --kubeconfig "$kubeconfig" >/dev/null 2>"$err" ||
    die 4 "cannot reach EKS cluster $TOKEN_CLUSTER with profile $profile. Run 'aws sso login --profile $profile'. AWS said: $(tail -n 1 "$err")"
  TOKEN="$(kubectl --kubeconfig "$kubeconfig" --namespace "$TOKEN_NAMESPACE" \
    get secret "$TOKEN_SECRET" --output 'jsonpath={.data.token}' 2>"$err" | base64 --decode)" ||
    die 4 "cannot read Secret $TOKEN_NAMESPACE/$TOKEN_SECRET on $TOKEN_CLUSTER. kubectl said: $(tail -n 1 "$err")"
  [ -n "$TOKEN" ] || die 4 "Secret $TOKEN_NAMESPACE/$TOKEN_SECRET on $TOKEN_CLUSTER has no token key"
}

# Sends one Prometheus query through Grafana. curl reads the header from stdin, so the
# token appears in no process argument list and no file.
probe_grafana() {
  local url="$1" body="$WORKDIR/probe" code
  code="$(printf 'Authorization: Bearer %s\n' "$TOKEN" | curl -sS -o "$body" -w '%{http_code}' -H @- \
    "$url/api/datasources/proxy/uid/prometheus-prod/api/v1/query?query=vector(1)")" ||
    die 5 "cannot reach $url"
  case "$code" in
    200) ;;
    401) die 5 "Grafana rejected the token (HTTP 401). It is expired or revoked. Ask the platform team to rotate it." ;;
    *) die 5 "Grafana answered HTTP $code to a Prometheus query: $(head -c 300 "$body")" ;;
  esac
}

binary_path() {
  echo "${XDG_CACHE_HOME:-$HOME/.cache}/sei-grafana-mcp/$MCP_GRAFANA_VERSION/mcp-grafana"
}

platform_asset() {
  local os arch
  case "$(uname -s)" in
    Darwin) os=Darwin ;;
    Linux) os=Linux ;;
    *) die 6 "no mcp-grafana build for OS $(uname -s)" ;;
  esac
  case "$(uname -m)" in
    arm64 | aarch64) arch=arm64 ;;
    x86_64 | amd64) arch=x86_64 ;;
    *) die 6 "no mcp-grafana build for CPU $(uname -m)" ;;
  esac
  echo "mcp-grafana_${os}_${arch}.tar.gz"
}

sha256_of() {
  if command -v shasum >/dev/null 2>&1; then
    shasum -a 256 "$1" | awk '{print $1}'
  else
    sha256sum "$1" | awk '{print $1}'
  fi
}

install_binary() {
  local bin="$1" asset want got url
  asset="$(platform_asset)"
  want="$(release_sha256 "$asset")"
  url="https://github.com/grafana/mcp-grafana/releases/download/v${MCP_GRAFANA_VERSION}/${asset}"
  log "downloading mcp-grafana $MCP_GRAFANA_VERSION ($asset)"
  curl -fsSL --retry 2 -o "$WORKDIR/$asset" "$url" || die 6 "download failed: $url"
  got="$(sha256_of "$WORKDIR/$asset")"
  [ "$got" = "$want" ] || die 6 "checksum mismatch for $asset: got $got, want $want"
  tar -xzf "$WORKDIR/$asset" -C "$WORKDIR" mcp-grafana || die 6 "cannot unpack $asset"
  # Copy to a private name in the cache directory, then rename. The rename stays on
  # one filesystem, so it is atomic: an interrupted or concurrent install never
  # leaves a partial binary at the path that later starts trust.
  local stage
  mkdir -p "$(dirname "$bin")"
  stage="$(mktemp "$(dirname "$bin")/.mcp-grafana.XXXXXX")"
  if ! { cp "$WORKDIR/mcp-grafana" "$stage" && chmod 0755 "$stage" && mv -f "$stage" "$bin"; }; then
    rm -f "$stage"
    die 6 "cannot install mcp-grafana to $bin"
  fi
}

ensure_binary() {
  BIN="$(binary_path)"
  [ -x "$BIN" ] || install_binary "$BIN"
}

self_path() {
  echo "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/$(basename "${BASH_SOURCE[0]}")"
}

serve() {
  local profile="$1" url="$2"
  require_tools aws kubectl curl tar base64
  make_workdir
  fetch_token "$profile"
  ensure_binary
  cleanup
  export GRAFANA_URL="$url" GRAFANA_SERVICE_ACCOUNT_TOKEN="$TOKEN"
  exec "$BIN" -t stdio --disable-write --enabled-tools "$ENABLED_TOOLS"
}

check() {
  local profile="$1" url="$2"
  require_tools aws kubectl curl tar base64
  make_workdir
  fetch_token "$profile"
  probe_grafana "$url"
  ensure_binary
  log "ok: $url answered a Prometheus query with the agent-viewer token. mcp-grafana $MCP_GRAFANA_VERSION is at $BIN."
}

register() {
  local profile="$1" dry_run="$2" self
  self="$(self_path)"
  local cmd=(claude mcp add --scope user "$SERVER_NAME" -- bash "$self" --aws-profile "$profile")
  if [ "$dry_run" = true ]; then
    log "would run: ${cmd[*]}"
    return 0
  fi
  require_tools claude
  "${cmd[@]}" >&2 || die 7 "'claude mcp add' failed. If $SERVER_NAME exists, remove it with 'claude mcp remove --scope user $SERVER_NAME' and run again."
  log "registered $SERVER_NAME. Restart Claude Code, or run /mcp, to load its tools."
}

plan() {
  local mode="$1" profile="$2" url="$3" bin
  bin="$(binary_path)"
  log "would read Secret $TOKEN_NAMESPACE/$TOKEN_SECRET on EKS $TOKEN_CLUSTER ($TOKEN_REGION) with profile $profile"
  if [ -x "$bin" ]; then
    log "would use $bin"
  else
    log "would download mcp-grafana $MCP_GRAFANA_VERSION ($(platform_asset)) to $bin and check its sha256"
  fi
  if [ "$mode" = check ]; then
    log "would send one Prometheus query to $url"
  else
    log "would run: $bin -t stdio --disable-write --enabled-tools $ENABLED_TOOLS (GRAFANA_URL=$url)"
  fi
}

main() {
  local profile="${AWS_PROFILE:-}" mode="serve" dry_run=false url
  while [ $# -gt 0 ]; do
    case "$1" in
      --aws-profile)
        [ $# -ge 2 ] || die 2 "--aws-profile needs a value"
        profile="$2"
        shift 2
        ;;
      --check) mode="check"; shift ;;
      --register) mode="register"; shift ;;
      --dry-run) dry_run=true; shift ;;
      -h | --help) usage; exit 0 ;;
      *) usage >&2; die 2 "unknown argument: $1" ;;
    esac
  done
  [ -n "$profile" ] || die 2 "no AWS profile. Pass --aws-profile <name>, or set AWS_PROFILE."
  url="${SEI_GRAFANA_URL:-$GRAFANA_URL_DEFAULT}"

  case "$mode" in
    register) register "$profile" "$dry_run" ;;
    check | serve)
      if [ "$dry_run" = true ]; then
        plan "$mode" "$profile" "$url"
      elif [ "$mode" = check ]; then
        check "$profile" "$url"
      else
        serve "$profile" "$url"
      fi
      ;;
  esac
}

main "$@"
