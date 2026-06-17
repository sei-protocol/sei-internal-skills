# Sei × Omnigent — Phase-1 Kubernetes deploy (PLT-672B)

Kustomize manifests for the Phase-1 Sei Omnigent integration. Spec: LLD
`11-omnigent-harness-adoption-lld.md` §2.4 + §2.4a. Posture (locked one-way
doors): **single-tenant · single-replica · trusted-operator · header-mode auth
behind Sei SSO**.

> **REVERSIBLE AUTHORING.** These files are authored for cross-review and a
> human one-way-door checkpoint. **Do not `kubectl apply` until that sign-off.**
> Several values are deliberate `PLACEHOLDER_*` markers (table below); a real
> apply requires filling them and resolving the cross-review items.

Render with `kubectl kustomize overlays/sei` (or `kustomize build overlays/sei`).

---

## Topology (why it is shaped this way)

Two paths, one Service/port — isolation is by NetworkPolicy peer-selector +
credential, **not** separate listeners (one-way-door flag #10).

```
                         ┌─────────────────────────────────────────────┐
  Internet ─443/TLS──▶ Ingress ──▶ oauth2-proxy ──:8000──▶ omnigent-server
                         (PLACEHOLDER       │  strips+resets              │  :8000
                          ingressclass)     │  X-Forwarded-Email          │
                                            │  from verified IdP claim    │
                                                                          │
              omnigent-host ──────────WSS /v1/hosts/{id}/tunnel──────────▶│
                (X-Omnigent-Host-Token; bypasses ingress + proxy entirely)│
                                                                          ▼
                                                              postgres (in-cluster)
```

- **Human path:** validated by oauth2-proxy, which is the **Ingress backend**
  (not an `auth-url` annotation — that leaves the app L7-routable). The proxy
  strips any inbound `X-Forwarded-Email` and re-sets it from the verified claim.
  The server's `_check_header` trusts that header verbatim, so the strip is the
  load-bearing impersonation control.
- **Host path:** the agent host dials the server ClusterIP directly with the
  managed launch token header; that branch returns before `get_user_id`, so
  `X-Forwarded-Email` is structurally inert there.

---

## ⚠️ CNI enforcement is a hard prerequisite

The server binds `0.0.0.0:8000`. The header-mode trust model depends **entirely**
on the CNI actually **enforcing NetworkPolicy** (Cilium or Calico). If the CNI
ignores NetworkPolicy (vanilla kubenet, or a flannel build without policy
support), **any in-cluster peer that reaches `:8000` can set `X-Forwarded-Email`
and impersonate any user, including admin.** With no enforcement the
NetworkPolicies are inert and this deploy is a blocking security finding.

Additionally: **FQDN egress filtering needs Cilium.** The `:443` egress
allowlists are written as `ipBlock` CIDRs (the vanilla fallback). On Cilium,
prefer `CiliumNetworkPolicy` `toFQDNs` for the GitHub/IdP/MCP/chain-RPC
allowlist — CIDR ranges for those endpoints drift and over-admit. CIDR-only is a
tracked degradation, not the target state.

---

## Apply order (AFTER checkpoint sign-off)

1. **Namespace + CNI check.** Confirm the namespace has the
   `kubernetes.io/metadata.name` label (used by the DNS + ingress-controller
   NetworkPolicies) and that the CNI enforces policy.
2. **Secrets** (via SealedSecrets/ExternalSecrets — never the raw
   `secrets.example.yaml`): `omnigent-ghcr`, `oauth2-proxy`, `postgres`. Create
   a throwaway `omnigent-host-token` so the host pod can schedule (the arm Job
   overwrites it).
3. **NetworkPolicies + RBAC + ConfigMap + PVCs** (the base; the default-deny
   lands first within the set).
4. **Postgres** StatefulSet + Service; wait for `pg_isready`.
5. **omnigent-server** — first boot runs the ~1min Alembic migration; the
   `startupProbe` gives 150s. Wait for Ready.
6. **oauth2-proxy + Ingress.**
7. **Arm the host** — run the `omnigent-host-arm` Job (see manual bootstrap).
8. **omnigent-host** Deployment — it dials the tunnel with the armed token.
9. **Enable `omnigent-host-rearm` CronJob.**

`kubectl kustomize overlays/sei | kubectl apply -f -` applies the whole set; the
ordering above matters mainly for first-boot (DB before server, arm before host).

---

## Manual bootstrap / pre-deploy gates

These are **not** automated and gate the deploy:

1. **PLT-675 — the 0.1.0→0.1.1 auth-path re-diff.** Re-verify the header-mode
   auth path and the `X-Forwarded-Email` trust against the pinned 0.1.1 image
   before deploy (the LLD's auth analysis was against the prior line). Confirm
   `_check_header` / `UnifiedAuthProvider.get_user_id` still behave as §2.4a
   describes.
2. **Arm the host (PLT-675).** The `omnigent-host-arm` Job mints the managed
   launch token, calls `register_managed_host(...)` against the server DB, and
   writes the raw token into the `omnigent-host-token` Secret. The arm script is
   now **implemented** (real `python -c` against omnigent's `HostStore`, fail-
   closed; the DSN comes from the `postgres` Secret's `DATABASE_URI` key via
   `secretKeyRef`, **no plaintext password in pod env** — FIX 4/5). Two items
   remain **pre-apply gates**: (i) re-verify the `register_managed_host`
   keyword-only signature (`host_store.py:524`) and that a synthetically-armed
   managed host is accepted by the tunnel auth branch **against the pinned
   omnigent 0.1.1 image** — this path is "off the intended launcher path"
   (DECISION-1 drift surface); (ii) confirm the pinned image carries the
   `kubernetes` python client for the Secret write (else use the `kubectl`
   fallback noted in `host-arm-job.yaml`).
3. **PLT-674 — live-rotation test.** Verify whether re-arming (token rotation +
   the host re-reading the Secret) re-auths the **existing** live tunnel or only
   the **next** dial. If only the next dial, schedule a rolling restart of
   `omnigent-host` shortly after each re-arm. See `host-rearm-cronjob.yaml`.
4. **Confirm `oauth2-proxy` header mapping flags** against the pinned
   oauth2-proxy version (v7.6.0 here). `--pass-user-headers=true` is the entire
   impersonation control — it SETS `X-Forwarded-Email` from the validated
   session and overwrites any client-supplied value. `--set-xauthrequest` is
   deliberately ABSENT (it emits the `X-Auth-Request-*` family used by the
   rejected nginx auth_url pattern, not what the server reads). The Ingress also
   clears the identity headers at the edge (`more_clear_input_headers`) as
   defense-in-depth.
5. **NEGATIVE TEST (pre-trust).** A request carrying a forged
   `X-Forwarded-Email: attacker@…` through the **full Ingress → oauth2-proxy**
   chain MUST NOT authenticate as that user. Run this before trusting the
   deploy: the Ingress edge strip + oauth2-proxy's overwrite must both hold.
6. **Active CNI-enforcement check (fast-follow, defense-in-depth).** Beyond this
   human README gate, add a boot/Job probe that confirms a *should-be-denied*
   connection (e.g. an unprivileged pod dialing `omnigent-server:8000`, or the
   host reaching a non-allowlisted `:443`) is **actually denied**. The header
   trust model is inert if the CNI silently ignores NetworkPolicy; an active
   probe catches that, a static manifest cannot.

---

## Secrets — never commit real material

`secrets.example.yaml` is a **template only** and is **not** referenced by any
`kustomization.yaml`. Provision the real Secrets with **SealedSecrets** (commit
ciphertext) or **ExternalSecrets** (commit a backend reference). Required:

| Secret | Holds |
| --- | --- |
| `omnigent-ghcr` | dockerconfigjson for the private GHCR pull (`read:packages` PAT) |
| `oauth2-proxy` | OIDC issuer/client-id/client-secret + cookie-secret |
| `postgres` | `POSTGRES_DB`/`USER`/`PASSWORD` + `DATABASE_URI` (the full DSN the arm/re-arm Jobs read via `secretKeyRef`) |
| `omnigent-host-token` | `OMNIGENT_HOST_TOKEN` — **written by the arm Job**, not pre-seeded |

---

## PLACEHOLDER_* — what an operator must supply

| Placeholder | Where | Meaning |
| --- | --- | --- |
| `PLACEHOLDER_NAMESPACE` | `overlays/sei/kustomization.yaml`, oauth2-proxy `--upstream`, host `--server` | Target namespace (must carry `kubernetes.io/metadata.name`) |
| `PLACEHOLDER_IMAGE` | overlay `images:` + all pod specs | GHCR image repo (e.g. `ghcr.io/...`) |
| `PLACEHOLDER_TAG` | overlay `images:` | **Pinned** tag/sha — never `:latest` |
| `PLACEHOLDER_STORAGECLASS` | `server-pvc.yaml`, `postgres-statefulset.yaml` | StorageClass for the PVCs |
| `PLACEHOLDER_ADMIN_EMAIL` | `server-config.yaml` `admins:` | 1–2 break-glass admin identities (verified IdP emails) |
| `PLACEHOLDER_ALLOWED_DOMAIN` | `server-config.yaml`, oauth2-proxy `--email-domain` | Sei IdP email domain allow-list |
| `PLACEHOLDER_PG_PASSWORD` | `server-config.yaml` DSN, `secrets.example.yaml` (`POSTGRES_PASSWORD` + `DATABASE_URI`) | Postgres password (keep in sync; the arm/re-arm Jobs no longer inline it — they read the `postgres` Secret's `DATABASE_URI` key. Prefer a managed DSN to retire the ConfigMap copy) |
| `PLACEHOLDER_HOST_ID` | host Deployment, arm Job, re-arm CronJob | The stable `host_<32hex>` id (same across all three) |
| `PLACEHOLDER_HOSTNAME` | `ingress.yaml` | Public FQDN for the SPA |
| `PLACEHOLDER_INGRESSCLASS` | `ingress.yaml` | IngressClass (annotations assume nginx; translate if not) |
| `PLACEHOLDER_TLS_ISSUER` | `ingress.yaml` | cert-manager ClusterIssuer |
| `PLACEHOLDER_OIDC_ISSUER_URL` | `secrets.example.yaml` (`oauth2-proxy`) | Sei IdP OIDC issuer URL |
| `PLACEHOLDER_OIDC_CLIENT_ID` / `_SECRET` | `secrets.example.yaml` | OIDC client credentials |
| `PLACEHOLDER_OAUTH2_COOKIE_SECRET` | `secrets.example.yaml` | 16/24/32 random bytes, base64 |
| `PLACEHOLDER_GHCR_DOCKERCONFIGJSON` | `secrets.example.yaml` | GHCR dockerconfigjson |
| `PLACEHOLDER_HOST_TOKEN_OVERWRITTEN_BY_ARM_JOB` | `secrets.example.yaml` | Throwaway; arm Job overwrites |
| `PLACEHOLDER_INGRESS_CONTROLLER_NAMESPACE` | `networkpolicies.yaml` | Namespace of the ingress controller |
| `PLACEHOLDER_INGRESS_CONTROLLER_LABEL_KEY` / `_VALUE` | `networkpolicies.yaml` | Pod label selector for the ingress controller |
| `PLACEHOLDER_CIDR_GITHUB` | `networkpolicies.yaml` (server + host egress) | GitHub `:443` CIDR(s) |
| `PLACEHOLDER_CIDR_IDP` | `networkpolicies.yaml` (server + oauth2-proxy egress — **NOT host**: the host does no OIDC) | Sei IdP `:443` CIDR(s) |
| `PLACEHOLDER_CIDR_MCP` | `networkpolicies.yaml` (server + host egress, conditional) | Approved MCP endpoint CIDR(s) |
| `PLACEHOLDER_CIDR_CHAIN_RPC` | `networkpolicies.yaml` (server + host egress, conditional) | Chain RPC CIDR(s) |
| `PLACEHOLDER_CIDR_K8S_APISERVER` | `networkpolicies.yaml` (arm egress) | Control-plane endpoint for the Secret write |

> **Egress allowlist is one-way-door flag #11.** Widening any `PLACEHOLDER_CIDR_*`
> weakens the C4 read-only backstop (the only bound on raw `curl`/`gh api`/
> git-over-HTTPS the host can make). Each addition is a security-review event.

---

## In-cluster Postgres → managed PG

Phase-1 runs `postgres:16-alpine` in-cluster. Swap to managed PG (RDS/CloudSQL)
when point-in-time recovery is required: point `database_uri` (and the arm Job's
`OMNIGENT_DATABASE_URI`) at the managed DSN via a Secret, drop the
`postgres-statefulset.yaml` / `postgres-service.yaml` from the base, and remove
the `postgres-ingress` NetworkPolicy. No server code change.

---

## Cross-review outcome + remaining pre-apply gates

**Resolved during cross-review (implemented in these manifests):**
- **Arm script — IMPLEMENTED.** `host-arm-job.yaml` / `host-rearm-cronjob.yaml`
  now run a real fail-closed `python -c` reusing `omnigent.stores.host_store.HostStore(dsn)`
  + `register_managed_host(...)` (keyword signature + import verified vs omnigent
  0.1.0; `HostStore.__init__(storage_location)` per `cli.py:3003`). The DB DSN is
  sourced via `secretKeyRef` (no inline password). **Still a pre-apply gate:** the
  synthetic-arming call must be re-confirmed accepted against the pinned **0.1.1**
  (PLT-675), and the image must carry the `kubernetes` python client (or `kubectl`).
- **oauth2-proxy header mapping — RESOLVED.** `--set-xauthrequest` removed (it fed
  the rejected nginx auth-url pattern); `--pass-user-headers=true` is the load-bearing
  control (sets + overwrites `X-Forwarded-Email` from the validated session, read
  verbatim at `auth.py:353`). An ingress-level `more_clear_input_headers` inbound
  strip was added as defense-in-depth.
- **`commonLabels` → `labels` migration — DONE** (details below).
- **securityContext floor** applied across Deployments + the arm/re-arm workloads
  (PSS-restricted baseline; postgres keeps a documented root carve-out).

**Remaining pre-apply gates (MUST pass before `kubectl apply`):**
1. **0.1.0→0.1.1 auth-path re-diff** (PLT-675) — re-verify the 4 auth-path files
   and the `register_managed_host`/`HostStore` shape against the pinned 0.1.1.
2. **Forged-`X-Forwarded-Email` negative test** — a request carrying a client
   `X-Forwarded-Email` through the full Ingress→oauth2-proxy chain MUST NOT
   authenticate as that user.
3. **PLT-674 live-rotation** — does token rotation re-auth the live tunnel or
   only the next dial? Determines whether a post-re-arm host restart is needed.
4. **CNI enforcement** — confirm the target cluster's CNI enforces NetworkPolicy
   (Cilium/Calico); the header-mode trust model collapses without it.
5. **Ingress controller** — annotations are nginx-specific (buffering/timeout +
   the `configuration-snippet` strip); translate for the actual
   `PLACEHOLDER_INGRESSCLASS`, or confirm snippet annotations are enabled.

### `commonLabels` → `labels` migration (DONE)
Both kustomizations now use
   `labels:` with `includeSelectors: false`. This is not cosmetic: under the old
   `commonLabels`, the `app.kubernetes.io/*` labels were injected into EVERY
   podSelector — including the cross-namespace **kube-dns peer** in the
   `allow-dns-egress` NetworkPolicy. That made the kube-dns selector
   `{k8s-app: kube-dns, app.kubernetes.io/instance: …, …}`, which matches NO real
   CoreDNS pod, so DNS egress would break at apply under an enforcing CNI. With
   `includeSelectors: false` the labels are descriptive only; selectors and
   policy peers stay on the bare `app:` key. The deprecation warning is also gone.
