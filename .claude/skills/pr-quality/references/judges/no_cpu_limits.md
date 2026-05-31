# Judge: no_cpu_limits (mechanical)

Mechanical YAML-AST check. Not an LLM judge.

## Mechanism

1. Read every changed YAML file (`*.yaml`, `*.yml`).
2. Pre-render Helm/Kustomize where templating present:
   - If file path contains `templates/` and adjacent `Chart.yaml` exists → `helm template . > /tmp/rendered.yaml`
   - If `kustomization.yaml` adjacent → `kustomize build . > /tmp/rendered.yaml`
   - Else: parse the file directly.
3. YAML-AST walk: find every node at path `*.resources.limits.cpu` (in K8s container spec).
4. If found AND the value is set (not `null`, not empty string), emit finding.

## Output shape per finding

```json
{
  "verdict": "violation",
  "span": "<file>:<line>",
  "citation": "no_cpu_limits",
  "confidence": "high",
  "explanation": "CPU limit set; throttling is an anti-pattern. Set requests only, leave limits unset."
}
```

## Scope filter

- File path matches `*.yaml` or `*.yml`
- Skip files that ONLY contain `kind: Kustomization` (those don't have container specs)
- Skip files under `.claude/skills/**` (skill metadata, not workload spec)

## False-positive defenses

- Comment-only matches (`# limits.cpu: 500m`) — eliminated by YAML-AST parse.
- Helm/Kustomize templating residue — eliminated by pre-rendering.
- YAML anchors (`&cpuLimit`) — followed during AST walk.

## Cites

Memory: `feedback_no_cpu_limits` — "set CPU requests, leave limits unset; throttling is an anti-pattern (memory limits stay)"
