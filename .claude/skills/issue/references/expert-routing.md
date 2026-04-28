# Expert Routing

How `/issue` discovers and suggests the **Relevant experts** field. The point of this field is routing: triagers should be able to assign without re-deriving who should look at the work.

## Discovery order

1. **Target repo's `.claude/agents/*.md`.** Authoritative when present. Each `.md` file's basename (minus `.md`) is the persona name; the frontmatter `description` and the body's first paragraph are signal for matching.
   - If CWD is the target repo, read the directory directly.
   - If targeting a different repo, the user is the source of truth — ask them which experts apply, or proceed with global personas.

2. **`AGENTS.md` at repo root** (if present). Often contains a curated table mapping work types to personas — much higher signal than scanning `.claude/agents/` blindly. Read this first when it exists.

3. **Global Claude personas** (fallback). The standard set: `kubernetes-specialist`, `platform-engineer`, `solidity-developer`, `network-specialist`, `security-specialist`, `tee-specialist`, `product-engineer`, `product-manager`, `opentelemetry-expert`. Also Sei-specific: `sei-network-specialist`. Use these names verbatim — they're recognized by `coral` / `council`.

4. **Repo-specific personas.** Some repos have their own (e.g. `blockchain-developer` and `reviewer` in Tide; `sei-network-specialist` in sei-k8s-controller). When a repo has its own, prefer it over a generic equivalent.

## Matching heuristics

Match the issue's **surface area** to personas, not the issue's **wording**.

| Surface area | Persona |
|---|---|
| Go controller / reconciler / CRD | `kubernetes-specialist` |
| K8s manifests, Kustomize, Helm, container images | `platform-engineer` |
| Solidity contract, EIP, ERC standard | `solidity-developer` (or `blockchain-developer` in Tide) |
| Service mesh, ingress, NetworkPolicy, cloud networking | `network-specialist` |
| Sei node networking specifically (CometBFT, Waterway, seid ports) | `sei-network-specialist` |
| Threat model, auth boundary, credential flow | `security-specialist` |
| TEE attestation, enclave bridges | `tee-specialist` |
| Product scope, MVP cut, deferral discipline | `product-manager` |
| Cross-component architecture, novel coordination patterns | `product-engineer` |
| Tracing, metrics, log pipelines, OTel SDK | `opentelemetry-expert` |
| Cross-component interface consistency | `reviewer` (Tide-specific) |

## Sizing the list

- **1 expert** — most issues. Single surface area, clean ownership.
- **2 experts** — issue spans a clean boundary (e.g. operator + manifest, or contract + indexer). Pick the boundary, not "everyone who might care."
- **3 experts** — cap. If the issue genuinely needs more, that's a signal it should be a /council workstream, not a single issue. Flag this to the user before filing.

The point is routing, not credit. Don't pad the list to acknowledge people.

## Confirm before rendering

Always show the user the suggested list and ask for adjustments before rendering the body. Format:

> Suggested experts: `kubernetes-specialist` (controller plan logic), `platform-engineer` (CRD field surface). Adjust?

Accept additions, removals, and replacements. The user owns the field — the skill suggests.
