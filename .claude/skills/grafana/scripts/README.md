# grafana scripts

## grafana-mcp.sh

`grafana-mcp.sh` runs [mcp-grafana](https://github.com/grafana/mcp-grafana) 0.17.2 read-only against `https://grafana.prod.platform.sei.io`. Claude Code starts it as the `sei-grafana` MCP server. The script has three modes:

| Mode | Flags | What it does |
|---|---|---|
| Serve | `--aws-profile <name>` | Reads the token, then runs `mcp-grafana -t stdio --disable-write --enabled-tools datasource,prometheus,loki,alerting`. Claude Code runs this mode. |
| Check | `--aws-profile <name> --check` | Reads the token, sends one Prometheus query through Grafana, makes sure that the binary is in place, and exits. |
| Register | `--aws-profile <name> --register` | Runs `claude mcp add --scope user sei-grafana -- bash <this script> --aws-profile <name>`. |

`--dry-run` prints what serve, check or register would do, and changes nothing. `--aws-profile` defaults to `$AWS_PROFILE`. `SEI_GRAFANA_URL` overrides the Grafana URL.

**The token.** At each start, the script reads Secret `monitoring/grafana-agent-viewer-token` from the `prod` EKS cluster (eu-central-1). It uses a temporary kubeconfig that authenticates through `aws eks get-token`. It never writes the token to disk or to a process argument list. Only the mcp-grafana process environment holds it. The platform repository holds the service account: `clusters/prod/monitoring/grafana-agent-viewer.yaml` in sei-protocol/platform.

**The binary.** On the first run, the script downloads the release archive for the host OS and CPU from GitHub. It checks the archive against a sha256 that the script pins. Then it installs the binary to `${XDG_CACHE_HOME:-~/.cache}/sei-grafana-mcp/0.17.2/mcp-grafana`. Later runs reuse that binary.

**Needs:** `aws` (v2), `kubectl`, `curl`, `tar`, `base64`, and `shasum` or `sha256sum`. Register mode also needs `claude`.

### Exit codes

| Code | Meaning | What to do |
|---|---|---|
| 0 | Success. Serve mode does not return: it runs mcp-grafana in its place. | None. |
| 2 | Usage error, or no AWS profile. | Pass `--aws-profile <name>`. |
| 3 | A needed tool is missing. The message names it. | Install the tool. |
| 4 | AWS or EKS access failed, or the token Secret is missing. | Run `aws sso login --profile <name>`. If the error persists, ask the platform team for EKS access to `prod`. |
| 5 | Grafana is unreachable, or it rejected the token. | On HTTP 401, ask the platform team to rotate the token. |
| 6 | The download, the checksum or the unpack failed, or no build exists for this host. | Do not bypass the checksum. Report the message. |
| 7 | `claude mcp add` failed. | If `sei-grafana` exists, remove it and register again. |
