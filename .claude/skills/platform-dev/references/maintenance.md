# Keeping platform-dev current

The platform changes often, and this skill is only useful while it matches the platform. This file holds the checks that prove each fact, the platform paths whose change makes the skill stale, and the steps to land a fix. Every check here is read-only.

Contents:

1. [Verify sweep](#1-verify-sweep)
2. [Update triggers](#2-update-triggers)
3. [Land an update](#3-land-an-update)
4. [Rule-metric sweep](#4-rule-metric-sweep)
5. [Sweep record](#5-sweep-record)

## 1. Verify sweep

Run a row's check before a fact from it decides an action. Run every row for a full refresh. Run the repository checks in a checkout of `sei-protocol/platform` at `origin/main`. The bold query names resolve in [observability](observability.md#2-named-queries).

| Fact | Where the skill states it | Check |
|---|---|---|
| The cells, their regions and Terraform roots | `SKILL.md` mental model, architecture 1 and 5 | `ls -d terraform/aws/189176372795/*/*/` |
| Which cells run Cilium | `SKILL.md` mental model, architecture 1 | `grep -rl 'base/cni-cilium' clusters/*/` |
| The Flux shape of each cell | `SKILL.md` mental model, architecture 3 | `kubectl --context <cell> get fluxinstances -A` (Flux Operator) and `kubectl --context <cell> -n flux-system get kustomizations` |
| Fetch and apply intervals | `SKILL.md` change path, architecture 3 | `kubectl --context <cell> -n flux-system get gitrepositories,kustomizations -o custom-columns=NAME:.metadata.name,INTERVAL:.spec.interval` |
| Merge rules and the required check | architecture 3 | `gh api repos/sei-protocol/platform/rules/branches/main` |
| What the manifest check enforces | architecture 3 and 4 | Read `.github/workflows/k8s-manifest-check.yml` |
| Which objects carry the prune guard | `SKILL.md` guardrail 4, architecture 4 | `grep -rln 'kustomize.toolkit.fluxcd.io/prune: disabled' clusters manifests`, and the SeiNode query in architecture 4 |
| No Terraform state lock | `SKILL.md` guardrail 7, procedure C | `grep -rn 'use_lockfile' terraform/` returns nothing |
| The gateway shape | architecture 6 | **gateway-replicas**, and `kubectl --context <cell> -n gateway get pdb` |
| external-dns runs upsert-only | architecture 6, FM5 | `kubectl --context <cell> -n external-dns get deploy external-dns -o jsonpath='{.spec.template.spec.containers[0].args}'` |
| The federated cells | `SKILL.md` access, observability 1 | **cells-reporting** |
| The datasource uids | `SKILL.md` access, observability 1 | `mcp__sei-grafana__list_datasources` |
| Exporters that run on prod only | observability 2 and 6 | **metric-exists** on `istio_requests_total` and `certmanager_certificate_expiration_timestamp_seconds` |
| Every named query returns data | observability 2 | Run each one. A query that returns nothing on a healthy fleet, where the text says it should, is stale. |
| Every log selector matches | observability 3 | `sum(count_over_time(<selector>[24h]))` on `loki-fleet` for each row, with `cluster="prod"` |
| Alert routing per cell | observability 4 | `alerting_manage_routing` for prod. For the other cells, read the Alertmanager config under `clusters/<cell>/monitoring/` and `scripts/test_alertmanager_routes.py`. |
| Where each chain runs | architecture 9 | The SeiNode count loop in architecture 9 |
| The CI tool versions | procedure E | Read `.github/workflows/monitoring-config-validation.yml` |
| The release runbook path | procedure I | `gh api repos/sei-protocol/runbooks/contents/platform/chain-release` |
| The sibling skills' scope | `SKILL.md` description, routing, related | Read the `description:` of `/giga-dev`, `/harbor-dev`, `/grafana` and `/kubernetes` |

When a check disagrees with the skill, the live result wins. Tell the user what differs, and propose the fix through [section 3](#3-land-an-update).

## 2. Update triggers

A sei-protocol/platform pull request that changes one of these paths can make the skill stale. Check the named sections after it merges.

| Platform path | Skill sections to check |
|---|---|
| `terraform/aws/189176372795/**` (a new or removed root, a new cell) | `SKILL.md` mental model, architecture 1 and 5, procedures B and C |
| `clusters/*/flux-system/**`, a cell's `flux.tf` | `SKILL.md` mental model, architecture 3 |
| `clusters/base/**`, the Components | architecture 1 and 4 |
| `clusters/*/kustomization.yaml` (a cell gains or loses a directory) | architecture 9 |
| `manifests/base/monitoring/**`, `clusters/*/monitoring/**` | observability 2, 4 and 6, diagnosis, procedure E |
| `clusters/base/gateway/**`, `clusters/*/gateway/**`, external-dns, cert-manager | architecture 6, diagnosis FM5 to FM7, procedure F |
| `clusters/*/.sops.yaml` | architecture 7, procedure D |
| `clusters/harbor/engineers/**` | procedure G |
| `clusters/harbor/nightly/**`, the nightly workflows | architecture 9, diagnosis FM13 |
| `.github/workflows/k8s-manifest-check.yml`, the branch rules | architecture 3 |
| `.github/workflows/monitoring-config-validation.yml` | procedure E |
| `.agent/runbooks/**`, sei-protocol/runbooks `platform/chain-release/**` | procedures B and I |

A change in a sibling skill can also move this skill's routing. Check the description and the related list when `/grafana`, `/giga-dev`, `/harbor-dev` or `/kubernetes` changes scope.

## 3. Land an update

1. Branch in `sei-protocol/sei-internal-skills` from `origin/main`.
2. Edit `.claude/skills/platform-dev/`. Keep structure in the text, and keep counts and versions in the checks.
3. Update the [sweep record](#5-sweep-record) when you ran the full sweep.
4. Run `scripts/skill-package-checks.sh --skill-dir .claude/skills/platform-dev`, `make test-skill-package-checks` and `./writing/scripts/lint.sh` on the changed files.
5. Open a pull request titled `fix(platform-dev): <what changed>`. In the body, cite the platform pull request or the check output that shows the change.

## 4. Rule-metric sweep

A rule whose metric has no series never fires ([FM14](diagnosis.md#fm14-an-alert-never-fires)). This sweep finds every such rule.

1. List the metric names that each rule reads. In a platform checkout, run:

   ```sh
   python3 - <<'EOF'
   import glob, re, yaml
   SKIP = set("""sum min max avg count stddev stdvar topk bottomk quantile count_values group by without on
   ignoring group_left group_right bool and or unless offset rate irate increase delta idelta deriv
   predict_linear histogram_quantile label_replace label_join absent absent_over_time present_over_time
   avg_over_time min_over_time max_over_time sum_over_time count_over_time last_over_time
   quantile_over_time stddev_over_time changes resets time vector scalar clamp clamp_min clamp_max abs
   ceil floor round exp ln log2 log10 sqrt sort sort_desc timestamp inf nan""".split())
   files = glob.glob("manifests/base/monitoring/alerts/**/*.yaml", recursive=True) + \
           glob.glob("clusters/*/monitoring/**/*.yaml", recursive=True)
   for f in sorted(set(files)):
       if "/tests/" in f:
           continue
       for doc in yaml.safe_load_all(open(f)):
           if not isinstance(doc, dict) or doc.get("kind") != "PrometheusRule":
               continue
           for group in doc["spec"].get("groups", []):
               for rule in group.get("rules", []):
                   e = str(rule.get("expr", ""))
                   e = re.sub(r'"[^"]*"|\{[^}]*\}|\[[^\]]*\]', " ", e)
                   e = re.sub(r'\b(by|without|on|ignoring|group_left|group_right)\s*\([^)]*\)', " ", e)
                   e = re.sub(r'\b\d[\d.]*(e\d+|[smhdwy])?\b', " ", e)
                   for name in sorted(set(re.findall(r'[A-Za-z_:][A-Za-z0-9_:]*', e)) - SKIP):
                       print(f"{name}\t{rule.get('alert') or rule.get('record')}\t{f}")
   EOF
   ```

2. For each distinct metric name, run **metric-exists** through `/grafana`. Keep the names with an empty result.
3. Sort each empty name into one of four causes:
   - **Event-only.** The metric appears only while a job runs or after a first error, such as the nightly run metrics or a discard counter. Keep the rule.
   - **Renamed.** An exporter upgrade changed the name. Search with `list_prometheus_metric_names` for a close name. Fix the rule.
   - **Not scraped.** The component has no scrape target in a cell. Check `up{job=~".*<component>.*"}`. Fix the scrape configuration.
   - **Dev only.** The rule runs on dev, which Thanos does not hold. Check it on dev's own Prometheus.
4. Report the renamed and not-scraped rules to the user, with the rule, the file and the result. A fix is a platform pull request that the `sre-engineer` agent reviews.

## 5. Sweep record

| Date | Platform `main` | Result |
|---|---|---|
| 2026-10-13 | `6da7be91` | First release of the skill. Every named query and log selector returned data. The rule-metric sweep found rules to fix, and the user received the list. |

A record older than about one month means the skill has not had a full check for a while. Tell the user, and offer the sweep before you rely on a drift-prone fact.
